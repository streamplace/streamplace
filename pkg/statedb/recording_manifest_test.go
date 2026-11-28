//go:build stationslate && linux && cgo

package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/stationshard"
)

func TestShardedRecordingManifestTakeover(t *testing.T) {
	cfg := stationtest.Config(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prefix := "recording-manifest-test/" + uuid.NewString()
	const did, owner = "did:plc:recordingtest", "did:plc:recordingorigin"
	const first, second = "at://" + did + "/place.stream.livestream/first", "at://" + did + "/place.stream.livestream/second"
	const ttl = 2 * time.Second
	objects := blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		keys, err := objects.List(ctx, prefix+"/")
		require.NoError(t, err)
		for _, key := range keys {
			require.NoError(t, objects.Delete(ctx, key))
		}
	})
	openStore := func() *stationshard.Store {
		store, err := stationshard.Open(stationshard.Config{S3: cfg, Prefix: prefix, Owner: owner, TTL: ttl})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		return store
	}
	a, b := openStore(), openStore()
	t0 := time.Now().UTC().Truncate(time.Millisecond).Add(-time.Hour)
	stateA, err := MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	sqlDB, err := stateA.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	legacyAt := t0.Add(-2 * time.Second)
	legacyEnd, duplicateEnd := t0.Add(-time.Second), t0.Add(3*time.Second)
	legacyKey, duplicateKey := prefix+"/media/legacy.m4s", prefix+"/media/a.m4s"
	require.NoError(t, stateA.DB.WithContext(ctx).Create(&[]S3Segment{
		{ID: "legacy", RepoDID: did, Bucket: cfg.Bucket, Key: legacyKey, LivestreamURI: first, StartedAt: legacyAt, CompletedAt: &legacyEnd},
		{ID: "legacy-copy", RepoDID: did, Bucket: cfg.Bucket, Key: duplicateKey, LivestreamURI: first, StartedAt: t0.Add(time.Second), CompletedAt: &duplicateEnd, Size: 99},
		{ID: "legacy-open", RepoDID: did, Bucket: cfg.Bucket, Key: prefix + "/media/legacy-open.m4s", LivestreamURI: first, StartedAt: t0.Add(20 * time.Second)},
	}).Error)
	// Both readers see the same imported SQL snapshot.
	stateB := &StatefulDB{DB: stateA.DB, StationShards: b}
	stateA.StationShards = a
	leaseA, err := a.ClaimRole(ctx, did, recordingRole)
	require.NoError(t, err)
	writerA, err := stateA.NewRecordingWriter(ctx, leaseA)
	require.NoError(t, err)

	start := func(writer s3.Recorder, chapter, name string, at time.Time) (id, key string) {
		t.Helper()
		key = prefix + "/media/" + name + ".m4s"
		id, err := writer.RecordStart(ctx, did, cfg.Bucket, key, chapter, at)
		require.NoError(t, err)
		return id, key
	}
	idA, keyA := start(writerA, first, "a", t0.Add(3*time.Second))
	indexA := []s3.RecordedSegment{{StartMs: t0.Add(3 * time.Second).UnixMilli(), Size: 3}, {StartMs: t0.UnixMilli(), Size: 4}}

	// A successfully staged immutable PUT is not a published completion.
	stagedAt := time.Now().UTC().Truncate(time.Millisecond)
	staged, err := json.Marshal(recordingIndex{CompletedAt: stagedAt, Parts: 1, Size: 7, Segments: indexA})
	require.NoError(t, err)
	require.NoError(t, a.PutImmutable(ctx, did, recordingIndexKey(idA), staged))
	before, err := stateB.ListS3SegmentsForLivestream(ctx, first)
	require.NoError(t, err)
	require.Equal(t, []string{legacyKey, duplicateKey}, RecordingObjectKeys(before))
	require.Equal(t, "legacy-copy", before[1].ID, "staging must not replace the imported row")
	require.NoError(t, writerA.RecordComplete(ctx, idA, 1, 7, indexA))
	require.NoError(t, writerA.RecordComplete(ctx, idA, 1, 7, indexA))
	require.ErrorIs(t, writerA.RecordComplete(ctx, idA, 1, 8, indexA), stationshard.ErrConflict)
	require.ErrorIs(t, writerA.RecordAbort(ctx, idA), stationshard.ErrConflict)
	require.NoError(t, a.RenewRole(ctx, leaseA))
	pausedID, pausedKey := start(writerA, first, "paused", t0.Add(10*time.Second))
	retryID, err := writerA.RecordStart(ctx, did, cfg.Bucket, pausedKey, first, t0.Add(10*time.Second))
	require.NoError(t, err)
	require.Equal(t, pausedID, retryID)
	count, err := stateB.CountOpenS3Segments(ctx, first)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	paused, err := stateB.LatestOpenS3Segment(ctx, did, t0)
	require.NoError(t, err)
	require.NotNil(t, paused)
	require.Equal(t, pausedID, paused.ID)
	_, err = b.ClaimRole(ctx, did, recordingRole)
	require.ErrorIs(t, err, stationshard.ErrHeld)

	var leaseB stationshard.Lease
	deadline := time.Now().Add(3 * ttl)
	for {
		leaseB, err = b.ClaimRole(ctx, did, recordingRole)
		if !errors.Is(err, stationshard.ErrHeld) {
			require.NoError(t, err)
			break
		}
		require.True(t, time.Now().Before(deadline), "record lease did not expire")
		time.Sleep(50 * time.Millisecond)
	}
	require.Equal(t, leaseA.Owner, leaseB.Owner)
	require.NotEqual(t, leaseA.Incarnation, leaseB.Incarnation)
	require.Greater(t, leaseB.Epoch, leaseA.Epoch)
	count, err = stateB.CountOpenS3Segments(ctx, first)
	require.NoError(t, err)
	require.Zero(t, count, "the successor need not activate to hide a fenced open")
	writerB, err := stateB.NewRecordingWriter(ctx, leaseB)
	require.NoError(t, err)
	count, err = stateB.CountOpenS3Segments(ctx, first)
	require.NoError(t, err)
	require.Zero(t, count)
	open, err := stateB.LatestOpenS3Segment(ctx, did, t0)
	require.NoError(t, err)
	require.Nil(t, open)

	// Complete B with an earlier media start than A, despite publishing later.
	tB := t0.Add(-time.Second)
	idB, keyB := start(writerB, second, "b", tB)
	_, err = writerA.RecordStart(ctx, did, cfg.Bucket, prefix+"/media/late.m4s", first, t0)
	require.ErrorIs(t, err, stationshard.ErrFenced)
	require.ErrorIs(t, writerA.RecordComplete(ctx, pausedID, 1, 7, indexA), stationshard.ErrFenced)
	require.ErrorIs(t, writerA.RecordAbort(ctx, pausedID), stationshard.ErrFenced)
	require.ErrorIs(t, writerA.RecordAbort(ctx, idB), stationshard.ErrFenced)
	open, err = stateA.LatestOpenS3Segment(ctx, did, tB)
	require.NoError(t, err)
	require.NotNil(t, open)
	require.Equal(t, idB, open.ID)
	_, err = a.GetImmutable(ctx, did, recordingIndexKey(pausedID))
	require.ErrorIs(t, err, stationshard.ErrNotFound)
	indexB := []s3.RecordedSegment{{StartMs: tB.UnixMilli(), Size: 4}}
	require.NoError(t, writerB.RecordComplete(ctx, idB, 1, 4, indexB))
	require.NoError(t, b.ReleaseRole(ctx, leaseB))

	segs, err := stateA.ListS3SegmentsForLivestreams(ctx, []string{first, second})
	require.NoError(t, err)
	require.Equal(t, []string{legacyKey, keyB, keyA}, RecordingObjectKeys(segs))
	require.Equal(t, "legacy", segs[0].ID)
	require.Equal(t, idB, segs[1].ID)
	require.Equal(t, idA, segs[2].ID)
	require.Equal(t, indexA, segs[2].Segments)
	require.Equal(t, stagedAt, *segs[2].CompletedAt)
	require.Equal(t, cfg.Bucket, segs[2].Bucket)
	require.EqualValues(t, 7, segs[2].Size)
	require.EqualValues(t, 1, segs[2].PartCount)
	require.Equal(t, t0, segs[2].StartedAt)
	segs, err = stateB.ListS3SegmentsInRange(ctx, did, legacyAt, t0.Add(20*time.Second))
	require.NoError(t, err)
	require.Equal(t, []string{legacyKey, keyB, keyA}, RecordingObjectKeys(segs))
	require.Equal(t, idA, segs[2].ID)
	segs, err = stateA.ListS3SegmentsInRange(ctx, did, t0.Add(3*time.Second), t0.Add(3*time.Second))
	require.NoError(t, err)
	require.Equal(t, []string{keyA}, RecordingObjectKeys(segs))
	require.Equal(t, idA, segs[0].ID)
	segs, err = stateA.ListS3SegmentsInRange(ctx, did, t0.Add(4*time.Second), t0.Add(5*time.Second))
	require.NoError(t, err)
	require.Empty(t, segs, "wall-clock completion is not a media bound")

	control, err := readRecordingRole(ctx, a, did)
	require.NoError(t, err)
	require.Nil(t, control.Open)
	page, err := readRecordingPage(ctx, a, did, control.Head)
	require.NoError(t, err)
	require.Len(t, page.Entries, 2)
	require.Equal(t, recordingWriterFence(leaseA), page.Entries[0].Fence)
	require.Equal(t, recordingWriterFence(leaseB), page.Entries[1].Fence)
	captured, err := a.GetImmutable(ctx, did, recordingIndexKey(idB))
	require.NoError(t, err)
	var metadataB recordingIndex
	require.NoError(t, json.Unmarshal(captured, &metadataB))
	require.Equal(t, indexB, metadataB.Segments)
	require.EqualValues(t, 4, metadataB.Size)

	require.NoError(t, b.Close())
	segs, err = stateB.ListS3SegmentsForLivestream(ctx, first)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.Nil(t, segs)
	segs, err = stateB.ListS3SegmentsInRange(ctx, did, legacyAt, t0)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.Nil(t, segs)

	require.NoError(t, sqlDB.Close())
	segs, err = stateA.ListS3SegmentsForLivestream(ctx, first)
	require.Error(t, err)
	require.Nil(t, segs)
	segs, err = stateA.ListS3SegmentsInRange(ctx, did, legacyAt, t0)
	require.Error(t, err)
	require.Nil(t, segs)
}

