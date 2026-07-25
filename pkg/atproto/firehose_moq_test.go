package atproto

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	"github.com/ipfs/go-cid"
	atmoq "github.com/streamplace/atmoq/go"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/model"
)

// a real CIDv1 to populate the required commit/identity link fields so CBOR
// marshalling produces a frame indistinguishable from a live one.
const testCommitCID = "bafyreidfcyqcar7vbkjs3umpv6up2dgun5amkhcrxnsmsfqxg2znj4dqvm"

// collectScheduler returns a parallel scheduler that forwards every dispatched
// event onto the returned channel, so a test can assert what the decode routed.
func collectScheduler(t *testing.T) (*parallel.Scheduler, <-chan *events.XRPCStreamEvent) {
	t.Helper()
	out := make(chan *events.XRPCStreamEvent, 16)
	sched := parallel.NewScheduler(1, 16, "moqtest", func(ctx context.Context, ev *events.XRPCStreamEvent) error {
		out <- ev
		return nil
	})
	t.Cleanup(sched.Shutdown)
	return sched, out
}

func mustCID(t *testing.T) cid.Cid {
	t.Helper()
	c, err := cid.Decode(testCommitCID)
	require.NoError(t, err)
	return c
}

func TestDispatchMoqFrameCommit(t *testing.T) {
	var buf bytes.Buffer
	hdr := events.EventHeader{Op: events.EvtKindMessage, MsgType: "#commit"}
	require.NoError(t, hdr.MarshalCBOR(&buf))
	commit := &comatproto.SyncSubscribeRepos_Commit{
		Seq:    42,
		Repo:   "did:plc:abc123",
		Rev:    "rev1",
		Commit: glex.Link(mustCID(t)),
		Time:   "2026-06-25T00:00:00Z",
		Blocks: glex.Bytes{},
		Ops:    []comatproto.SyncSubscribeRepos_RepoOp{},
		Blobs:  []glex.Link{},
	}
	require.NoError(t, commit.MarshalCBOR(&buf))

	sched, out := collectScheduler(t)
	atsync := &ATProtoSynchronizer{}
	require.NoError(t, atsync.dispatchMoqFrame(context.Background(), buf.Bytes(), 1, "moqt://test", "moq", sched))

	select {
	case ev := <-out:
		require.NotNil(t, ev.RepoCommit)
		require.Equal(t, int64(42), ev.RepoCommit.Seq)
		require.Equal(t, "did:plc:abc123", ev.RepoCommit.Repo)
	case <-time.After(2 * time.Second):
		t.Fatal("commit frame was not dispatched")
	}
}

func TestDispatchMoqFrameIdentity(t *testing.T) {
	var buf bytes.Buffer
	hdr := events.EventHeader{Op: events.EvtKindMessage, MsgType: "#identity"}
	require.NoError(t, hdr.MarshalCBOR(&buf))
	handle := "alice.test"
	identity := &comatproto.SyncSubscribeRepos_Identity{
		Seq:    7,
		Did:    "did:plc:abc123",
		Handle: &handle,
		Time:   "2026-06-25T00:00:00Z",
	}
	require.NoError(t, identity.MarshalCBOR(&buf))

	sched, out := collectScheduler(t)
	atsync := &ATProtoSynchronizer{}
	require.NoError(t, atsync.dispatchMoqFrame(context.Background(), buf.Bytes(), 1, "moqt://test", "moq", sched))

	select {
	case ev := <-out:
		require.NotNil(t, ev.RepoIdentity)
		require.Equal(t, "did:plc:abc123", ev.RepoIdentity.Did)
	case <-time.After(2 * time.Second):
		t.Fatal("identity frame was not dispatched")
	}
}

