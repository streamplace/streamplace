package s3

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

// fakeCopyClient is an in-memory stand-in for the subset of *s3.Client that
// Copy uses. HeadObject reports a configurable size/content-type (so the
// >5 GiB multipart path can be exercised without allocating gigabytes), and
// the multipart ops record the ranges they were asked to copy.
type fakeCopyClient struct {
	headSize        int64
	headContentType string
	headErr         error

	copyDelay time.Duration
	failPart  int32 // if >0, UploadPartCopy for this part number returns an error

	srcBytes []byte            // for the small-object CopyObject path
	dst      map[string][]byte // CopyObject writes here when non-nil

	mu              sync.Mutex
	copyObjectCalls int
	createCalls     int
	createdType     string
	partRanges      map[int32][2]int64
	completedParts  []types.CompletedPart
	aborted         bool
	inFlight        int
	maxInFlight     int
}

func (f *fakeCopyClient) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if f.headErr != nil {
		return nil, f.headErr
	}
	out := &s3.HeadObjectOutput{ContentLength: aws.Int64(f.headSize)}
	if f.headContentType != "" {
		out.ContentType = aws.String(f.headContentType)
	}
	return out, nil
}

func (f *fakeCopyClient) CopyObject(_ context.Context, in *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.copyObjectCalls++
	if f.dst != nil {
		f.dst[aws.ToString(in.Key)] = f.srcBytes
	}
	return &s3.CopyObjectOutput{}, nil
}

func (f *fakeCopyClient) CreateMultipartUpload(_ context.Context, in *s3.CreateMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	f.createdType = aws.ToString(in.ContentType)
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String("test-upload-id")}, nil
}