func TestShardedRecordingRejectsUnboundWritesAndInvalidAuthority(t *testing.T) {
	state := &StatefulDB{StationShards: &stationshard.Store{}}
	ctx := context.Background()
	_, err := state.RecordStart(ctx, "did:plc:one", "bucket", "key", "", time.Now())
	require.ErrorIs(t, err, errRecordingLeaseRequired)
	require.ErrorIs(t, state.RecordComplete(ctx, "id", 1, 4, nil), errRecordingLeaseRequired)
	require.ErrorIs(t, state.RecordAbort(ctx, "id"), errRecordingLeaseRequired)
	_, err = state.GetS3Segment(ctx, "id")
	require.ErrorContains(t, err, "station DID")
	_, err = state.ListS3SegmentsForLivestreams(ctx, []string{
		"at://did:plc:one/place.stream.livestream/first",
		"at://did:plc:two/place.stream.livestream/second",
	})
	require.ErrorContains(t, err, "different DIDs")
	_, err = state.CountOpenS3Segments(ctx, "at://example.com/place.stream.livestream/chapter")
	require.ErrorContains(t, err, "DID authority")
	_, err = state.CountOpenS3Segments(ctx, "at://did:plc:one/place.stream.livestream")
	require.ErrorContains(t, err, "livestream record URI")
}
