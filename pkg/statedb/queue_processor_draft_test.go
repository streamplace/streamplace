package statedb

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

// The worker must create an implicit draft before processing can finish.
func TestDraftLifecycleThroughVODProcessor(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:draft"

		require.NoError(t, state.CreateUpload(ctx, &Upload{
			ID: "up-lifecycle", RepoDID: did, MimeType: "video/mp4", Backend: "file", Filename: "original.mp4",
		}))

		// Fake processor: simulates vod.ProcessVOD's internal SetUploadProcessed
		// (writes the finished fields onto the Upload row) then returns a cid.
		// Tracks are deferred to publishDraft time now, so SetUploadProcessed
		// stores the probe + signingKey (not track refs).
		probeJSON := `{"durationMs":98765,"video":{"codec":"h264","width":1280,"height":720,"fpsNum":30,"fpsDen":1}}`
		state.SetVODProcessor(func(ctx context.Context, task VODProcessTask) (string, error) {
			draft, err := state.getDraftByUpload(ctx, task.UploadID)
			require.NoError(t, err)
			require.NotNil(t, draft)
			require.NoError(t, state.SetUploadProcessed(ctx, task.UploadID, 98765, "muxlcid-lifecycle", "did:key:signing", probeJSON, 123456))
			return "muxlcid-lifecycle", nil
		})

		task := enqueueClaimedTask(t, state, TaskVODProcess, VODProcessTask{
			UploadID: "up-lifecycle", RepoDID: did, MimeType: "video/mp4", Backend: "file",
		})
		require.NoError(t, state.runClaimedTask(ctx, task, time.Hour))
		requireTaskStatus(t, state, task.ID, TaskStatusCompleted)
		// The draft must now be 'ready'. durationMs + content_cid are filled
		// from the upload row; source is nil (tracks are published at
		// publishDraft time, not at ready time).
		dv, err := state.getDraftByUpload(ctx, "up-lifecycle")
		require.NoError(t, err)
		require.NotNil(t, dv)
		require.Equal(t, "muxlcid-lifecycle", dv.ContentCID, "content_cid must be backfilled from the upload row")
		rec, err := unmarshalDraft(dv.Data)
		require.NoError(t, err)
		require.Equal(t, "ready", rec.Status)
		require.Equal(t, "original.mp4", rec.Title)
		require.NotNil(t, rec.DurationMs)
		require.Equal(t, int64(98765), *rec.DurationMs)
		require.Nil(t, rec.Source, "source must be nil at ready time (tracks deferred to publish)")
		// The Upload row carries the deferred track-publishing inputs.
		up, err := state.GetUpload(ctx, "up-lifecycle")
		require.NoError(t, err)
		require.Equal(t, "did:key:signing", up.SigningKey)
		require.Contains(t, up.ProbeJSON, "h264")
	})
}

// TestDraftLifecycleErrorFlipsDraft verifies a failed processor flips the tied
// draft to 'error' so the user isn't stuck looking at 'processing' forever.
func TestDraftLifecycleErrorFlipsDraft(t *testing.T) {
	cli := &config.CLI{DBURL: ":memory:"}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := MakeDB(t.Context(), cli, nil, mod)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, state.CreateUpload(ctx, &Upload{ID: "up-fail", RepoDID: "did:plc:err", Backend: "file"}))
	// No pre-created draft: even a synchronous failure must reach the implicit draft.

	state.SetVODProcessor(func(ctx context.Context, task VODProcessTask) (string, error) {
		return "", errBoom
	})

	task := enqueueClaimedTask(t, state, TaskVODProcess, VODProcessTask{
		UploadID: "up-fail", RepoDID: "did:plc:err", Backend: "file",
	}, WithMaxTries(1))
	require.ErrorIs(t, state.runClaimedTask(ctx, task, time.Hour), errBoom)
	requireTaskStatus(t, state, task.ID, TaskStatusFailed)

	dv, err := state.getDraftByUpload(ctx, "up-fail")
	require.NoError(t, err)
	require.NotNil(t, dv)
	rec, err := unmarshalDraft(dv.Data)
	require.NoError(t, err)
	require.Equal(t, "error", rec.Status)
	require.NotNil(t, rec.Error)
	require.Equal(t, "boom", *rec.Error)
}

