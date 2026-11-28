package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
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
	conditions    []string
}

type abortProbe struct {
	fakeUploadAPI
	liveBoundedAbort bool
}

func (p *abortProbe) AbortMultipartUpload(ctx context.Context, in *awss3.AbortMultipartUploadInput, opts ...func(*awss3.Options)) (*awss3.AbortMultipartUploadOutput, error) {
	deadline, bounded := ctx.Deadline()
	p.liveBoundedAbort = ctx.Err() == nil && bounded && time.Until(deadline) <= 2*time.Second
	return p.fakeUploadAPI.AbortMultipartUpload(ctx, in, opts...)
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

func (f *fakeUploadAPI) CompleteMultipartUpload(_ context.Context, in *awss3.CompleteMultipartUploadInput, _ ...func(*awss3.Options)) (*awss3.CompleteMultipartUploadOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conditions = append(f.conditions, aws.ToString(in.IfNoneMatch))
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

// fakeRecorder captures the (key, livestreamURI, start) of every started
// object, the segment index of every completed one, and the aborted ids.
type fakeRecorder struct {
	mu          sync.Mutex
	keys        []string
	uris        []string
	started     []time.Time
	indexes     [][]RecordedSegment
	aborted     []string
	starts      int
	startErr    error
	completeErr error
	abortErr    error
}

func (r *fakeRecorder) RecordStart(_ context.Context, _, _, key, livestreamURI string, started time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, key)
	r.uris = append(r.uris, livestreamURI)
	r.started = append(r.started, started)
	r.starts++
	return fmt.Sprintf("rec-%d", r.starts), r.startErr
}

func (r *fakeRecorder) RecordComplete(_ context.Context, _ string, _ int32, _ int64, segments []RecordedSegment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.indexes = append(r.indexes, segments)
	return r.completeErr
}

func (r *fakeRecorder) RecordAbort(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aborted = append(r.aborted, id)
	return r.abortErr
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

	// Hold creation until both segments are queued under their own URI.
	fc.mu.Lock()
	u.SetLivestreamURI("at://A")
	firstErr := u.AddSegment(ctx, seg, time.Now())
	u.SetLivestreamURI("at://B")
	secondErr := u.AddSegment(ctx, seg, time.Now())
	fc.mu.Unlock()
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)

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
	require.NoError(t, u.AddSegment(ctx, seg, time.Now()))
	waitForStarts(t, rec, 1) // object 1

	require.NoError(t, u.Cutover(ctx)) // completes object 1
	require.NoError(t, u.AddSegment(ctx, seg, time.Now()))
	waitForStarts(t, rec, 2) // object 2

	require.NoError(t, u.Close(ctx))

	require.Equal(t, 2, rec.count(), "Cutover must close the object so the next segment starts a new one")
	keys := rec.startKeys()
	require.Len(t, keys, 2)
	require.NotEqual(t, keys[0], keys[1], "post-cutover object must have a distinct key")
}

// TestS3UploaderRecordsSegmentIndex proves each completed object is recorded
// with exactly its own segments' start times and sizes, in order, and starts
// at its first segment's start: clips locate segments inside recorded objects
// by that index, so a segment attributed to the wrong object (or a stale
// index carried across a cutover) would cut the wrong bytes.
func TestS3UploaderRecordsSegmentIndex(t *testing.T) {
	fc := &fakeUploadAPI{}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec)

	ctx := context.Background()
	t0 := time.UnixMilli(1_760_000_000_000)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	require.NoError(t, u.AddSegment(ctx, make([]byte, 100), at(0)))
	require.NoError(t, u.AddSegment(ctx, make([]byte, 200), at(2)))
	require.NoError(t, u.Cutover(ctx))
	require.NoError(t, u.AddSegment(ctx, make([]byte, 300), at(4)))
	require.NoError(t, u.Close(ctx))

	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.Equal(t, []time.Time{at(0), at(4)}, rec.started)
	require.Equal(t, [][]RecordedSegment{
		{{StartMs: at(0).UnixMilli(), Size: 100}, {StartMs: at(2).UnixMilli(), Size: 200}},
		{{StartMs: at(4).UnixMilli(), Size: 300}},
	}, rec.indexes)
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
		require.NoError(t, u.AddSegment(ctx, make([]byte, mb*1024*1024), time.Now()))
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

	require.NoError(t, u.AddSegment(ctx, seg, time.Now()))
	waitForStarts(t, rec, 1)           // object 1
	require.NoError(t, u.Cutover(ctx)) // complete fails -> object 1 abandoned+aborted

	require.NoError(t, u.AddSegment(ctx, seg, time.Now()))
	waitForStarts(t, rec, 2) // loop survived: object 2 started

	require.NoError(t, u.Close(ctx), "mid-stream failure must not surface at Close; the final object completed fine")

	fc.mu.Lock()
	defer fc.mu.Unlock()
	require.Equal(t, 2, fc.creates, "a fresh object must start after the failure")
	require.Equal(t, 1, fc.aborts, "the broken object must be aborted, not leaked")
	require.Equal(t, 1, fc.completes, "the post-failure object must complete")
	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.Equal(t, []string{"rec-1"}, rec.aborted, "the broken object's row must be dropped, not left open forever")
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
	if err := u.AddSegment(context.Background(), []byte("late"), time.Now()); err == nil {
		t.Fatalf("AddSegment after Close should return an error")
	}
}

