package s3

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

// fakeUploadAPI is an in-memory stand-in for the multipart subset of *s3.Client
// the upload loop drives. It hands back dummy upload IDs / etags; set
// failCompletes to make the first N CompleteMultipartUpload calls fail.
type fakeUploadAPI struct {
	mu            sync.Mutex
	creates       int
	partSizes     []int
	failCompletes int
	completes     int
	aborts        int
}

func (f *fakeUploadAPI) CreateMultipartUpload(_ context.Context, _ *awss3.CreateMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CreateMultipartUploadOutput, error) {
	f.mu.Lock()
	f.creates++
	n := f.creates
	f.mu.Unlock()
	return &awss3.CreateMultipartUploadOutput{UploadId: aws.String(fmt.Sprintf("up-%d", n))}, nil
}

func (f *fakeUploadAPI) UploadPart(_ context.Context, in *awss3.UploadPartInput, _ ...func(*awss3.Options)) (*awss3.UploadPartOutput, error) {
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.partSizes = append(f.partSizes, len(body))
	f.mu.Unlock()
	return &awss3.UploadPartOutput{ETag: aws.String("etag")}, nil
}

func (f *fakeUploadAPI) CompleteMultipartUpload(_ context.Context, _ *awss3.CompleteMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CompleteMultipartUploadOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCompletes > 0 {
		f.failCompletes--
		return nil, fmt.Errorf("InvalidPart: all non-trailing parts must have the same length")
	}
	f.completes++
	return &awss3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeUploadAPI) AbortMultipartUpload(_ context.Context, _ *awss3.AbortMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.AbortMultipartUploadOutput, error) {
	f.mu.Lock()
	f.aborts++
	f.mu.Unlock()
	return &awss3.AbortMultipartUploadOutput{}, nil
}

// fakeRecorder captures the (key, livestreamURI) of every started object.
type fakeRecorder struct {
	mu     sync.Mutex
	keys   []string
	uris   []string
	starts int
}

func (r *fakeRecorder) RecordStart(_ context.Context, _, _, key, livestreamURI string, _ time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, key)
	r.uris = append(r.uris, livestreamURI)
	r.starts++
	return fmt.Sprintf("rec-%d", r.starts), nil
}

func (r *fakeRecorder) RecordComplete(_ context.Context, _ string, _ int32, _ int64) error {
	return nil
}

func (r *fakeRecorder) count() int { r.mu.Lock(); defer r.mu.Unlock(); return r.starts }

func (r *fakeRecorder) startURIs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.uris...)
}

func (r *fakeRecorder) startKeys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.keys...)
}

func waitForStarts(t *testing.T, rec *fakeRecorder, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rec.count() >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d RecordStart calls (got %d)", n, rec.count())
}

// TestS3UploaderCutoverOnLivestreamChange proves the uploader rolls over to a
// fresh object the moment the livestream URI changes (a new "chapter" record),
// so each object belongs to exactly one livestream — which is what lets finalize
// select one livestream's objects. cutoverEvery is set huge so ONLY the
// livestream change can trigger the rollover.
func TestS3UploaderCutoverOnLivestreamChange(t *testing.T) {
	fc := &fakeUploadAPI{}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec)

	ctx := context.Background()
	seg := make([]byte, 1024) // well under minPartSize: buffered until the object completes

	u.SetLivestreamURI("at://A")
	require.NoError(t, u.AddSegment(ctx, seg))
	waitForStarts(t, rec, 1) // object 1, livestream A

	u.SetLivestreamURI("at://B")
	require.NoError(t, u.AddSegment(ctx, seg))
	waitForStarts(t, rec, 2) // livestream changed -> object 2, livestream B

	require.NoError(t, u.Close(ctx))

	require.Equal(t, []string{"at://A", "at://B"}, rec.startURIs(),
		"each object must be tagged with the livestream active when it started")
	keys := rec.startKeys()
	require.Len(t, keys, 2)
	require.NotEqual(t, keys[0], keys[1], "rolled-over objects must have distinct keys")
}

