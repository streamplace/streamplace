package statedb

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

// TestDraftLifecycleThroughVODProcessor is the gating test for the draft
// lifecycle wiring: it proves the queue handler re-reads the Upload row (which
// the real vodProcessor populates via SetUploadProcessed internally) after the
// processor returns just a cid, and writes source/durationMs/content_cid onto
// the draft. A fake processor that returns only a cid must still produce a
// 'ready' draft carrying the upload's source/durationMs.
func TestDraftLifecycleThroughVODProcessor(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:draft"

		// An upload + its processing draft (as createUpload/onComplete would make).
		require.NoError(t, state.CreateUpload(ctx, &Upload{
			ID: "up-lifecycle", RepoDID: did, MimeType: "video/mp4", Backend: "file",
		}))
		_, err := state.CreateDraft(ctx, did, "up-lifecycle", &streamplace.VodDraftVideo{
			LexiconTypeID: "place.stream.vod.draftVideo",
			Title:         "from upload",
			Status:        "processing",
			CreatedAt:     "2026-01-01T00:00:00Z",
		})
		require.NoError(t, err)

		// Fake processor: simulates vod.ProcessVOD's internal SetUploadProcessed
		// (writes the finished fields onto the Upload row) then returns a cid.
		trackURIs := `[{"uri":"at://did:plc:trackhost/place.stream.media.track/t1","cid":"bafytrack1"}]`
		state.SetVODProcessor(func(ctx context.Context, task VODProcessTask) (string, error) {
			require.NoError(t, state.SetUploadProcessed(ctx, task.UploadID, trackURIs, 98765, "muxlcid-lifecycle"))
			return "muxlcid-lifecycle", nil
		})

		// Drive one task through the processor directly. processVODProcessTask
		// also calls CompleteTask at the end, which no-ops on a synthetic task
		// ID with no DB row — that's fine; the draft update happens first.
		task := &AppTask{ID: 1, Type: TaskVODProcess, Payload: mustMarshal(t, VODProcessTask{
			UploadID: "up-lifecycle", RepoDID: did, MimeType: "video/mp4", Backend: "file",
		})}
		_ = state.processVODProcessTask(ctx, task)

		// The draft must now be 'ready' and carry the upload's values.
		dv, err := state.GetDraftByUpload(ctx, "up-lifecycle")
		require.NoError(t, err)
		require.NotNil(t, dv)
		require.Equal(t, "muxlcid-lifecycle", dv.ContentCID, "content_cid must be backfilled from the upload row")
		rec, err := unmarshalDraft(dv.Data)
		require.NoError(t, err)
		require.Equal(t, "ready", rec.Status)
		require.NotNil(t, rec.DurationMs)
		require.Equal(t, int64(98765), *rec.DurationMs)
		require.NotNil(t, rec.Source)
		require.NotNil(t, rec.Source.MediaDefs_SourceTracks)
		require.Len(t, rec.Source.MediaDefs_SourceTracks.Tracks, 1)
		require.Equal(t, "at://did:plc:trackhost/place.stream.media.track/t1", rec.Source.MediaDefs_SourceTracks.Tracks[0].Uri)
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
	_, err = state.CreateDraft(ctx, "did:plc:err", "up-fail", &streamplace.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "will fail", Status: "processing", CreatedAt: "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	state.SetVODProcessor(func(ctx context.Context, task VODProcessTask) (string, error) {
		return "", errBoom
	})

	task := &AppTask{ID: 2, Type: TaskVODProcess, Payload: mustMarshal(t, VODProcessTask{
		UploadID: "up-fail", RepoDID: "did:plc:err", Backend: "file",
	})}
	// processVODProcessTask returns the wrapped error (logged upstream); that's fine.
	_ = state.processVODProcessTask(ctx, task)

	dv, err := state.GetDraftByUpload(ctx, "up-fail")
	require.NoError(t, err)
	rec, err := unmarshalDraft(dv.Data)
	require.NoError(t, err)
	require.Equal(t, "error", rec.Status)
	require.NotNil(t, rec.Error)
	require.Equal(t, "boom", *rec.Error)
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