// A failure known to precede dispatch can leave a manual draft; success cannot.
func TestFinalizeLivestreamVODPublishFailureLeavesDraft(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:publishfail"
		finalizerCalls := 0
		state.SetLivestreamVODFinalizer(func(ctx context.Context, task FinalizeLivestreamVODTask) (string, error) {
			finalizerCalls++
			require.NoError(t, state.SetUploadProcessed(ctx, task.UploadID, 5000, "muxlcid-"+task.UploadID, "did:key:signing", `{"durationMs":5000}`, 100))
			return "muxlcid-" + task.UploadID, nil
		})
		publishErr := error(errBoom)
		state.SetVideoPublisher(func(ctx context.Context, task FinalizeLivestreamVODTask) (string, string, error) {
			if publishErr != nil {
				return "", "", publishErr
			}
			out, err := RunTaskEffect(ctx, "video", func(context.Context) ([2]string, error) {
				return [2]string{"at://" + did + "/place.stream.video/v", "bafyvideo"}, nil
			})
			return out[0], out[1], err
		})
		finalize := func(uploadID string) {
			require.NoError(t, state.CreateLivestreamUpload(ctx, uploadID, did, "at://"+did+"/place.stream.livestream/"+uploadID))
			task := enqueueClaimedTask(t, state, TaskFinalizeLivestreamVOD, FinalizeLivestreamVODTask{
				UploadID: uploadID, RepoDID: did,
				Publish: &VideoDraft{Title: "Late night stream", Tags: []string{"chill"}},
			}, WithMaxTries(2))
			if publishErr != nil {
				require.ErrorIs(t, state.runClaimedTask(ctx, task, time.Hour), errBoom)
				requireTaskStatus(t, state, task.ID, TaskStatusPending)
				task = dequeueTestTask(t, state, TaskFinalizeLivestreamVOD)
			}
			require.NoError(t, state.runClaimedTask(ctx, task, time.Hour))
			requireTaskStatus(t, state, task.ID, TaskStatusCompleted)
		}

		finalize("up-unpublished")
		dv, err := state.getDraftByUpload(ctx, "up-unpublished")
		require.NoError(t, err)
		require.NotNil(t, dv, "a draft to publish by hand")
		rec, err := unmarshalDraft(dv.Data)
		require.NoError(t, err)
		require.Equal(t, "ready", rec.Status)
		require.Equal(t, "Late night stream", rec.Title)
		require.Equal(t, []string{"chill"}, rec.Tags)

		publishErr = nil
		finalize("up-published")
		dv, err = state.getDraftByUpload(ctx, "up-published")
		require.NoError(t, err)
		require.Nil(t, dv, "nothing left to publish")
		require.Equal(t, 2, finalizerCalls, "publication retries reuse finalized uploads")
	})
}

func TestAutomaticPublicationResumesReceiptsAndBlocksAmbiguity(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		const did = "did:web:streamer.example"
		const uploadID = "auto-publish-resume"
		require.NoError(t, state.CreateLivestreamUpload(ctx, uploadID, did, "at://"+did+"/place.stream.livestream/live"))
		require.NoError(t, state.SetUploadProcessed(ctx, uploadID, 5000, "muxlcid", "did:key:signing", `{"durationMs":5000}`, 100))
		trackWrites, videoWrites := 0, 0
		preflightFails := true
		state.SetVideoPublisher(func(ctx context.Context, _ FinalizeLivestreamVODTask) (string, string, error) {
			ref, err := RunTaskEffect(ctx, "track-1", func(context.Context) (string, error) {
				trackWrites++
				return "track-reference", nil
			})
			if err != nil {
				return "", "", err
			}
			require.Equal(t, "track-reference", ref)
			if preflightFails {
				return "", "", errBoom
			}
			_, err = RunTaskEffect(ctx, "video", func(context.Context) (string, error) {
				videoWrites++
				return "", errBoom // PDS may have committed the record.
			})
			return "", "", err
		})
		task := enqueueClaimedTask(t, state, TaskFinalizeLivestreamVOD, FinalizeLivestreamVODTask{
			UploadID: uploadID, RepoDID: did, Publish: &VideoDraft{Title: "Automatic"},
		})
		require.ErrorIs(t, state.runClaimedTask(ctx, task, time.Hour), errBoom)
		requireTaskStatus(t, state, task.ID, TaskStatusPending)
		preflightFails = false
		next := dequeueTestTask(t, state, task.Type)
		require.ErrorIs(t, state.runClaimedTask(ctx, next, time.Hour), ErrTaskEffectUncertain)
		requireTaskStatus(t, state, task.ID, TaskStatusBlocked)
		require.Equal(t, 1, trackWrites)
		require.Equal(t, 1, videoWrites)
		retry, err := state.DequeueTask(ctx, "another-worker", task.Type)
		require.NoError(t, err)
		require.Nil(t, retry)
		draft, err := state.getDraftByUpload(ctx, uploadID)
		require.NoError(t, err)
		require.Nil(t, draft, "uncertainty must not manufacture a second publication path")
	})
}

var errBoom = errString("boom")

type errString string

func (e errString) Error() string { return string(e) }

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
