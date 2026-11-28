package statedb

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func newDraftRec(title, status string) *placestream.VodDraftVideo {
	return &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         title,
		Status:        status,
		CreatedAt:     "2026-01-01T00:00:00Z",
	}
}

func TestDraftVideoCRUDRoundTrip(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		did := "did:plc:alice"

		drafts, err := state.OpenDrafts(ctx, did, nil)
		require.NoError(t, err)
		defer drafts.Close()
		dv, err := drafts.Create(ctx, "upload-123", newDraftRec("My Draft", "processing"))
		require.NoError(t, err)
		require.NotEmpty(t, dv.URI)
		require.Equal(t, did, dv.UserDID)
		require.NotEmpty(t, dv.CID)
		require.Equal(t, "upload-123", dv.OriginUploadID)
		// URI matches the personal-space shape.
		require.Contains(t, dv.URI, "ats://"+did+"/place.stream.vod.drafts/self/"+did+"/place.stream.vod.draftVideo/")

		// Get
		got, err := drafts.Get(ctx, dv.URI)
		require.NoError(t, err)
		require.Equal(t, dv.URI, got.URI)
		require.Equal(t, dv.CID, got.CID)
		require.Equal(t, dv.Data, got.Data)

		// CBOR round-trips faithfully.
		gotRec, err := unmarshalDraft(got.Data)
		require.NoError(t, err)
		require.Equal(t, "My Draft", gotRec.Title)
		require.Equal(t, "processing", gotRec.Status)

		// GetDraftByUpload
		byUp, err := state.getDraftByUpload(ctx, "upload-123")
		require.NoError(t, err)
		require.Equal(t, dv.URI, byUp.URI)

		// Delete
		require.NoError(t, drafts.Delete(ctx, dv.URI))
		_, err = drafts.Get(ctx, dv.URI)
		require.ErrorIs(t, err, ErrDraftNotFound)
	})
}

func TestDraftVideoUpdateMetadataRecomputesCID(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		did := "did:plc:bob"

		drafts, err := state.OpenDrafts(ctx, did, nil)
		require.NoError(t, err)
		defer drafts.Close()
		dv, err := drafts.Create(ctx, "upload-456", newDraftRec("Original", "processing"))
		require.NoError(t, err)
		origCID := dv.CID

		// Partial update of an editable field.
		title := "Edited Title"
		updated, err := drafts.Update(ctx, &placestream.VodUpdateDraft_Input{Uri: dv.URI, Title: &title})
		require.NoError(t, err)
		require.NotNil(t, updated)

		// CID changed because the CBOR body changed.
		require.NotEqual(t, origCID, updated.CID)

		// The status (server-authoritative) is preserved.
		rec, err := unmarshalDraft(updated.Data)
		require.NoError(t, err)
		require.Equal(t, "Edited Title", rec.Title)
		require.Equal(t, "processing", rec.Status, "status must be preserved across metadata update")
	})
}

func TestDraftVideoListOwnershipScoped(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		alice := "did:plc:alice"
		bob := "did:plc:bob"

		for i := 0; i < 3; i++ {
			_, err := state.insertDraft(ctx, alice, "up-alice-"+strconv.Itoa(i), "", "", newDraftRec("A"+strconv.Itoa(i), "processing"))
			require.NoError(t, err)
		}
		_, err := state.insertDraft(ctx, bob, "up-bob-0", "", "", newDraftRec("B0", "processing"))
		require.NoError(t, err)

		// Alice sees only her 3 drafts.
		aliceStore, err := state.OpenDrafts(ctx, alice, nil)
		require.NoError(t, err)
		defer aliceStore.Close()
		aliceDrafts, _, err := aliceStore.List(ctx, 100, "")
		require.NoError(t, err)
		require.Len(t, aliceDrafts, 3)
		for _, d := range aliceDrafts {
			require.Equal(t, alice, d.UserDID)
		}

		// Bob sees only his 1.
		bobStore, err := state.OpenDrafts(ctx, bob, nil)
		require.NoError(t, err)
		defer bobStore.Close()
		bobDrafts, _, err := bobStore.List(ctx, 100, "")
		require.NoError(t, err)
		require.Len(t, bobDrafts, 1)
	})
}

func TestSetDraftReadyAndError(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		did := "did:plc:carol"

		// ready path
		dv, err := state.insertDraft(ctx, did, "up-ready", "", "", newDraftRec("Ready me", "processing"))
		require.NoError(t, err)

		require.NoError(t, state.CreateUpload(ctx, &Upload{
			ID: "up-ready", RepoDID: did, TrackURIs: "[]", DurationMS: 12345,
			ContentCID: "muxlcid123", ProcessingStatus: "done",
		}))
		err = state.syncDraftResult(ctx, "up-ready")
		require.NoError(t, err)

		ready, err := state.getDraft(ctx, did, dv.URI)
		require.NoError(t, err)
		require.Equal(t, "muxlcid123", ready.ContentCID)
		rec, err := unmarshalDraft(ready.Data)
		require.NoError(t, err)
		require.Equal(t, "ready", rec.Status)
		require.NotNil(t, rec.DurationMs)
		require.Equal(t, int64(12345), *rec.DurationMs)
		require.NotNil(t, rec.Source)
		require.NotNil(t, rec.Source.MediaDefs_SourceTracks)

		// error path
		dv2, err := state.insertDraft(ctx, did, "up-fail", "", "", newDraftRec("Fail me", "processing"))
		require.NoError(t, err)
		require.NoError(t, state.CreateUpload(ctx, &Upload{
			ID: "up-fail", RepoDID: did, ProcessingStatus: "error", ProcessingError: "codec not supported",
		}))
		err = state.syncDraftResult(ctx, "up-fail")
		require.NoError(t, err)
		failed, err := state.getDraft(ctx, did, dv2.URI)
		require.NoError(t, err)
		rec2, err := unmarshalDraft(failed.Data)
		require.NoError(t, err)
		require.Equal(t, "error", rec2.Status)
		require.NotNil(t, rec2.Error)
		require.Equal(t, "codec not supported", *rec2.Error)
	})
}

func TestSetDraftOnMissingUploadIsNoOp(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		// A pre-drafts-era upload (no draft row) must not error.
		require.NoError(t, state.syncDraftResult(t.Context(), "no-such-upload"))
	})
}