// An event type we don't index (#account) must decode-but-drop, exactly as it
// does over WebSocket where no callback is registered for it.
func TestDispatchMoqFrameIgnoredType(t *testing.T) {
	var buf bytes.Buffer
	hdr := events.EventHeader{Op: events.EvtKindMessage, MsgType: "#account"}
	require.NoError(t, hdr.MarshalCBOR(&buf))
	account := &comatproto.SyncSubscribeRepos_Account{
		Seq:    1,
		Did:    "did:plc:abc123",
		Active: true,
		Time:   "2026-06-25T00:00:00Z",
	}
	require.NoError(t, account.MarshalCBOR(&buf))

	sched, out := collectScheduler(t)
	atsync := &ATProtoSynchronizer{}
	require.NoError(t, atsync.dispatchMoqFrame(context.Background(), buf.Bytes(), 1, "moqt://test", "moq", sched))

	select {
	case ev := <-out:
		t.Fatalf("unindexed frame should not dispatch, got %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// expected: nothing dispatched
	}
}

// TestFirehoseMoqLive exercises the full decode path against a real MoQ relay.
// Network-gated: set SP_MOQ_LIVE_TEST=moqt://host to run it.
func TestFirehoseMoqLive(t *testing.T) {
	relay := os.Getenv("SP_MOQ_LIVE_TEST")
	if relay == "" {
		t.Skip("set SP_MOQ_LIVE_TEST=moqt://host to run the live moq relay test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sess, err := atmoq.Dial(ctx, relay, &atmoq.Options{})
	require.NoError(t, err)
	defer sess.Close()
	sub, err := sess.Subscribe(ctx, atmoq.DefaultBroadcast, atmoq.DefaultTrack)
	require.NoError(t, err)
	defer sub.Close()

	sched, out := collectScheduler(t)
	atsync := &ATProtoSynchronizer{}

	commits := 0
	for commits < 5 {
		raw, _, err := sub.ReadFrame(ctx)
		require.NoError(t, err, "reading %d live frames", commits)
		require.NoError(t, atsync.dispatchMoqFrame(ctx, raw, 1, "moqt://test", "moq", sched))
		select {
		case ev := <-out:
			if ev.RepoCommit != nil {
				require.NotEmpty(t, ev.RepoCommit.Repo)
				commits++
			}
		case <-time.After(time.Second):
		}
	}
	t.Logf("decoded %d live commit frames from %s", commits, relay)
}

// TestRelayCursorPersistResumeLive exercises the whole cross-restart resume path
// against a real windowed atmoq relay: tail live, persist the group cursor to a
// file-backed DB, simulate a Streamplace restart by reopening the DB with a new
// model, and confirm the resumed cursor replays from the persisted group while a
// fresh live subscribe starts past it. Network-gated: set
// SP_MOQ_REPLAY_TEST=moqt://host to a relay run with --replay-window-secs.
func TestRelayCursorPersistResumeLive(t *testing.T) {
	relay := os.Getenv("SP_MOQ_REPLAY_TEST")
	if relay == "" {
		t.Skip("set SP_MOQ_REPLAY_TEST=moqt://host (a windowed atmoq relay) to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	dbPath := t.TempDir() + "/cursor.db"
	mod, err := model.MakeDB(dbPath)
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}

	// Phase 1: fresh cursor tails live; record + persist the high-water group.
	rc := atsync.newRelayCursor(ctx, relay)
	_, ok := rc.groupStart()
	require.False(t, ok, "fresh cursor should tail the live edge")

	sess, err := atmoq.Dial(ctx, relay, &atmoq.Options{})
	require.NoError(t, err)
	sub, err := sess.Subscribe(ctx, atmoq.DefaultBroadcast, atmoq.DefaultTrack)
	require.NoError(t, err)
	for i := 0; i < 80; i++ {
		_, g, err := sub.ReadFrame(ctx)
		require.NoError(t, err)
		rc.observeGroup(g)
	}
	persisted, ok := rc.groupStart()
	require.True(t, ok)
	sub.Close()
	sess.Close()
	rc.flush(ctx) // write the cursor to the DB
	t.Logf("phase1: persisted group cursor G=%d", persisted)

	// Let the live edge advance past the persisted group.
	time.Sleep(5 * time.Second)

	// Phase 2: "restart" — a new model over the same DB file reloads the cursor.
	mod2, err := model.MakeDB(dbPath)
	require.NoError(t, err)
	resumed := (&ATProtoSynchronizer{Model: mod2}).newRelayCursor(ctx, relay)
	rg, ok := resumed.groupStart()
	require.True(t, ok, "a restart should resume the stored group")
	require.Equal(t, persisted, rg, "resumed group must match the persisted one")

	// Separate sessions (avoid two subs on one): resume vs live.
	resumeSess, err := atmoq.Dial(ctx, relay, &atmoq.Options{})
	require.NoError(t, err)
	defer resumeSess.Close()
	rsub, err := resumeSess.SubscribeFrom(ctx, atmoq.DefaultBroadcast, atmoq.DefaultTrack, rg)
	require.NoError(t, err)
	_, resumeFirst, err := rsub.ReadFrame(ctx)
	require.NoError(t, err)

	liveSess, err := atmoq.Dial(ctx, relay, &atmoq.Options{})
	require.NoError(t, err)
	defer liveSess.Close()
	lsub, err := liveSess.Subscribe(ctx, atmoq.DefaultBroadcast, atmoq.DefaultTrack)
	require.NoError(t, err)
	_, liveFirst, err := lsub.ReadFrame(ctx)
	require.NoError(t, err)

	t.Logf("phase2: resumed from persisted G=%d -> first group %d ; live -> first group %d",
		rg, resumeFirst, liveFirst)
	require.LessOrEqual(t, resumeFirst, rg, "resume should replay at/before the persisted group")
	require.Greater(t, liveFirst, rg, "live edge should have advanced past the persisted group")
}
