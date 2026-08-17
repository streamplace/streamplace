package atproto

import (
	"context"
	"testing"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/constants"
)

func repoOp(action, path string) *indigoatproto.SyncSubscribeRepos_RepoOp {
	return &indigoatproto.SyncSubscribeRepos_RepoOp{Action: action, Path: path}
}

// TestIndexedCollection pins the keep/skip decision that now runs before the
// CAR parse. It has to answer exactly what the per-op filter answered, because
// it is the same function -- a divergence here would silently stop indexing a
// collection.
func TestIndexedCollection(t *testing.T) {
	require.True(t, indexedCollection(constants.APP_BSKY_FEED_POST), "a member of CollectionFilter")
	require.True(t, indexedCollection(constants.APP_BSKY_GRAPH_FOLLOW))
	require.True(t, indexedCollection(constants.PLACE_STREAM_LIVE_RECOMMENDATIONS))

	// Everything under our own namespace is ours, listed or not: the filter
	// names only the app.bsky.* collections it wants.
	require.True(t, indexedCollection(constants.PLACE_STREAM_CHAT_MESSAGE))
	require.True(t, indexedCollection(constants.PLACE_STREAM_VIDEO))
	require.True(t, indexedCollection("place.stream.something.invented.tomorrow"))

	// The majority of the firehose, which we index nothing from.
	require.False(t, indexedCollection("app.bsky.feed.like"))
	require.False(t, indexedCollection("app.bsky.feed.repost"))
	require.False(t, indexedCollection("com.whtwnd.blog.entry"))
	require.False(t, indexedCollection("placestream.notours"))
	require.False(t, indexedCollection(""))
}

// TestCommitHasIndexedOps is the hoist itself: from op paths alone, without
// touching the CAR, does this commit contain anything for us?
func TestCommitHasIndexedOps(t *testing.T) {
	boring := &indigoatproto.SyncSubscribeRepos_Commit{
		Ops: []*indigoatproto.SyncSubscribeRepos_RepoOp{
			repoOp("create", "app.bsky.feed.like/3lpabc"),
			repoOp("delete", "app.bsky.feed.repost/3lpdef"),
		},
	}
	require.False(t, commitHasIndexedOps(boring))

	// One interesting op among many boring ones is enough -- the CAR has to be
	// read for the commit as a whole.
	mixed := &indigoatproto.SyncSubscribeRepos_Commit{
		Ops: []*indigoatproto.SyncSubscribeRepos_RepoOp{
			repoOp("create", "app.bsky.feed.like/3lpabc"),
			repoOp("create", constants.PLACE_STREAM_CHAT_MESSAGE+"/3lpghi"),
		},
	}
	require.True(t, commitHasIndexedOps(mixed))

	// A commit with no ops at all has nothing for us.
	require.False(t, commitHasIndexedOps(&indigoatproto.SyncSubscribeRepos_Commit{}))

	// An unparsable path counts as interesting, so the main loop reaches it and
	// logs the error it always logged rather than dropping it in silence.
	require.True(t, commitHasIndexedOps(&indigoatproto.SyncSubscribeRepos_Commit{
		Ops: []*indigoatproto.SyncSubscribeRepos_RepoOp{repoOp("create", "nopath")},
	}))
}

// TestHandleCommitEventOpsTracksFilteredCommits is the subtle half of the
// hoist, and the reason the early return sits where it does.
//
// A repo we track writes plenty of records we index nothing from. Those commits
// still carry the rev chain, so trackCommitRev has to see them -- otherwise our
// stored rev falls behind, and the next commit we DO care about arrives with a
// Since we have never heard of, looks like a firehose gap, and orders a repair
// of a repo that was never damaged. trackCommitRev needs no CAR, so it runs on
// the far side of the skip.
//
// The empty Blocks here is the load-bearing part of the fixture: an empty CAR
// cannot be parsed, so a test that passes proves the parse was skipped.
func TestHandleCommitEventOpsTracksFilteredCommits(t *testing.T) {
	ctx := context.Background()
	atsync, mod := contiguityTestSync(t)
	require.NoError(t, mod.UpdateRepo(syncedRepo("did:plc:filtered", "3lprev0000000")))

	evt := commitEvent("did:plc:filtered", "3lprev0000000", "3lprev0000001")
	evt.Ops = []*indigoatproto.SyncSubscribeRepos_RepoOp{
		repoOp("create", "app.bsky.feed.like/3lpabc"),
	}
	atsync.handleCommitEventOps(ctx, evt)

	got, err := mod.GetRepo("did:plc:filtered")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "3lprev0000001", got.Version,
		"a commit whose ops are all filtered still advances the rev chain")
	require.Empty(t, got.RepairFrom, "and orders no repair")

	// A commit that DOES carry something we index cannot skip the CAR, and an
	// unreadable one has to leave the rev where it was -- half-applied is the
	// one state the chain must never record as clean.
	evt = commitEvent("did:plc:filtered", "3lprev0000001", "3lprev0000002")
	evt.Ops = []*indigoatproto.SyncSubscribeRepos_RepoOp{
		repoOp("create", constants.PLACE_STREAM_CHAT_MESSAGE+"/3lpghi"),
	}
	atsync.handleCommitEventOps(ctx, evt)

	got, err = mod.GetRepo("did:plc:filtered")
	require.NoError(t, err)
	require.Equal(t, "3lprev0000001", got.Version,
		"an event that bailed out before indexing its ops must not claim the commit")

	// tooBig is unchanged: skipped entirely, rev included.
	evt = commitEvent("did:plc:filtered", "3lprev0000001", "3lprev0000003")
	evt.TooBig = true
	atsync.handleCommitEventOps(ctx, evt)
	got, err = mod.GetRepo("did:plc:filtered")
	require.NoError(t, err)
	require.Equal(t, "3lprev0000001", got.Version)
}
