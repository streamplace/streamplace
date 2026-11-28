//go:build stationslate && linux && cgo

package vod

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/stationshard"
)

type handoffStartObserver struct {
	s3.Recorder
	started chan string
}

func (r handoffStartObserver) RecordStart(ctx context.Context, did, bucket, key, uri string, started time.Time) (string, error) {
	id, err := r.Recorder.RecordStart(ctx, did, bucket, key, uri, started)
	if err == nil {
		r.started <- key
	}
	return id, err
}

func TestStationRecordingHandoffVOD(t *testing.T) {
	cfg := stationtest.Config(t)
	segs := liveSegments(t)
	require.Len(t, segs, 2, "fixture GoPs")
	oracle := flatOracle(t, segs)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	id := uuid.NewString()
	prefix := "station-recording-handoff/" + id + "/"
	did, owner := "did:test:streamer-"+id, "did:test:node-"+id
	uri := "at://" + did + "/place.stream.livestream/handoff"
	at0 := time.Now().UTC().Truncate(time.Millisecond).Add(-time.Minute)
	at1 := at0.Add(2 * time.Second)
	client := s3.NewClient(cfg)
	store := blob.NewS3Store(client, cfg.Bucket)
	t.Cleanup(func() { cleanHandoffPrefix(t, store, prefix) })

	open := func(ttl time.Duration) *stationshard.Store {
		shard, err := stationshard.Open(stationshard.Config{S3: cfg, Prefix: prefix + "shards", Owner: owner, TTL: ttl})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, shard.Close()) })
		return shard
	}
	const ttl = 2 * time.Second
	shardA, shardB := open(ttl), open(30*time.Second)
	stateA, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	sqlDB, err := stateA.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	legacyAt, legacyKey := at0.Add(-2*time.Second), prefix+"raw/legacy.m4s"
	require.NoError(t, writeBlob(ctx, store, legacyKey, segs[0]))
	legacyID, err := stateA.RecordStart(ctx, did, cfg.Bucket, legacyKey, "at://"+did+"/place.stream.livestream/legacy", legacyAt)
	require.NoError(t, err)
	legacyIndex := []s3.RecordedSegment{{StartMs: legacyAt.UnixMilli(), Size: int64(len(segs[0]))}}
	require.NoError(t, stateA.RecordComplete(ctx, legacyID, 1, int64(len(segs[0])), legacyIndex))
	_, err = stateA.RecordStart(ctx, did, cfg.Bucket, prefix+"raw/legacy-open.m4s", uri, at0)
	require.NoError(t, err)
	// Both readers see the same imported SQL snapshot.
	stateB := &statedb.StatefulDB{DB: stateA.DB, StationShards: shardB}
	stateA.StationShards = shardA

	start := func(recorder s3.Recorder, seg []byte, at time.Time) (*s3.S3Uploader, string) {
		started := make(chan string, 1)
		up := s3.NewS3Uploader(cfg, did, prefix+"raw/", time.Hour, handoffStartObserver{recorder, started})
		t.Cleanup(func() {
			_ = up.Close(context.Background())
		})
		up.SetLivestreamURI(uri)
		require.NoError(t, up.AddSegment(ctx, seg, at))
		select {
		case key := <-started:
			return up, key
		case <-ctx.Done():
			t.Fatalf("waiting for durable recording start: %v", up.Close(ctx))
			return nil, ""
		}
	}
	listKeys := func(state *statedb.StatefulDB) []string {
		entries, err := state.ListS3SegmentsForLivestreams(ctx, []string{uri})
		require.NoError(t, err)
		return statedb.RecordingObjectKeys(entries)
	}
	readObject := func(key string) []byte {
		r, err := store.Open(ctx, key)
		require.NoError(t, err)
		defer r.Close()
		data, err := io.ReadAll(io.NewSectionReader(r, 0, r.Size()))
		require.NoError(t, err)
		return data
	}

	leaseA, err := shardA.ClaimRole(ctx, did, "record")
	require.NoError(t, err)
	recorderA, err := stateA.NewRecordingWriter(ctx, leaseA)
	require.NoError(t, err)
	a0, keyA0 := start(recorderA, segs[0], at0)
	require.NoError(t, a0.Close(ctx))
	require.NoError(t, shardA.RenewRole(ctx, leaseA))
	late, lateKey := start(recorderA, segs[1], at1)
	_, err = shardB.ClaimRole(ctx, did, "record")
	require.ErrorIs(t, err, stationshard.ErrHeld)

	// A stops renewing but leaves its multipart upload and writer alive.
	var leaseB stationshard.Lease
	require.Eventually(t, func() bool {
		leaseB, err = shardB.ClaimRole(ctx, did, "record")
		return !errors.Is(err, stationshard.ErrHeld)
	}, 3*ttl, 100*time.Millisecond, "B should acquire the expired recording role")
	require.NoError(t, err)
	require.NotEqual(t, leaseA.Incarnation, leaseB.Incarnation)
	require.Greater(t, leaseB.Epoch, leaseA.Epoch)
	recorderB, err := stateB.NewRecordingWriter(ctx, leaseB)
	require.NoError(t, err)
	require.Equal(t, []string{keyA0}, listKeys(stateB), "takeover must retain A's completed recording")
	b1, keyB1 := start(recorderB, segs[1], at1)
	require.NoError(t, b1.Close(ctx))

	require.ErrorIs(t, late.Close(ctx), stationshard.ErrFenced)
	require.True(t, bytes.Equal(segs[1], readObject(lateKey)), "A's late multipart must complete in S3")
	keys := listKeys(stateB)
	require.Equal(t, []string{keyA0, keyB1}, keys, "the late object must not enter the recording catalog")

	clip, err := FindClip(ctx, stateB, store, fakeBuffer{}, did, at0, at1, false)
	require.NoError(t, err)
	var clipped bytes.Buffer
	require.NoError(t, clip.WriteMP4(ctx, &clipped))
	require.True(t, bytes.Equal(oracle, clipped.Bytes()), "public clip differs from flat MUXL oracle")

	header, err := synthFlatHeaderForObjects(ctx, store, keys)
	require.NoError(t, err)
	contentKey := prefix + "final.mp4"
	require.NoError(t, assembleContentBlob(ctx, store, header, keys, contentKey))
	require.True(t, bytes.Equal(oracle, readObject(contentKey)), "finalized S3 object differs from flat MUXL oracle")
}

func cleanHandoffPrefix(t *testing.T, store *blob.S3Store, prefix string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, bucket := store.Client(), aws.String(store.Bucket())
	pages := awss3.NewListMultipartUploadsPaginator(client, &awss3.ListMultipartUploadsInput{
		Bucket: bucket, Prefix: aws.String(prefix),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			t.Errorf("list unfinished handoff uploads: %v", err)
			break
		}
		for _, upload := range page.Uploads {
			if strings.HasPrefix(aws.ToString(upload.Key), prefix) {
				_, err := client.AbortMultipartUpload(ctx, &awss3.AbortMultipartUploadInput{
					Bucket: bucket, Key: upload.Key, UploadId: upload.UploadId,
				})
				if err != nil {
					t.Errorf("abort unfinished handoff upload: %v", err)
				}
			}
		}
	}
	keys, err := store.List(ctx, prefix)
	require.NoError(t, err)
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			if err := store.Delete(ctx, key); err != nil {
				t.Errorf("delete handoff object: %v", err)
			}
		}
	}
}