func (f *fakeCopyClient) UploadPartCopy(_ context.Context, in *s3.UploadPartCopyInput, _ ...func(*s3.Options)) (*s3.UploadPartCopyOutput, error) {
	f.mu.Lock()
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	f.mu.Unlock()

	if f.copyDelay > 0 {
		time.Sleep(f.copyDelay)
	}

	num := aws.ToInt32(in.PartNumber)
	var start, end int64
	if _, err := fmt.Sscanf(aws.ToString(in.CopySourceRange), "bytes=%d-%d", &start, &end); err != nil {
		return nil, fmt.Errorf("bad CopySourceRange %q: %w", aws.ToString(in.CopySourceRange), err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.inFlight--
	if f.failPart > 0 && num == f.failPart {
		return nil, fmt.Errorf("simulated failure on part %d", num)
	}
	f.partRanges[num] = [2]int64{start, end}
	return &s3.UploadPartCopyOutput{
		CopyPartResult: &types.CopyPartResult{ETag: aws.String(fmt.Sprintf("etag-%d", num))},
	}, nil
}

func (f *fakeCopyClient) CompleteMultipartUpload(_ context.Context, in *s3.CompleteMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completedParts = in.MultipartUpload.Parts
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeCopyClient) AbortMultipartUpload(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.aborted = true
	return &s3.AbortMultipartUploadOutput{}, nil
}

// TestCopySmallObjectUsesCopyObject verifies sub-5-GiB objects take the
// single-call CopyObject path and land byte-for-byte at the destination.
func TestCopySmallObjectUsesCopyObject(t *testing.T) {
	src := []byte("hello world, a small object")
	fake := &fakeCopyClient{
		headSize:        int64(len(src)),
		headContentType: "video/mp4",
		srcBytes:        src,
		dst:             map[string][]byte{},
		partRanges:      map[int32][2]int64{},
	}
	require.NoError(t, copyObject(context.Background(), fake, "bucket", "src", "dst"))
	require.Equal(t, 1, fake.copyObjectCalls)
	require.Equal(t, 0, fake.createCalls, "small object must not open a multipart upload")
	require.Equal(t, src, fake.dst["dst"])
}

// TestCopyAtThresholdUsesCopyObject pins the boundary: an object exactly at
// maxCopyObjectSize still copies in one shot.
func TestCopyAtThresholdUsesCopyObject(t *testing.T) {
	fake := &fakeCopyClient{headSize: maxCopyObjectSize, partRanges: map[int32][2]int64{}}
	require.NoError(t, copyObject(context.Background(), fake, "bucket", "src", "dst"))
	require.Equal(t, 1, fake.copyObjectCalls)
	require.Equal(t, 0, fake.createCalls)
}

// TestCopyLargeObjectUsesMultipart verifies an over-5-GiB object is copied
// with a multipart UploadPartCopy whose parts tile the source exactly,
// preserve content type, finish in ascending order, and run concurrently.
func TestCopyLargeObjectUsesMultipart(t *testing.T) {
	const size = int64(12) * 1024 * 1024 * 1024 // 12 GiB -> spans many parts
	fake := &fakeCopyClient{
		headSize:        size,
		headContentType: "video/mp4",
		copyDelay:       5 * time.Millisecond, // force overlap so concurrency is observable
		partRanges:      map[int32][2]int64{},
	}
	require.NoError(t, copyObject(context.Background(), fake, "bucket", "src", "dst"))

	require.Equal(t, 0, fake.copyObjectCalls, "large object must not use single CopyObject")
	require.Equal(t, 1, fake.createCalls)
	require.Equal(t, "video/mp4", fake.createdType, "content type must be preserved")

	wantParts := int((size + copyPartSize - 1) / copyPartSize)
	require.Len(t, fake.partRanges, wantParts)
	require.Len(t, fake.completedParts, wantParts)

	// Parts must tile [0,size) contiguously with no gaps or overlaps, and
	// every part except the last is exactly copyPartSize.
	var covered int64
	for n := int32(1); n <= int32(wantParts); n++ {
		r, ok := fake.partRanges[n]
		require.Truef(t, ok, "missing part %d", n)
		require.Equalf(t, covered, r[0], "part %d start should continue from previous end", n)
		require.GreaterOrEqual(t, r[1], r[0])
		if int(n) < wantParts {
			require.Equalf(t, int64(copyPartSize), r[1]-r[0]+1, "non-final part %d should be a full part", n)
		}
		covered = r[1] + 1
	}
	require.Equal(t, size, covered, "parts must cover the whole object")

	// CompleteMultipartUpload requires ascending part numbers.
	for i := 1; i < len(fake.completedParts); i++ {
		require.Less(t,
			aws.ToInt32(fake.completedParts[i-1].PartNumber),
			aws.ToInt32(fake.completedParts[i].PartNumber))
	}
	require.Greater(t, fake.maxInFlight, 1, "expected part copies to run concurrently")
	require.False(t, fake.aborted)
}

// TestCopyLargeObjectAbortsOnPartError verifies a failed part copy aborts
// the multipart upload (so S3 doesn't retain orphaned parts) and never
// completes it.
func TestCopyLargeObjectAbortsOnPartError(t *testing.T) {
	const size = int64(12) * 1024 * 1024 * 1024
	fake := &fakeCopyClient{
		headSize:   size,
		partRanges: map[int32][2]int64{},
		failPart:   3,
	}
	err := copyObject(context.Background(), fake, "bucket", "src", "dst")
	require.Error(t, err)
	require.Contains(t, err.Error(), "upload part copy 3")
	require.True(t, fake.aborted, "a failed part must abort the upload")
	require.Empty(t, fake.completedParts, "must not complete after a part failure")
}

// TestCopyHeadErrorPropagates verifies a missing source surfaces the
// HeadObject error (which blob.Move sniffs as NotFound for idempotency)
// before any copy is attempted.
func TestCopyHeadErrorPropagates(t *testing.T) {
	fake := &fakeCopyClient{headErr: fmt.Errorf("api error NotFound: object missing")}
	err := copyObject(context.Background(), fake, "bucket", "missing", "dst")
	require.Error(t, err)
	require.Contains(t, err.Error(), "NotFound")
	require.Equal(t, 0, fake.copyObjectCalls)
	require.Equal(t, 0, fake.createCalls)
}
