package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeConcatS3 is an in-memory stand-in for the subset of S3 ConcatWithHeader
// uses. CompleteMultipartUpload reconstructs the destination object from the
// recorded parts in part-number order and enforces S3's rule that every part
// except the last is ≥5MB, so a ConcatWithHeader bug that emits a small middle
// part fails loudly here rather than only against real S3.
type fakeConcatS3 struct {
	objects   map[string][]byte
	parts     map[int32][]byte // partNumber -> bytes, for the single in-flight upload
	assembled map[string][]byte
	aborted   bool
}

func newFakeConcatS3(objects map[string][]byte) *fakeConcatS3 {
	return &fakeConcatS3{
		objects:   objects,
		parts:     map[int32][]byte{},
		assembled: map[string][]byte{},
	}
}

func (f *fakeConcatS3) HeadObject(_ context.Context, in *awss3.HeadObjectInput, _ ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	b, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, fmt.Errorf("no such object %s", aws.ToString(in.Key))
	}
	return &awss3.HeadObjectOutput{ContentLength: aws.Int64(int64(len(b)))}, nil
}

func (f *fakeConcatS3) GetObject(_ context.Context, in *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	b, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, fmt.Errorf("no such object %s", aws.ToString(in.Key))
	}
	start, end := parseRange(aws.ToString(in.Range), len(b))
	chunk := b[start : end+1]
	return &awss3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(chunk))}, nil
}

func (f *fakeConcatS3) CreateMultipartUpload(_ context.Context, _ *awss3.CreateMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CreateMultipartUploadOutput, error) {
	return &awss3.CreateMultipartUploadOutput{UploadId: aws.String("upload-1")}, nil
}

func (f *fakeConcatS3) UploadPart(_ context.Context, in *awss3.UploadPartInput, _ ...func(*awss3.Options)) (*awss3.UploadPartOutput, error) {
	body, _ := io.ReadAll(in.Body)
	f.parts[aws.ToInt32(in.PartNumber)] = body
	return &awss3.UploadPartOutput{ETag: aws.String(fmt.Sprintf("etag-%d", aws.ToInt32(in.PartNumber)))}, nil
}

func (f *fakeConcatS3) UploadPartCopy(_ context.Context, in *awss3.UploadPartCopyInput, _ ...func(*awss3.Options)) (*awss3.UploadPartCopyOutput, error) {
	// CopySource is "bucket/key"; strip the bucket prefix.
	src := aws.ToString(in.CopySource)
	key := src[strings.Index(src, "/")+1:]
	b, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("copy from missing object %s", key)
	}
	start, end := parseRange(aws.ToString(in.CopySourceRange), len(b))
	f.parts[aws.ToInt32(in.PartNumber)] = append([]byte(nil), b[start:end+1]...)
	return &awss3.UploadPartCopyOutput{
		CopyPartResult: &types.CopyPartResult{ETag: aws.String(fmt.Sprintf("etag-%d", aws.ToInt32(in.PartNumber)))},
	}, nil
}

func (f *fakeConcatS3) CompleteMultipartUpload(_ context.Context, in *awss3.CompleteMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CompleteMultipartUploadOutput, error) {
	nums := make([]int32, 0, len(in.MultipartUpload.Parts))
	for _, p := range in.MultipartUpload.Parts {
		nums = append(nums, aws.ToInt32(p.PartNumber))
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })
	var out []byte
	for i, n := range nums {
		body := f.parts[n]
		isLast := i == len(nums)-1
		if !isLast && len(body) < minPartSize {
			return nil, fmt.Errorf("EntityTooSmall: part %d is %d bytes (<5MB) and not last", n, len(body))
		}
		out = append(out, body...)
	}
	f.assembled[aws.ToString(in.Key)] = out
	return &awss3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeConcatS3) AbortMultipartUpload(_ context.Context, _ *awss3.AbortMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.AbortMultipartUploadOutput, error) {
	f.aborted = true
	return &awss3.AbortMultipartUploadOutput{}, nil
}

// CopyObject is required to satisfy copyAPI (embedded in concatAPI) but unused.
func (f *fakeConcatS3) CopyObject(_ context.Context, _ *awss3.CopyObjectInput, _ ...func(*awss3.Options)) (*awss3.CopyObjectOutput, error) {
	return nil, errors.New("unexpected CopyObject")
}

func parseRange(r string, total int) (int64, int64) {
	// "bytes=start-end"
	spec := strings.TrimPrefix(r, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	start, _ := strconv.ParseInt(parts[0], 10, 64)
	end, _ := strconv.ParseInt(parts[1], 10, 64)
	if end > int64(total)-1 {
		end = int64(total) - 1
	}
	return start, end
}

func filled(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

const mb = 1024 * 1024

func TestConcatWithHeader(t *testing.T) {
	header := filled('H', 1024)

	cases := []struct {
		name    string
		objects map[string][]byte
		order   []string
		wantErr error
	}{
		{
			name: "large objects partial first part",
			objects: map[string][]byte{
				"a": filled('a', 10*mb),
				"b": filled('b', 8*mb),
				"c": filled('c', 7*mb),
			},
			order: []string{"a", "b", "c"},
		},
		{
			name: "medium objects absorb whole first object",
			objects: map[string][]byte{
				"a": filled('a', 6*mb),
				"b": filled('b', 6*mb),
				"c": filled('c', 6*mb),
			},
			order: []string{"a", "b", "c"},
		},
		{
			name: "single small object is the only part",
			objects: map[string][]byte{
				"a": filled('a', 2*mb),
			},
			order: []string{"a"},
		},
		{
			name: "small last object is allowed",
			objects: map[string][]byte{
				"a": filled('a', 8*mb),
				"b": filled('b', 1*mb),
			},
			order: []string{"a", "b"},
		},
		{
			name: "small middle object falls back",
			objects: map[string][]byte{
				"a": filled('a', 6*mb),
				"b": filled('b', 2*mb),
				"c": filled('c', 6*mb),
			},
			order:   []string{"a", "b", "c"},
			wantErr: ErrConcatPartTooSmall,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeConcatS3(tc.objects)
			err := concatWithHeader(context.Background(), fake, "bucket", header, tc.order, "dst", "video/mp4")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want error %v, got %v", tc.wantErr, err)
				}
				if !fake.aborted {
					t.Fatalf("expected multipart upload to be aborted on too-small part")
				}
				return
			}
			if err != nil {
				t.Fatalf("concatWithHeader: %v", err)
			}
			// Expected = header ++ objects in order.
			want := append([]byte(nil), header...)
			for _, k := range tc.order {
				want = append(want, tc.objects[k]...)
			}
			got := fake.assembled["dst"]
			if !bytes.Equal(got, want) {
				t.Fatalf("assembled mismatch: got %d bytes, want %d bytes", len(got), len(want))
			}
		})
	}
}
