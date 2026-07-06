package s3

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

// fakeMultipartClient is a minimal in-memory stand-in for *s3.Client that
// records uploaded parts and tracks how many uploads run concurrently.
type fakeMultipartClient struct {
	uploadDelay time.Duration

	mu          sync.Mutex
	parts       map[int32][]byte
	completed   []types.CompletedPart
	inFlight    int
	maxInFlight int
}

func newFakeMultipartClient(delay time.Duration) *fakeMultipartClient {
	return &fakeMultipartClient{uploadDelay: delay, parts: map[int32][]byte{}}
}

func (f *fakeMultipartClient) CreateMultipartUpload(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String("test-upload-id")}, nil
}

func (f *fakeMultipartClient) UploadPart(_ context.Context, in *s3.UploadPartInput, _ ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	f.mu.Lock()
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	f.mu.Unlock()

	if f.uploadDelay > 0 {
		time.Sleep(f.uploadDelay)
	}
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.parts[*in.PartNumber] = body
	f.inFlight--
	f.mu.Unlock()
	return &s3.UploadPartOutput{ETag: aws.String(fmt.Sprintf("etag-%d", *in.PartNumber))}, nil
}

func (f *fakeMultipartClient) CompleteMultipartUpload(_ context.Context, in *s3.CompleteMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	f.completed = in.MultipartUpload.Parts
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeMultipartClient) AbortMultipartUpload(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	return &s3.AbortMultipartUploadOutput{}, nil
}

func (f *fakeMultipartClient) PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return &s3.PutObjectOutput{}, nil
}

// TestMultipartWriterConcurrent verifies that parts upload concurrently,
// are finalized in ascending part-number order despite finishing out of
// order, and reassemble byte-for-byte into what was written.
func TestMultipartWriterConcurrent(t *testing.T) {
	// A per-part delay forces overlap so we can observe concurrency.
	fake := newFakeMultipartClient(20 * time.Millisecond)
	w, err := newMultipartWriter(context.Background(), fake, "bucket", "key", "video/mp4")
	require.NoError(t, err)

	// Three full parts plus a sub-part remainder.
	total := MultipartPartSize*3 + 1234
	want := make([]byte, total)
	for i := range want {
		want[i] = byte(i * 7 % 251)
	}

	// Write in 1 MB chunks to exercise buffering across part boundaries.
	const chunk = 1 << 20
	for off := 0; off < total; off += chunk {
		end := off + chunk
		if end > total {
			end = total
		}
		n, werr := w.Write(want[off:end])
		require.NoError(t, werr)
		require.Equal(t, end-off, n)
	}
	require.NoError(t, w.Complete())

	// 3 full parts + 1 remainder = 4 parts.
	require.Len(t, fake.parts, 4)
	require.Greater(t, fake.maxInFlight, 1, "expected parts to upload concurrently")

	// Parts handed to CompleteMultipartUpload must be strictly ascending.
	require.Len(t, fake.completed, 4)
	for i := 1; i < len(fake.completed); i++ {
		require.Less(t,
			aws.ToInt32(fake.completed[i-1].PartNumber),
			aws.ToInt32(fake.completed[i].PartNumber),
			"parts must be in ascending order for CompleteMultipartUpload")
	}

	// Full parts are exactly MultipartPartSize; the last is the remainder.
	require.Equal(t, MultipartPartSize, len(fake.parts[1]))
	require.Equal(t, 1234, len(fake.parts[4]))

	// Reassemble in part order and compare to the original bytes.
	var got []byte
	for i := int32(1); i <= int32(len(fake.parts)); i++ {
		got = append(got, fake.parts[i]...)
	}
	require.Equal(t, want, got)
}

// TestUploadWriterCommitsOnClose verifies UploadWriter.Close finalizes the
// object (CompleteMultipartUpload) rather than aborting it, and that the
// written bytes reassemble into a single object. This is the property that
// makes UploadWriter safe as a fire-and-forget io.WriteCloser for debug
// recordings, where the raw MultipartWriter.Close() would silently abort.
func TestUploadWriterCommitsOnClose(t *testing.T) {
	fake := newFakeMultipartClient(0)
	w, err := newUploadWriter(context.Background(), fake, "bucket", "debug-recordings/did/x.rtcrec.cbor", "application/cbor")
	require.NoError(t, err)
	require.Equal(t, "debug-recordings/did/x.rtcrec.cbor", w.Name())

	want := make([]byte, MultipartPartSize+4096)
	for i := range want {
		want[i] = byte(i * 3 % 251)
	}
	n, err := w.Write(want)
	require.NoError(t, err)
	require.Equal(t, len(want), n)

	require.NoError(t, w.Close())

	// Close must have completed (not aborted) the upload.
	require.NotEmpty(t, fake.completed, "Close should CompleteMultipartUpload")

	// Reassemble parts in order and compare to what we wrote.
	got := make([]byte, 0, len(want))
	for i := int32(1); i <= int32(len(fake.parts)); i++ {
		got = append(got, fake.parts[i]...)
	}
	require.Equal(t, want, got)
}
