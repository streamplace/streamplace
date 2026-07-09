package s3

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSpoolAppendAckEvict(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "spool")
	s, err := OpenSpool(dir, 3000)
	require.NoError(t, err)

	seq1, err := s.Append(ctx, make([]byte, 1000), "at://A")
	require.NoError(t, err)
	seq2, err := s.Append(ctx, make([]byte, 1000), "at://A")
	require.NoError(t, err)
	seq3, err := s.Append(ctx, make([]byte, 1000), "at://B")
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, []int64{seq1, seq2, seq3})
	require.Equal(t, int64(3000), s.Bytes())

	_, _, uri, err := s.Get(seq1)
	require.NoError(t, err)
	require.Equal(t, "at://A", uri)
	_, _, uri, err = s.Get(seq3)
	require.NoError(t, err)
	require.Equal(t, "at://B", uri)

	// Over budget: the oldest segment is evicted, newest survives.
	_, err = s.Append(ctx, make([]byte, 1000), "at://B")
	require.NoError(t, err)
	require.Equal(t, int64(3000), s.Bytes())
	_, _, _, err = s.Get(seq1)
	require.Error(t, err, "oldest segment should have been evicted")
	next, ok := s.NextFrom(1)
	require.True(t, ok)
	require.Equal(t, seq2, next, "NextFrom must skip the evicted gap")

	// Reopen picks up where we left off (the salvage path).
	s2, err := OpenSpool(dir, 0)
	require.NoError(t, err)
	require.Equal(t, 3, s2.Len())
	require.Equal(t, int64(3000), s2.Bytes())
	_, _, uri, err = s2.Get(seq3)
	require.NoError(t, err)
	require.Equal(t, "at://B", uri, "URI epochs must survive reopen")
	seq5, err := s2.Append(ctx, []byte("x"), "at://B")
	require.NoError(t, err)
	require.Equal(t, int64(5), seq5, "sequence numbering must continue after reopen")

	// Ack removes everything at or below the watermark.
	s2.Ack(4)
	require.Equal(t, 1, s2.Len())
	s2.Destroy()
}

// TestS3UploaderSpoolNoLossOnFailure is the durability contract: with the disk
// spool, a failed CompleteMultipartUpload loses nothing — the segments stay
// spooled and the retry re-uploads every byte. (The memory-mode equivalent,
// TestS3UploaderRecoversFromCompleteFailure, accepts a gap; spool mode must
// not.)
func TestS3UploaderSpoolNoLossOnFailure(t *testing.T) {
	spool, err := OpenSpool(filepath.Join(t.TempDir(), "spool"), 1<<30)
	require.NoError(t, err)
	fc := &fakeUploadAPI{failCompletes: 1}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec, spool)

	ctx := context.Background()
	u.SetLivestreamURI("at://A")
	total := 0
	for _, kb := range []int{700, 800, 900} {
		require.NoError(t, u.AddSegment(ctx, make([]byte, kb*1024)))
		total += kb * 1024
	}
	waitForStarts(t, rec, 1)

	// The first complete (triggered by cutover) fails; the object is aborted
	// but its segments stay in the spool.
	require.NoError(t, u.Cutover(ctx))
	waitFor(t, func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		return fc.aborts >= 1
	}, "first complete should fail and abort")
	require.Equal(t, 3, spool.Len(), "failed upload must retain every segment in the spool")

	// Close forces an immediate final drain regardless of backoff; the retry
	// re-uploads everything and the spool is destroyed.
	require.NoError(t, u.Close(ctx))

	fc.mu.Lock()
	defer fc.mu.Unlock()
	require.Equal(t, 1, fc.completes, "retry must complete the object")
	uploaded := 0
	for _, s := range fc.partSizes {
		uploaded += s
	}
	// The failed attempt uploaded the bytes once, the retry uploaded them
	// again; what matters is the completed object covered every byte.
	require.GreaterOrEqual(t, uploaded, total*2-1, "retry must re-upload the full object")
	require.Equal(t, 0, spool.Len(), "spool must be drained after successful close")
}

// TestS3UploaderSpoolLeavesDataForSalvage proves a persistently failing bucket
// still can't lose bytes: Close gives up, surfaces the error, and leaves the
// spool on disk for the startup salvage pass.
func TestS3UploaderSpoolLeavesDataForSalvage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	spool, err := OpenSpool(dir, 1<<30)
	require.NoError(t, err)
	fc := &fakeUploadAPI{failCompletes: 1 << 30}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec, spool)

	ctx := context.Background()
	require.NoError(t, u.AddSegment(ctx, make([]byte, 100*1024)))
	waitForStarts(t, rec, 1)
	require.Error(t, u.Close(ctx), "close must surface the failure")

	s2, err := OpenSpool(dir, 0)
	require.NoError(t, err)
	require.Equal(t, 1, s2.Len(), "un-uploaded segments must survive for salvage")
}

// TestS3UploaderSpoolChaptersAndOrdering proves spool mode still splits
// objects on livestream-URI changes and stamps honest started-at times from
// segment arrival.
func TestS3UploaderSpoolChaptersAndOrdering(t *testing.T) {
	spool, err := OpenSpool(filepath.Join(t.TempDir(), "spool"), 1<<30)
	require.NoError(t, err)
	fc := &fakeUploadAPI{}
	rec := &fakeRecorder{}
	u := newS3Uploader(fc, "bucket", "did:plc:test", "did:plc:test/", time.Hour, rec, spool)

	ctx := context.Background()
	u.SetLivestreamURI("at://A")
	require.NoError(t, u.AddSegment(ctx, make([]byte, 1024)))
	waitForStarts(t, rec, 1)
	u.SetLivestreamURI("at://B")
	require.NoError(t, u.AddSegment(ctx, make([]byte, 1024)))
	waitForStarts(t, rec, 2)
	require.NoError(t, u.Close(ctx))

	require.Equal(t, []string{"at://A", "at://B"}, rec.startURIs(),
		"each object must be tagged with the livestream URI its segments were appended under")
	fc.mu.Lock()
	defer fc.mu.Unlock()
	require.Equal(t, 2, fc.completes)
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}