func TestS3UploaderObjectKeysUniqueAcrossUploaders(t *testing.T) {
	fc, rec := &fakeUploadAPI{}, &fakeRecorder{}
	const prefix = "live-rec/did:plc:test/"
	a := newS3Uploader(fc, "bucket", "did:plc:test", prefix, time.Hour, rec)
	b := newS3Uploader(fc, "bucket", "did:plc:test", prefix, time.Hour, rec)
	ctx := t.Context()
	start := time.Now()
	require.NoError(t, a.AddSegment(ctx, []byte("a"), start))
	require.NoError(t, b.AddSegment(ctx, []byte("b"), start))
	require.NoError(t, a.Close(ctx))
	require.NoError(t, b.Close(ctx))

	keys := rec.startKeys()
	require.Len(t, keys, 2)
	require.NotEqual(t, keys[0], keys[1])
	for _, key := range keys {
		id, ok := strings.CutPrefix(key, prefix)
		require.True(t, ok)
		id, ok = strings.CutSuffix(id, ".m4s")
		require.True(t, ok)
		_, err := uuid.Parse(id)
		require.NoError(t, err)
	}
	require.Equal(t, []string{"*", "*"}, fc.conditions)
}

func TestS3UploaderPersistenceFailures(t *testing.T) {
	persistenceErr := errors.New("metadata write failed")
	t.Run("start failure aborts multipart without publishing", func(t *testing.T) {
		fc, rec := &fakeUploadAPI{}, &fakeRecorder{startErr: persistenceErr}
		u := newS3Uploader(fc, "bucket", "did:plc:test", "live/", time.Hour, rec)
		require.NoError(t, u.AddSegment(t.Context(), []byte("segment"), time.Now()))
		require.ErrorIs(t, u.Close(t.Context()), persistenceErr)
		require.Equal(t, 1, fc.aborts)
		require.Zero(t, fc.completes)
		require.Empty(t, fc.partSizes)
		require.Empty(t, rec.aborted, "start was not acknowledged")
		require.Empty(t, rec.indexes)
	})
	t.Run("completion failure cannot abort an already committed object", func(t *testing.T) {
		fc, rec := &fakeUploadAPI{}, &fakeRecorder{completeErr: persistenceErr}
		u := newS3Uploader(fc, "bucket", "did:plc:test", "live/", time.Hour, rec)
		require.NoError(t, u.AddSegment(t.Context(), []byte("segment"), time.Now()))
		require.NoError(t, u.Cutover(t.Context()))
		require.ErrorIs(t, u.Close(t.Context()), persistenceErr, "even a mid-stream completion failure must surface")
		require.Equal(t, 1, fc.completes)
		require.Zero(t, fc.aborts)
		require.Empty(t, rec.aborted, "the metadata commit may have succeeded before losing its ACK")
	})
	t.Run("metadata abort failure surfaces", func(t *testing.T) {
		fc, rec := &fakeUploadAPI{failCompletes: 1}, &fakeRecorder{abortErr: persistenceErr}
		u := newS3Uploader(fc, "bucket", "did:plc:test", "live/", time.Hour, rec)
		require.NoError(t, u.AddSegment(t.Context(), []byte("segment"), time.Now()))
		err := u.Close(t.Context())
		require.ErrorIs(t, err, persistenceErr)
		require.ErrorContains(t, err, "InvalidPart")
		require.Equal(t, 1, fc.aborts)
		require.Zero(t, fc.completes)
		require.Equal(t, []string{"rec-1"}, rec.aborted)
	})
}

func TestFencedS3UploaderStopsOnAmbiguousPersistence(t *testing.T) {
	persistenceErr := errors.New("metadata ACK lost")
	for _, duringStart := range []bool{true, false} {
		name := "complete"
		if duringStart {
			name = "start"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			client, rec := &abortProbe{}, &fakeRecorder{}
			if duringStart {
				rec.startErr = persistenceErr
			} else {
				rec.completeErr = persistenceErr
			}
			u := newS3UploaderWithFailureSignal(client, "bucket", "did:plc:test", "live/", time.Hour, rec, make(chan struct{}))
			t.Cleanup(func() { _ = u.Close(ctx) })
			require.NoError(t, u.AddSegment(ctx, []byte("segment"), time.Now()))
			if !duringStart {
				require.NoError(t, u.Cutover(ctx))
			}
			select {
			case <-u.PersistenceFailed():
			case <-ctx.Done():
				t.Fatal("metadata failure did not notify the lease holder")
			}
			require.Error(t, u.AddSegment(ctx, []byte("later"), time.Now()))
			require.ErrorIs(t, u.Close(ctx), persistenceErr)
			require.Equal(t, 1, rec.count())
			require.Empty(t, rec.aborted)
			if duringStart {
				require.Equal(t, 1, client.aborts)
				require.True(t, client.liveBoundedAbort)
				require.Zero(t, client.completes)
			} else {
				require.Equal(t, 1, client.completes)
				require.Zero(t, client.aborts)
			}
		})
	}
}
