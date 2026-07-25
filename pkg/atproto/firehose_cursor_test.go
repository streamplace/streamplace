package atproto

import (
	"context"
	"testing"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
)

func TestRelayCursorResume(t *testing.T) {
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}
	ctx := context.Background()
	const host = "wss://relay.example"

	// A fresh relay has no stored cursor, so we dial with none and tail live
	// rather than backfilling the relay's whole history.
	rc := atsync.newRelayCursor(ctx, host)
	if _, ok := rc.param(); ok {
		t.Fatal("fresh relay should not send a cursor")
	}

	// observe tracks the high-water mark and never regresses on out-of-order
	// frames (the parallel scheduler can surface them out of sequence).
	rc.observe(100)
	rc.observe(50)
	rc.observe(120)
	if v, ok := rc.param(); !ok || v != 120 {
		t.Fatalf("expected cursor 120, got %d (ok=%v)", v, ok)
	}

	// flush persists the high-water mark to the index DB.
	rc.flush(ctx)
	stored, err := mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(120), stored.Cursor)

	// A second flush with no advance is a no-op, and the stored value is stable.
	rc.observe(120)
	rc.flush(ctx)
	stored, err = mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.Equal(t, int64(120), stored.Cursor)

	// A new cursor (as if the process restarted) resumes from the stored value.
	resumed := atsync.newRelayCursor(ctx, host)
	v, ok := resumed.param()
	require.True(t, ok)
	require.Equal(t, int64(120), v)

	// Cursors are independent per relay.
	other := atsync.newRelayCursor(ctx, "wss://other.example")
	if _, ok := other.param(); ok {
		t.Fatal("a different relay must not inherit another relay's cursor")
	}
}

func TestRelayCursorGroupResume(t *testing.T) {
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}
	ctx := context.Background()
	const host = "moqt://relay.example"

	// A fresh moqt:// relay has no stored group, so we tail the live edge rather
	// than requesting replay.
	rc := atsync.newRelayCursor(ctx, host)
	if _, ok := rc.groupStart(); ok {
		t.Fatal("fresh moq relay should not request replay")
	}

	// observeGroup tracks the high-water group and never regresses on
	// out-of-order delivery.
	rc.observeGroup(500)
	rc.observeGroup(200)
	rc.observeGroup(640)
	g, ok := rc.groupStart()
	require.True(t, ok)
	require.Equal(t, uint64(640), g)

	// flush persists the high-water group as the relay's cursor.
	rc.flush(ctx)
	stored, err := mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(640), stored.Cursor)

	// As if the process restarted: a new cursor resumes replay from the stored
	// group (connectRelayMoq calls SubscribeFrom with it).
	resumed := atsync.newRelayCursor(ctx, host)
	g, ok = resumed.groupStart()
	require.True(t, ok)
	require.Equal(t, uint64(640), g)
}

// Regression: connectRelayMoq shares repoStreamCallbacks with the WebSocket
// path, and those callbacks observe the upstream at-sequence on every event.
// On a moq relay that seq (billions) must stay out of the group cursor
// (thousands): observe() is max-wins, so one poisoned observation would bury
// the group cursor and the next SubscribeFrom would wait forever, silently, on
// a group the relay will never produce.
func TestMoqCallbacksDontPoisonGroupCursor(t *testing.T) {
	atsync := &ATProtoSynchronizer{
		commitDedup:   newFirehoseDeduper(dedupWindow),
		identityDedup: newFirehoseDeduper(dedupWindow),
	}
	commit := &indigoatproto.SyncSubscribeRepos_Commit{
		Seq:    5_000_000_000, // an upstream at-seq, far above any group seq
		Repo:   "did:plc:abc123",
		Commit: lexutil.LexLink(mustCID(t)),
	}
	// Pre-claim the commit in the deduper so the callback stops after its
	// cursor/metrics bookkeeping instead of spawning the indexing handler.
	atsync.commitDedup.seen(commit.Commit.String())

	// moq relay: the group cursor must survive the callback untouched.
	moqCursor := &relayCursor{host: "moqt://relay.example"}
	moqCursor.observeGroup(640)
	rsc := atsync.repoStreamCallbacks(context.Background(), "moqt://relay.example", moqCursor, func() {})
	require.NoError(t, rsc.RepoCommit(commit))
	g, ok := moqCursor.groupStart()
	require.True(t, ok)
	require.Equal(t, uint64(640), g, "upstream at-seq must not bury the group cursor")

	// websocket relay: the same callback is what advances the cursor.
	wsCursor := &relayCursor{host: "wss://relay.example"}
	rsc = atsync.repoStreamCallbacks(context.Background(), "wss://relay.example", wsCursor, func() {})
	require.NoError(t, rsc.RepoCommit(commit))
	v, ok := wsCursor.param()
	require.True(t, ok)
	require.Equal(t, int64(5_000_000_000), v)
}

func TestRelayCursorReset(t *testing.T) {
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}
	ctx := context.Background()
	const host = "moqt://relay.example"

	rc := atsync.newRelayCursor(ctx, host)
	rc.observeGroup(640)
	rc.flush(ctx)

	// A cursor a resume proved untrustworthy is abandoned wholesale: reset
	// drops it below max-wins observe()'s reach, and the next flush heals the
	// stored row too.
	rc.reset()
	if _, ok := rc.groupStart(); ok {
		t.Fatal("reset cursor should tail live")
	}
	rc.flush(ctx)
	stored, err := mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(0), stored.Cursor)

	// A restart after the reset flush also tails live.
	resumed := atsync.newRelayCursor(ctx, host)
	if _, ok := resumed.groupStart(); ok {
		t.Fatal("restarted cursor should tail live after reset")
	}
}
