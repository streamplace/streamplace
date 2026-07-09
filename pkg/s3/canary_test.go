package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

// fakeCanaryS3 is an in-memory S3 for the canary: it stores objects, tracks a
// single in-flight multipart upload, and — like fakeConcatS3 — enforces R2's
// uniform-length rule at CompleteMultipartUpload, since catching that class of
// provider strictness is the canary's whole reason to exist.
type fakeCanaryS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	parts   map[int32][]byte
	aborted bool
	deleted []string
}

func newFakeCanaryS3() *fakeCanaryS3 {
	return &fakeCanaryS3{objects: map[string][]byte{}, parts: map[int32][]byte{}}
}

func (f *fakeCanaryS3) PutObject(_ context.Context, in *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.objects[aws.ToString(in.Key)] = body
	f.mu.Unlock()
	return &awss3.PutObjectOutput{}, nil
}

func (f *fakeCanaryS3) GetObject(_ context.Context, in *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	f.mu.Lock()
	b, ok := f.objects[aws.ToString(in.Key)]
	f.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no such object %s", aws.ToString(in.Key))
	}
	return &awss3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func (f *fakeCanaryS3) DeleteObject(_ context.Context, in *awss3.DeleteObjectInput, _ ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
	f.mu.Lock()
	key := aws.ToString(in.Key)
	delete(f.objects, key)
	f.deleted = append(f.deleted, key)
	f.mu.Unlock()
	return &awss3.DeleteObjectOutput{}, nil
}

func (f *fakeCanaryS3) CreateMultipartUpload(_ context.Context, _ *awss3.CreateMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CreateMultipartUploadOutput, error) {
	return &awss3.CreateMultipartUploadOutput{UploadId: aws.String("canary-upload")}, nil
}

func (f *fakeCanaryS3) UploadPart(_ context.Context, in *awss3.UploadPartInput, _ ...func(*awss3.Options)) (*awss3.UploadPartOutput, error) {
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.parts[aws.ToInt32(in.PartNumber)] = body
	f.mu.Unlock()
	return &awss3.UploadPartOutput{ETag: aws.String(fmt.Sprintf("etag-%d", aws.ToInt32(in.PartNumber)))}, nil
}

func (f *fakeCanaryS3) UploadPartCopy(_ context.Context, in *awss3.UploadPartCopyInput, _ ...func(*awss3.Options)) (*awss3.UploadPartCopyOutput, error) {
	src := aws.ToString(in.CopySource)
	key := src[len("bucket/"):]
	f.mu.Lock()
	defer f.mu.Unlock()
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

func (f *fakeCanaryS3) CompleteMultipartUpload(_ context.Context, in *awss3.CompleteMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CompleteMultipartUploadOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	nums := make([]int32, 0, len(in.MultipartUpload.Parts))
	for _, p := range in.MultipartUpload.Parts {
		nums = append(nums, aws.ToInt32(p.PartNumber))
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })
	var out []byte
	for i, n := range nums {
		body := f.parts[n]
		if i < len(nums)-1 && len(body) != len(f.parts[nums[0]]) {
			return nil, fmt.Errorf("InvalidPart: all non-trailing parts must have the same length")
		}
		out = append(out, body...)
	}
	f.objects[aws.ToString(in.Key)] = out
	return &awss3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeCanaryS3) AbortMultipartUpload(_ context.Context, _ *awss3.AbortMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.AbortMultipartUploadOutput, error) {
	f.mu.Lock()
	f.aborted = true
	f.mu.Unlock()
	return &awss3.AbortMultipartUploadOutput{}, nil
}

func TestCanaryPassesAgainstCompliantBackend(t *testing.T) {
	fake := newFakeCanaryS3()
	require.NoError(t, runCanary(context.Background(), fake, "bucket"))
	require.Empty(t, fake.objects, "canary must clean up its probe objects")
	require.Len(t, fake.deleted, 2, "both source and destination must be deleted")
	require.False(t, fake.aborted, "successful probe should not abort")
}

// failingCompleteS3 wraps the fake to reject completes, standing in for a
// provider that can't support the live-rec call pattern.
type failingCompleteS3 struct{ *fakeCanaryS3 }

func (f *failingCompleteS3) CompleteMultipartUpload(_ context.Context, _ *awss3.CompleteMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CompleteMultipartUploadOutput, error) {
	return nil, fmt.Errorf("InvalidPart: nope")
}

func TestCanaryFailsAndCleansUpAgainstBrokenBackend(t *testing.T) {
	fake := &failingCompleteS3{newFakeCanaryS3()}
	err := runCanary(context.Background(), fake, "bucket")
	require.Error(t, err)
	require.Contains(t, err.Error(), "complete multipart upload")
	require.True(t, fake.aborted, "failed probe must abort the multipart upload")
	require.Empty(t, fake.objects, "failed probe must still delete the source object")
}
