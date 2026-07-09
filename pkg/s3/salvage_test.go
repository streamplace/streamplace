package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSalvageSpools proves the full crash-recovery round trip: an uploader
// whose bucket is broken leaves its spool behind at Close, and the next
// startup's salvage pass uploads every byte, tags objects with the right
// livestream URIs, and cleans the directory away.
func TestSalvageSpools(t *testing.T) {
	root := filepath.Join(t.TempDir(), "live-rec-spool")
	did := "did:plc:salvage"
	dir := filepath.Join(root, did, "1234567890")

	// Phase 1: a stream records against a permanently broken bucket.
	spool, err := OpenSpool(dir, 1<<30)
	require.NoError(t, err)
	broken := &fakeUploadAPI{failCompletes: 1 << 30}
	rec := &fakeRecorder{}
	u := newS3Uploader(broken, "bucket", did, "live-rec/"+did+"/", time.Hour, rec, spool)
	ctx := context.Background()
	u.SetLivestreamURI("at://A")
	require.NoError(t, u.AddSegment(ctx, make([]byte, 300*1024)))
	waitForStarts(t, rec, 1)
	u.SetLivestreamURI("at://B")
	require.NoError(t, u.AddSegment(ctx, make([]byte, 200*1024)))
	require.Error(t, u.Close(ctx), "close must surface that segments were left behind")

	// Phase 2: "restart" — the bucket is healthy again and salvage runs.
	healthy := &fakeUploadAPI{}
	rec2 := &fakeRecorder{}
	require.NoError(t, salvageSpools(ctx, healthy, "bucket", rec2, root,
		func(d string) string { return "live-rec/" + d + "/" }, time.Hour, time.Now()))

	healthy.mu.Lock()
	defer healthy.mu.Unlock()
	require.Equal(t, 2, healthy.completes, "one object per livestream URI epoch")
	uploaded := 0
	for _, s := range healthy.partSizes {
		uploaded += s
	}
	require.Equal(t, (300+200)*1024, uploaded, "every spooled byte must be salvaged")
	require.Equal(t, []string{"at://A", "at://B"}, rec2.startURIs(),
		"salvaged objects must keep their livestream tagging")

	_, err = os.Stat(dir)
	require.True(t, os.IsNotExist(err), "drained spool dir must be removed")
	_, err = os.Stat(filepath.Join(root, did))
	require.True(t, os.IsNotExist(err), "empty did dir must be removed")
}

// TestSalvageSpoolsSkipsLiveSessions proves the boot-time cutoff: a session
// dir whose UnixNano name is at/after startedBefore belongs to a live session
// of this process and must not be touched — that's what makes salvage safe to
// run concurrently with new streams at startup.
func TestSalvageSpoolsSkipsLiveSessions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "live-rec-spool")
	did := "did:plc:live"
	cutoff := time.Now()
	liveDir := filepath.Join(root, did, fmt.Sprintf("%d", time.Now().Add(time.Second).UnixNano()))

	spool, err := OpenSpool(liveDir, 0)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = spool.Append(ctx, make([]byte, 1024), "at://A")
	require.NoError(t, err)

	healthy := &fakeUploadAPI{}
	rec := &fakeRecorder{}
	require.NoError(t, salvageSpools(ctx, healthy, "bucket", rec, root,
		func(d string) string { return "live-rec/" + d + "/" }, time.Hour, cutoff))

	healthy.mu.Lock()
	defer healthy.mu.Unlock()
	require.Equal(t, 0, healthy.creates, "a live session's spool must not be salvaged")
	s2, err := OpenSpool(liveDir, 0)
	require.NoError(t, err)
	require.Equal(t, 1, s2.Len(), "the live session's segments must be untouched")
}

// TestSalvageSpoolsPartialFailure proves a mid-salvage failure keeps the
// un-uploaded remainder on disk for the next attempt, without re-uploading
// what already completed.
func TestSalvageSpoolsPartialFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "live-rec-spool")
	did := "did:plc:partial"
	dir := filepath.Join(root, did, "42")

	spool, err := OpenSpool(dir, 0)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = spool.Append(ctx, make([]byte, 100*1024), "at://A")
	require.NoError(t, err)
	_, err = spool.Append(ctx, make([]byte, 150*1024), "at://B")
	require.NoError(t, err)

	// The first complete (object A) succeeds, the second (object B) fails.
	flaky := &fakeUploadAPI{completesBeforeFail: 1, failCompletes: 1 << 30}
	rec := &fakeRecorder{}
	require.NoError(t, salvageSpools(ctx, flaky, "bucket", rec, root,
		func(d string) string { return "live-rec/" + d + "/" }, time.Hour, time.Now()))

	// Object A's segment was acked away; B's remains for the next run.
	s2, err := OpenSpool(dir, 0)
	require.NoError(t, err)
	require.Equal(t, 1, s2.Len(), "failed object's segments must survive")
	require.Equal(t, int64(150*1024), s2.Bytes())

	// Next run with a healthy bucket finishes the job.
	healthy := &fakeUploadAPI{}
	rec3 := &fakeRecorder{}
	require.NoError(t, salvageSpools(ctx, healthy, "bucket", rec3, root,
		func(d string) string { return "live-rec/" + d + "/" }, time.Hour, time.Now()))
	healthy.mu.Lock()
	defer healthy.mu.Unlock()
	require.Equal(t, 1, healthy.completes)
	require.Equal(t, []string{"at://B"}, rec3.startURIs())
	_, err = os.Stat(dir)
	require.True(t, os.IsNotExist(err))
}