// TestS3UploaderCutoverCompletesObject proves Cutover closes out the current
// object so the next segment starts a fresh one. This is what makes a recording
// finalize-able the moment a livestream ends, rather than lingering until the
// cutoverEvery timer or stream teardown. cutoverEvery is huge so only the
// explicit Cutover can trigger the rollover.
func TestS3UploaderCutoverCompletesObject(t *testing.T) {
	fc := &fakeUploadAPI{}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec)

	ctx := context.Background()
	seg := make([]byte, 1024) // under minPartSize: buffered until the object completes

	u.SetLivestreamURI("at://A")
	require.NoError(t, u.AddSegment(ctx, seg))
	waitForStarts(t, rec, 1) // object 1

	require.NoError(t, u.Cutover(ctx)) // completes object 1
	require.NoError(t, u.AddSegment(ctx, seg))
	waitForStarts(t, rec, 2) // object 2

	require.NoError(t, u.Close(ctx))

	require.Equal(t, 2, rec.count(), "Cutover must close the object so the next segment starts a new one")
	keys := rec.startKeys()
	require.Len(t, keys, 2)
	require.NotEqual(t, keys[0], keys[1], "post-cutover object must have a distinct key")
}

// TestS3UploaderUniformParts proves the uploader slices its buffer into
// exactly liveUploadPartSize parts regardless of segment sizes, with only the
// object's final part smaller. R2 rejects multipart completes whose
// non-trailing parts differ in length, so "flush whatever accumulated past
// 5MB" (the old behavior) breaks against R2 with real, variable-size
// segments.
func TestS3UploaderUniformParts(t *testing.T) {
	fc := &fakeUploadAPI{}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec)

	ctx := context.Background()
	total := 0
	for _, mb := range []int{2, 3, 4, 3} { // 12MB in irregular chunks
		require.NoError(t, u.AddSegment(ctx, make([]byte, mb*1024*1024)))
		total += mb * 1024 * 1024
	}
	require.NoError(t, u.Close(ctx))

	fc.mu.Lock()
	sizes := append([]int(nil), fc.partSizes...)
	fc.mu.Unlock()
	require.NotEmpty(t, sizes)
	got := 0
	for i, s := range sizes {
		got += s
		if i < len(sizes)-1 {
			require.Equal(t, liveUploadPartSize, s, "non-trailing part %d must be exactly liveUploadPartSize", i+1)
		} else {
			require.LessOrEqual(t, s, liveUploadPartSize)
		}
	}
	require.Equal(t, total, got, "flushed parts must cover every byte exactly once")
}

// TestS3UploaderRecoversFromCompleteFailure proves a failed
// CompleteMultipartUpload doesn't wedge the uploader: the broken object is
// aborted and abandoned, and the next segment starts a fresh object that
// uploads normally. Before this behavior existed, the first error killed the
// upload loop and the rest of the stream was silently never recorded — which
// is exactly how R2's InvalidPart rejection presented in production.
func TestS3UploaderRecoversFromCompleteFailure(t *testing.T) {
	fc := &fakeUploadAPI{failCompletes: 1}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec)

	ctx := context.Background()
	seg := make([]byte, 1024)

	require.NoError(t, u.AddSegment(ctx, seg))
	waitForStarts(t, rec, 1)           // object 1
	require.NoError(t, u.Cutover(ctx)) // complete fails -> object 1 abandoned+aborted

	require.NoError(t, u.AddSegment(ctx, seg))
	waitForStarts(t, rec, 2) // loop survived: object 2 started

	require.NoError(t, u.Close(ctx), "mid-stream failure must not surface at Close; the final object completed fine")

	fc.mu.Lock()
	defer fc.mu.Unlock()
	require.Equal(t, 2, fc.creates, "a fresh object must start after the failure")
	require.Equal(t, 1, fc.aborts, "the broken object must be aborted, not leaked")
	require.Equal(t, 1, fc.completes, "the post-failure object must complete")
}

// TestS3UploaderCloseIdempotent exercises the lifecycle fix that re-enabled
// live S3 upload: Close must be safe to call repeatedly and concurrently (it
// was a plain close(segCh) before, which panicked on the second call), and
// AddSegment after Close must return an error rather than panic with
// "send on closed channel". No segments are added, so the upload loop completes
// without making any S3 calls — this stays a pure unit test.
func TestS3UploaderCloseIdempotent(t *testing.T) {
	u := NewS3Uploader(Config{
		Region:          "us-east-1",
		Endpoint:        "http://127.0.0.1:0",
		Bucket:          "test",
		AccessKeyID:     "k",
		SecretAccessKey: "s",
	}, "did:plc:test", "did:plc:test/", time.Minute, nil)

	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = u.Close(context.Background())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Close call %d returned error: %v", i, err)
		}
	}

	// A late AddSegment must be rejected, not panic on a closed channel.
	if err := u.AddSegment(context.Background(), []byte("late")); err == nil {
		t.Fatalf("AddSegment after Close should return an error")
	}
}
