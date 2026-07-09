package statedb

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func newDraftRec(title, status string) *streamplace.VodDraftVideo {
	return &streamplace.VodDraftVideo{
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

		// Create
		rec := newDraftRec("My Draft", "processing")
		dv, err := state.CreateDraft(ctx, did, "upload-123", rec)
		require.NoError(t, err)
		require.NotEmpty(t, dv.URI)
		require.Equal(t, did, dv.UserDID)
		require.NotEmpty(t, dv.CID)
		require.Equal(t, "upload-123", dv.OriginUploadID)
		// URI matches the personal-space shape.
		require.Contains(t, dv.URI, "ats://"+did+"/place.stream.vod.drafts/self/"+did+"/place.stream.vod.draftVideo/")

		// Get
		got, err := state.GetDraft(ctx, dv.URI)
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
		byUp, err := state.GetDraftByUpload(ctx, "upload-123")
		require.NoError(t, err)
		require.Equal(t, dv.URI, byUp.URI)

		// Delete
		deleted, err := state.DeleteDraft(ctx, dv.URI)
		require.NoError(t, err)
		require.True(t, deleted)
		gone, err := state.GetDraft(ctx, dv.URI)
		require.NoError(t, err)
		require.Nil(t, gone)
	})
}

func TestDraftVideoUpdateMetadataRecomputesCID(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		did := "did:plc:bob"

		dv, err := state.CreateDraft(ctx, did, "upload-456", newDraftRec("Original", "processing"))
		require.NoError(t, err)
		origCID := dv.CID

		// Partial update of an editable field.
		updated, err := state.UpdateDraftMetadata(ctx, dv.URI, func(rec *streamplace.VodDraftVideo) {
			rec.Title = "Edited Title"
		})
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
			_, err := state.CreateDraft(ctx, alice, "up-alice-"+strconv.Itoa(i), newDraftRec("A"+strconv.Itoa(i), "processing"))
			require.NoError(t, err)
		}
		_, err := state.CreateDraft(ctx, bob, "up-bob-0", newDraftRec("B0", "processing"))
		require.NoError(t, err)

		// Alice sees only her 3 drafts.
		aliceDrafts, err := state.ListDrafts(ctx, alice, 100, "")
		require.NoError(t, err)
		require.Len(t, aliceDrafts, 3)
		for _, d := range aliceDrafts {
			require.Equal(t, alice, d.UserDID)
		}

		// Bob sees only his 1.
		bobDrafts, err := state.ListDrafts(ctx, bob, 100, "")
		require.NoError(t, err)
		require.Len(t, bobDrafts, 1)
	})
}

func TestSetDraftReadyAndError(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		did := "did:plc:carol"

		// ready path
		dv, err := state.CreateDraft(ctx, did, "up-ready", newDraftRec("Ready me", "processing"))
		require.NoError(t, err)

		sourceTracks := &streamplace.VodDraftVideo_Source{
			MediaDefs_SourceTracks: &streamplace.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        nil,
			},
		}
		err = state.SetDraftReady(ctx, "up-ready", sourceTracks, 12345, "muxlcid123")
		require.NoError(t, err)

		ready, err := state.GetDraft(ctx, dv.URI)
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
		dv2, err := state.CreateDraft(ctx, did, "up-fail", newDraftRec("Fail me", "processing"))
		require.NoError(t, err)
		err = state.SetDraftError(ctx, "up-fail", "codec not supported")
		require.NoError(t, err)
		failed, err := state.GetDraft(ctx, dv2.URI)
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
		err := state.SetDraftReady(t.Context(), "no-such-upload", nil, 0, "")
		require.NoError(t, err)
		err = state.SetDraftError(t.Context(), "no-such-upload", "x")
		require.NoError(t, err)
	})
}
