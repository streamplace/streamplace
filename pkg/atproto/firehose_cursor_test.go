package atproto

import (
	"context"
	"testing"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
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

// TestRelayCursorEventTime covers the second half of the cursor: the newest
// event time is what makes the cursor's age knowable, so it has to high-water,
// persist and reload alongside the sequence.
func TestRelayCursorEventTime(t *testing.T) {
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}
	ctx := context.Background()
	const host = "wss://relay.example"

	rc := atsync.newRelayCursor(ctx, host)
	require.Equal(t, int64(0), rc.lastEventTime(), "a fresh relay has seen no events")

	// Max-wins, like the seq: relays stamp events from several workers, so a
	// slightly older stamp arriving late must not walk the mark backwards.
	base := time.Unix(1700000000, 0)
	rc.observeTime(base)
	rc.observeTime(base.Add(-30 * time.Second))
	rc.observeTime(base.Add(90 * time.Second))
	require.Equal(t, base.Add(90*time.Second).Unix(), rc.lastEventTime())

	// flush writes both columns, and a flush with nothing new is a no-op.
	rc.observe(120)
	rc.flush(ctx)
	stored, err := mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(120), stored.Cursor)
	require.Equal(t, base.Add(90*time.Second).Unix(), stored.LastEventTime)

	// The event time advancing on its own is enough to trigger a write -- a moq
	// relay whose group cursor is unchanged still gets its freshness recorded.
	rc.observeTime(base.Add(5 * time.Minute))
	rc.flush(ctx)
	stored, err = mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.Equal(t, int64(120), stored.Cursor)
	require.Equal(t, base.Add(5*time.Minute).Unix(), stored.LastEventTime)

	// A restart reloads both, so the staleness check has something to judge.
	resumed := atsync.newRelayCursor(ctx, host)
	v, ok := resumed.param()
	require.True(t, ok)
	require.Equal(t, int64(120), v)
	require.Equal(t, base.Add(5*time.Minute).Unix(), resumed.lastEventTime())

	// reset drops the event time with the cursor: a fresh timestamp next to a
	// zero cursor would claim we are current with a relay we just abandoned.
	resumed.reset()
	require.Equal(t, int64(0), resumed.lastEventTime())
	_, ok = resumed.param()
	require.False(t, ok)
}

// TestRelayCursorDropIfStale is the production incident in a unit test: a
// cursor whose newest event is hours old must not be replayed from, because
// asking a relay to re-send hours of the whole network is the thing that buried
// the node. Fresh cursors are left alone, since replay is genuinely cheaper
// across an ordinary restart.
func TestRelayCursorDropIfStale(t *testing.T) {
	ctx := context.Background()
	const window = 15 * time.Minute

	newCursor := func(seq int64, lastEvent time.Time) *relayCursor {
		rc := &relayCursor{host: "wss://relay.example"}
		rc.latest.set(seq)
		if !lastEvent.IsZero() {
			rc.observeTime(lastEvent)
		}
		return rc
	}

	// Inside the window: keep the cursor and replay from it.
	rc := newCursor(120, time.Now().Add(-time.Minute))
	require.False(t, rc.dropIfStale(ctx, window))
	v, ok := rc.param()
	require.True(t, ok)
	require.Equal(t, int64(120), v)

	// Older than the window: drop it and tail live, leaving the gap for the
	// sweep's head check.
	rc = newCursor(120, time.Now().Add(-2*time.Hour))
	require.True(t, rc.dropIfStale(ctx, window))
	_, ok = rc.param()
	require.False(t, ok, "a dropped cursor must not be sent to the relay")
	require.Equal(t, int64(0), rc.lastEventTime())

	// A nonzero cursor with NO event time is a row written before the column
	// existed -- exactly the state production was poisoned with. Unknown age
	// means unbounded replay, so it counts as stale.
	rc = newCursor(120, time.Time{})
	require.Equal(t, int64(0), rc.lastEventTime())
	require.True(t, rc.dropIfStale(ctx, window))
	_, ok = rc.param()
	require.False(t, ok)

	// A zero cursor is already tailing live; there is nothing to drop, so
	// nothing is reported (and no sweep gets kicked).
	rc = newCursor(0, time.Time{})
	require.False(t, rc.dropIfStale(ctx, window))

	// Window 0 disables the cap entirely: however old, replay from it.
	rc = newCursor(120, time.Now().Add(-72*time.Hour))
	require.False(t, rc.dropIfStale(ctx, 0))
	v, ok = rc.param()
	require.True(t, ok)
	require.Equal(t, int64(120), v)

	// The moq flavour of the same cursor is dropped on the same terms, so
	// SubscribeFrom asks for the live edge rather than a two-hour replay.
	rc = &relayCursor{host: "moqt://relay.example"}
	rc.observeGroup(640)
	rc.observeTime(time.Now().Add(-2 * time.Hour))
	require.True(t, rc.dropIfStale(ctx, window))
	_, ok = rc.groupStart()
	require.False(t, ok)
}

