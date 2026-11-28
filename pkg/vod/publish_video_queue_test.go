package vod

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/statedb"
)

func TestQueuedPublishVideoReplaysWithoutWritableOAuth(t *testing.T) {
	for _, tc := range []struct {
		name string
		want statedb.TaskStatus
	}{
		{"catalog ready", statedb.TaskStatusCompleted},
		{"catalog cannot advertise", statedb.TaskStatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			state := newDraftTestState(t)
			state.CLI.StationDID = "did:web:station.example"
			store, err := blob.NewFileStore(t.TempDir())
			require.NoError(t, err)
			const owner = "did:plc:ewvi7nxzyoun6zhxrhs64oiz"
			const uploadID = "finished"
			const videoURI = "at://" + owner + "/place.stream.video/original-rkey"
			const recordCID = "bafy-original-record"
			content, init := []byte("VOD media"), []byte("init data")
			cid, initCID := bdasl.CID(content), bdasl.CID(init)
			require.NoError(t, state.CreateLivestreamUpload(ctx, uploadID, owner, "at://"+owner+"/place.stream.livestream/live"))
			require.NoError(t, state.SetUploadProcessed(ctx, uploadID, 1000, cid, "did:key:signing", `{"durationMs":1000}`, int64(len(content))))
			if tc.want == statedb.TaskStatusCompleted {
				require.NoError(t, blob.CreateOnce(ctx, store, BlobsPrefix+cid+".mp4", content))
				require.NoError(t, blob.CreateOnce(ctx, store, BlobsPrefix+initCID+".mp4", init))
				require.NoError(t, writeMetafile(ctx, store, cid, &Metafile{
					BlobCID: cid, BlobSize: int64(len(content)),
					Tracks: map[string]MetafileTrack{"1": {InitCID: initCID, BlobCID: cid, Type: "video"}},
				}))
			}
			_, err = state.EnqueueTask(ctx, statedb.TaskFinalizeLivestreamVOD, statedb.FinalizeLivestreamVODTask{
				UploadID: uploadID, RepoDID: owner, Publish: &statedb.VideoDraft{Title: "Replay"},
			}, statedb.WithMaxTries(2))
			require.NoError(t, err)
			first, err := state.DequeueTask(ctx, "first", statedb.TaskFinalizeLivestreamVOD)
			require.NoError(t, err)
			require.NotNil(t, first)
			receipt, err := json.Marshal(comatproto.RepoPutRecord_Output{
				Uri: videoURI, Cid: recordCID, Commit: &comatproto.RepoDefs_CommitMeta{Rev: "2222222222223"},
			})
			require.NoError(t, err)
			require.NoError(t, state.DB.Create(&statedb.AppTaskEffect{
				TaskID: first.ID, EffectKey: "video", Target: videoURI, Receipt: receipt,
			}).Error)
			require.NoError(t, state.ReleaseTask(ctx, first.ID, first.ClaimToken))
			// No OAuth session or proxy is configured, and the claim has a confirmed receipt.
			type result struct {
				uri, cid   string
				foreignErr error
				err        error
			}
			published := make(chan result, 1)
			state.SetVideoPublisher(func(ctx context.Context, task statedb.FinalizeLivestreamVODTask) (string, string, error) {
				_, _, foreignErr := PublishVideo(ctx, state, store, "did:web:other.example", task.UploadID, task.Publish.Record())
				uri, cid, err := PublishVideo(ctx, state, store, task.RepoDID, task.UploadID, task.Publish.Record())
				published <- result{uri, cid, foreignErr, err}
				return uri, cid, err
			})
			workerCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			finished := make(chan error, 1)
			go func() { finished <- state.ProcessQueue(workerCtx, 1) }()
			t.Cleanup(func() {
				stop()
				require.ErrorIs(t, <-finished, workerCtx.Err())
			})
			select {
			case out := <-published:
				require.ErrorIs(t, out.foreignErr, ErrUploadNotFound)
				if tc.want == statedb.TaskStatusCompleted {
					require.NoError(t, out.err)
					require.Equal(t, videoURI, out.uri)
					require.Equal(t, recordCID, out.cid)
				} else {
					require.ErrorIs(t, out.err, blob.ErrNotFound)
				}
			case <-workerCtx.Done():
				t.Fatal("queue did not attempt replay")
			}
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				row, err := state.GetTask(ctx, first.ID)
				require.NoError(c, err)
				require.NotNil(c, row)
				require.Equal(c, tc.want, row.Status)
			}, time.Second, 10*time.Millisecond)
			entry, advertised, err := stationOriginState(ctx, store, state.CLI.StationDID, cid)
			require.NoError(t, err)
			require.Equal(t, tc.want == statedb.TaskStatusCompleted, advertised)
			if advertised {
				require.Equal(t, []string{videoURI}, entry.sources)
			}
		})
	}
}
