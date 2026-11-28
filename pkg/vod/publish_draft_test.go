package vod

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces/spacestest"
	"stream.place/streamplace/pkg/statedb"
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

	drafts, err := state.OpenDrafts(t.Context(), "did:plc:alice", nil)
	require.NoError(t, err)
	defer drafts.Close()
	_, _, err = PublishDraft(t.Context(), state, store, drafts, nil, statedb.DraftURI("did:plc:alice", "none"))
	require.ErrorIs(t, err, statedb.ErrDraftNotFound)
}

func TestPublishDraftNotFoundForOtherUsersDraft(t *testing.T) {
	state := newDraftTestState(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	// Alice owns the draft; Bob tries to publish it.
	alice, err := state.OpenDrafts(ctx, "did:plc:alice", nil)
	require.NoError(t, err)
	dv, err := alice.Create(ctx, "up-1", &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "Alice's draft",
		Status:        "ready",
		CreatedAt:     "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	alice.Close()
	bob, err := state.OpenDrafts(ctx, "did:plc:bob", nil)
	require.NoError(t, err)
	defer bob.Close()
	_, _, err = PublishDraft(ctx, state, store, bob, nil, dv.URI)
	require.ErrorIs(t, err, statedb.ErrDraftNotFound, "a foreign caller must not learn the draft exists")
}

func TestPublishDraftNotReadyWhileProcessing(t *testing.T) {
	state := newDraftTestState(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	drafts, err := state.OpenDrafts(ctx, "did:plc:alice", nil)
	require.NoError(t, err)
	defer drafts.Close()
	dv, err := drafts.Create(ctx, "up-2", &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "Still cooking",
		Status:        "processing",
		CreatedAt:     "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	_, _, err = PublishDraft(ctx, state, store, drafts, nil, dv.URI)
	require.ErrorIs(t, err, ErrDraftNotReady)
}

func TestPublishDraftNotReadyWhenErrored(t *testing.T) {
	state := newDraftTestState(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	drafts, err := state.OpenDrafts(ctx, "did:plc:alice", nil)
	require.NoError(t, err)
	defer drafts.Close()
	dv, err := drafts.Create(ctx, "up-3", &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "Failed",
		Status:        "error",
		CreatedAt:     "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	_, _, err = PublishDraft(ctx, state, store, drafts, nil, dv.URI)
	require.ErrorIs(t, err, ErrDraftNotReady)
}

// draftConnectionsToVideo / draftActivityToVideo are pure mappers; exercise
// them directly since the full PublishDraft path can't run without an XRPC
// client in a unit test.
func TestDraftConnectionAndActivityMapping(t *testing.T) {
	require.Nil(t, draftConnectionsToVideo(nil))

	conn := &placestream.Video_Connection{LexiconTypeID: "place.stream.video#connection"}
	in := []placestream.VodDraftVideo_Connections_Elem{{Video_Connection: conn}, {}}
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

type draftRepoWriter func(input comatproto.RepoPutRecord_Input, output *comatproto.RepoPutRecord_Output) error

func (f draftRepoWriter) Do(_ context.Context, _, _, method string, _ map[string]any, body, out any) error {
	if method != "com.atproto.repo.putRecord" {
		return fmt.Errorf("unexpected public repo RPC %s", method)
	}
	return f(body.(comatproto.RepoPutRecord_Input), out.(*comatproto.RepoPutRecord_Output))
}

type draftDeleteFailure struct {
	spacestest.Fake
	err        error
	afterWrite func()
}

func (c *draftDeleteFailure) Do(ctx context.Context, kind, enc, method string, params map[string]any, body, out any) error {
	if method != "com.atproto.space.applyWrites" {
		return c.Fake.Do(ctx, kind, enc, method, params, body, out)
	}
	if c.err != nil {
		return c.err
	}
	err := c.Fake.Do(ctx, kind, enc, method, params, body, out)
	if err == nil && c.afterWrite != nil {
		c.afterWrite()
	}
	return err
}

func TestPublishPrivateDraftUsesLatestMetadataAndRequiresCleanup(t *testing.T) {
	ctx := t.Context()
	state := newDraftTestState(t)
	did := "did:plc:alice"
	space := &draftDeleteFailure{}
	drafts, err := state.OpenDrafts(ctx, did, space)
	require.NoError(t, err)
	defer drafts.Close()
	draft, err := drafts.Create(ctx, "", &placestream.VodDraftVideo{
		Title: "Initial", Status: "ready", CreatedAt: "2026-01-01T00:00:00Z",
		Source: &placestream.VodDraftVideo_Source{
			MediaDefs_SourceClip: &placestream.MediaDefs_SourceClip{
				Video: "at://" + did + "/place.stream.video/original", Start: 1, End: 2,
			},
		},
	})
	require.NoError(t, err)
	title := "Edited on PDS"
	_, err = drafts.Update(ctx, &placestream.VodUpdateDraft_Input{Uri: draft.URI, Title: &title})
	require.NoError(t, err)
	failed := errors.New("PDS temporarily unavailable")
	publicWrites := 0
	client := draftRepoWriter(func(input comatproto.RepoPutRecord_Input, output *comatproto.RepoPutRecord_Output) error {
		publicWrites++
		video := input.Record.Val.(*placestream.Video)
		require.Equal(t, did, input.Repo)
		require.Equal(t, title, video.Title)
		require.Equal(t, "at://"+did+"/place.stream.video/original", video.Source.MediaDefs_SourceClip.Video)
		output.Uri = "at://" + did + "/place.stream.video/" + input.Rkey
		output.Cid = "public-cid"
		return nil
	})
	space.err = failed
	_, _, err = PublishDraft(ctx, state, nil, drafts, client, draft.URI)
	require.ErrorIs(t, err, failed)
	_, err = drafts.Get(ctx, draft.URI)
	require.NoError(t, err, "the failed cleanup must leave the same draft retriable")
	space.err = nil
	uri, cid, err := PublishDraft(ctx, state, nil, drafts, client, draft.URI)
	require.NoError(t, err)
	rkey, err := draftTID(draft.URI)
	require.NoError(t, err)
	require.Equal(t, "at://"+did+"/place.stream.video/"+rkey, uri)
	require.Equal(t, "public-cid", cid)
	require.Equal(t, 1, publicWrites, "cleanup retry must not rewrite the public record")
	_, err = drafts.Get(ctx, draft.URI)
	require.ErrorIs(t, err, statedb.ErrDraftNotFound)
	require.Empty(t, space.Records)
}

func TestPublishPrivateDraftRetriesAfterPrivateDeletionAndSQLFailure(t *testing.T) {
	ctx := t.Context()
	state := newDraftTestState(t)
	const did = "did:plc:alice"
	space := &draftDeleteFailure{}
	drafts, err := state.OpenDrafts(ctx, did, space)
	require.NoError(t, err)
	row, err := drafts.Create(ctx, "", &placestream.VodDraftVideo{
		Title: "Ready", Status: "ready", CreatedAt: "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)
	rkey, err := draftTID(row.URI)
	require.NoError(t, err)
	videoURI := "at://" + did + "/place.stream.video/" + rkey
	client := draftRepoWriter(func(_ comatproto.RepoPutRecord_Input, out *comatproto.RepoPutRecord_Output) error {
		out.Uri, out.Cid = videoURI, "public-cid"
		return nil
	})

	interrupted, cancel := context.WithCancel(ctx)
	defer cancel()
	space.afterWrite = cancel
	_, _, err = PublishDraft(interrupted, state, nil, drafts, client, row.URI)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, space.Records, "private delete committed before the SQL failure")
	drafts.Close()

	space.afterWrite = nil
	retry, err := state.OpenDrafts(ctx, did, space)
	require.NoError(t, err)
	defer retry.Close()
	uri, cid, err := PublishDraft(ctx, state, nil, retry, nil, row.URI)
	require.NoError(t, err, "cleanup needs no second public repo write")
	require.Equal(t, videoURI, uri)
	require.Equal(t, "public-cid", cid)
	require.ErrorIs(t, retry.Delete(ctx, row.URI), statedb.ErrDraftNotFound, "retry removed the anchor")
}