// TestFirehoseReplayWindow pins the flag plumbing: unset CLI means the default,
// and an explicit zero means "never cap", which is how the escape hatch is
// spelled everywhere else in the sweep config.
func TestFirehoseReplayWindow(t *testing.T) {
	require.Equal(t, config.DefaultFirehoseReplayWindow,
		(&ATProtoSynchronizer{}).firehoseReplayWindow())
	require.Equal(t, 30*time.Second,
		(&ATProtoSynchronizer{CLI: &config.CLI{FirehoseReplayWindow: 30 * time.Second}}).firehoseReplayWindow())
	require.Equal(t, time.Duration(0),
		(&ATProtoSynchronizer{CLI: &config.CLI{}}).firehoseReplayWindow())
}

// TestRelayCursorObserveTimeFromCallbacks checks the wiring the staleness cap
// depends on: event times are recorded for both commit and identity events, on
// every transport (unlike the seq, which stays out of a moq relay's group
// cursor), and BEFORE dedup -- a duplicate is still evidence of how current
// this relay is.
func TestRelayCursorObserveTimeFromCallbacks(t *testing.T) {
	atsync := &ATProtoSynchronizer{
		commitDedup:   newFirehoseDeduper(dedupWindow),
		identityDedup: newFirehoseDeduper(dedupWindow),
	}
	evtTime := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	commit := &indigoatproto.SyncSubscribeRepos_Commit{
		Seq:    5_000_000_000,
		Repo:   "did:plc:abc123",
		Time:   evtTime,
		Commit: lexutil.LexLink(mustCID(t)),
	}
	// Pre-claim it so the callback stops after its bookkeeping instead of
	// running the indexing handler, which would want a whole model.
	atsync.commitDedup.seen(commit.Commit.String())

	want, err := time.Parse(time.RFC3339Nano, evtTime)
	require.NoError(t, err)

	moqCursor := &relayCursor{host: "moqt://relay.example"}
	rsc := atsync.repoStreamCallbacks(context.Background(), "moqt://relay.example", moqCursor, func() {})
	require.NoError(t, rsc.RepoCommit(commit))
	require.Equal(t, want.Unix(), moqCursor.lastEventTime(),
		"event time is transport-independent, unlike the seq")

	identity := &indigoatproto.SyncSubscribeRepos_Identity{
		Seq:  5_000_000_001,
		Did:  "did:plc:abc123",
		Time: evtTime,
	}
	atsync.identityDedup.seen(identityDedupKey(identity))
	wsCursor := &relayCursor{host: "wss://relay.example"}
	rsc = atsync.repoStreamCallbacks(context.Background(), "wss://relay.example", wsCursor, func() {})
	require.NoError(t, rsc.RepoIdentity(identity))
	require.Equal(t, want.Unix(), wsCursor.lastEventTime())

	// An unparsable stamp is simply not an observation, never a crash.
	wsCursor2 := &relayCursor{host: "wss://relay.example"}
	rsc = atsync.repoStreamCallbacks(context.Background(), "wss://relay.example", wsCursor2, func() {})
	require.NoError(t, rsc.RepoCommit(&indigoatproto.SyncSubscribeRepos_Commit{
		Seq:    7,
		Repo:   "did:plc:abc123",
		Time:   "not a timestamp",
		Commit: lexutil.LexLink(mustCID(t)),
	}))
	require.Equal(t, int64(0), wsCursor2.lastEventTime())
}
