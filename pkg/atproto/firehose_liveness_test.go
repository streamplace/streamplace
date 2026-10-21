package atproto

import (
	"context"
	"testing"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/stretchr/testify/require"
)

// TestFirehoseLivenessCountsUnindexedCommits pins the meaning of the node-wide
// "behind real time" mark: it follows the stream, not the index.
//
// A node subscribed to a relay whose traffic is mostly collections we skip used
// to leave the mark untouched for as long as no indexed op arrived, and so
// warned "firehose is Nm behind real time" while it was perfectly caught up.
// The fixture is a commit whose only op is a like -- nothing indexed -- so only
// observeTime can move the mark.
func TestFirehoseLivenessCountsUnindexedCommits(t *testing.T) {
	ctx := context.Background()
	atsync, _ := contiguityTestSync(t)
	atsync.commitDedup = newFirehoseDeduper(dedupWindow)
	atsync.identityDedup = newFirehoseDeduper(dedupWindow)

	evtTime := time.Date(2026, 10, 21, 22, 0, 0, 0, time.UTC)
	commit := func(at time.Time) *indigoatproto.SyncSubscribeRepos_Commit {
		return &indigoatproto.SyncSubscribeRepos_Commit{
			Seq:    1,
			Repo:   "did:plc:quiet",
			Time:   at.Format(time.RFC3339Nano),
			Commit: lexutil.LexLink(mustCID(t)),
			Ops: []*indigoatproto.SyncSubscribeRepos_RepoOp{
				repoOp("create", "app.bsky.feed.like/3lpabc"),
			},
		}
	}

	cursor := &relayCursor{host: "wss://relay.example"}
	rsc := atsync.repoStreamCallbacks(ctx, "wss://relay.example", cursor, func() {})

	require.NoError(t, rsc.RepoCommit(commit(evtTime)))
	require.Equal(t, evtTime.UnixNano(), atsync.lastEvent.get(),
		"a commit we index nothing from still proves how current the relay is")

	// A relay replaying older history must not drag the mark backwards. This
	// one is also a duplicate (same commit CID), and the mark has to move
	// before dedup: a duplicate is still evidence of what the relay sent.
	require.NoError(t, rsc.RepoCommit(commit(evtTime.Add(-time.Minute))))
	require.Equal(t, evtTime.UnixNano(), atsync.lastEvent.get())

	// A relay with a clock running ahead must not pin the mark into the
	// future: that would silence lag reporting until the bogus stamp caught
	// up. The observation folds to the receive time, so a later, correctly
	// dated event still advances it.
	require.NoError(t, rsc.RepoCommit(commit(time.Now().Add(time.Hour))))
	require.LessOrEqual(t, atsync.lastEvent.get(), time.Now().UnixNano(),
		"a future stamp must not push the position past the wall clock")
	before := atsync.lastEvent.get()
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, rsc.RepoCommit(commit(time.Now())))
	require.Greater(t, atsync.lastEvent.get(), before,
		"correctly dated events keep advancing after a bogus future one")
	require.Less(t, sinceNanos(atsync.lastEvent.get()), time.Minute)
}
