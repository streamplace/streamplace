package vod

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/placestream"
)

// newDraftTestState builds an in-memory statedb for draft tests that don't need
// gstreamer or a real XRPC/OAuth session. PublishDraft's early error paths
// (ownership, status) return before any of that is touched.
func newDraftTestState(t *testing.T) *statedb.StatefulDB {
	t.Helper()
	cli := &config.CLI{DBURL: ":memory:"}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(t.Context(), cli, nil, mod)
	require.NoError(t, err)
	return state
}

func TestPublishDraftNotFoundForMissingDraft(t *testing.T) {
	state := newDraftTestState(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	_, _, err = PublishDraft(t.Context(), state, store, "did:plc:alice", "ats://did:plc:alice/place.stream.vod.drafts/self/did:plc:alice/place.stream.vod.draftVideo/none")
	require.ErrorIs(t, err, ErrDraftNotFound)
}

func TestPublishDraftNotFoundForOtherUsersDraft(t *testing.T) {
	state := newDraftTestState(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	// Alice owns the draft; Bob tries to publish it.
	dv, err := state.CreateDraft(ctx, "did:plc:alice", "up-1", &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "Alice's draft",
		Status:        "ready",
		CreatedAt:     "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	_, _, err = PublishDraft(ctx, state, store, "did:plc:bob", dv.URI)
	require.ErrorIs(t, err, ErrDraftNotFound, "a foreign caller must not learn the draft exists")
}

func TestPublishDraftNotReadyWhileProcessing(t *testing.T) {
	state := newDraftTestState(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	dv, err := state.CreateDraft(ctx, "did:plc:alice", "up-2", &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "Still cooking",
		Status:        "processing",
		CreatedAt:     "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	_, _, err = PublishDraft(ctx, state, store, "did:plc:alice", dv.URI)
	require.ErrorIs(t, err, ErrDraftNotReady)
}

func TestPublishDraftNotReadyWhenErrored(t *testing.T) {
	state := newDraftTestState(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	dv, err := state.CreateDraft(ctx, "did:plc:alice", "up-3", &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "Failed",
		Status:        "error",
		CreatedAt:     "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	_, _, err = PublishDraft(ctx, state, store, "did:plc:alice", dv.URI)
	require.ErrorIs(t, err, ErrDraftNotReady)
}

// draftConnectionsToVideo / draftActivityToVideo are pure mappers; exercise
// them directly since the full PublishDraft path can't run without an XRPC
// client in a unit test.
func TestDraftConnectionAndActivityMapping(t *testing.T) {
	require.Nil(t, draftConnectionsToVideo(nil))

	conn := &placestream.Video_Connection{LexiconTypeID: "place.stream.video#connection"}
	in := []*placestream.VodDraftVideo_Connections_Elem{{Video_Connection: conn}, nil}
	out := draftConnectionsToVideo(in)
	require.Len(t, out, 1)
	require.Equal(t, conn, out[0].Video_Connection)

	require.Nil(t, draftActivityToVideo(nil))
	game := &placestream.Defs_ActivityGame{LexiconTypeID: "place.stream.defs#activityGame"}
	act := draftActivityToVideo(&placestream.VodDraftVideo_Activity{Defs_ActivityGame: game})
	require.NotNil(t, act)
	require.Equal(t, game, act.Defs_ActivityGame)
}

func TestDraftTID(t *testing.T) {
	did := "did:web:example.com"
	uri := "ats://" + did + "/place.stream.vod.drafts/self/" + did + "/place.stream.vod.draftVideo/3mpa4yiq4bncy"
	tid, err := draftTID(uri)
	require.NoError(t, err)
	require.Equal(t, "3mpa4yiq4bncy", tid)

	// No rkey segment.
	_, err = draftTID("ats://" + did + "/place.stream.vod.drafts/self/" + did + "/place.stream.vod.draftVideo/")
	require.Error(t, err)
	// No slash at all.
	_, err = draftTID("garbage")
	require.Error(t, err)
}
