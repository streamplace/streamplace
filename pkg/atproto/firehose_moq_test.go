package atproto

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/ipfs/go-cid"
	atmoq "github.com/streamplace/atmoq-go"
	"github.com/stretchr/testify/require"
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
		Commit: lexutil.LexLink(mustCID(t)),
		Time:   "2026-06-25T00:00:00Z",
		Blocks: lexutil.LexBytes{},
		Ops:    []*comatproto.SyncSubscribeRepos_RepoOp{},
		Blobs:  []lexutil.LexLink{},
	}
	require.NoError(t, commit.MarshalCBOR(&buf))

	sched, out := collectScheduler(t)
	atsync := &ATProtoSynchronizer{}
	require.NoError(t, atsync.dispatchMoqFrame(context.Background(), buf.Bytes(), sched))

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
	require.NoError(t, atsync.dispatchMoqFrame(context.Background(), buf.Bytes(), sched))

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
	require.NoError(t, atsync.dispatchMoqFrame(context.Background(), buf.Bytes(), sched))

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
		require.NoError(t, atsync.dispatchMoqFrame(ctx, raw, sched))
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
