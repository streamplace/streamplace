package atproto

import (
	"bytes"
	"context"
	"fmt"

	"stream.place/streamplace/pkg/comatproto"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	atmoq "github.com/streamplace/atmoq-go"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// connectRelayMoq consumes one relay's atproto firehose over MoQ transport
// (moqt:// and its aliases) instead of WebSocket, and pumps it until the
// connection drops or ctx is cancelled. atmoq-go subscribes at the live edge
// and delivers frames that are byte-identical to com.atproto.sync.subscribeRepos
// WebSocket messages, so each frame decodes through the same indigo event types
// and feeds the same scheduler + callbacks as connectRelay's WebSocket path.
//
// On a reconnect or restart it resumes replay from the last MoQ group it saw
// (via SubscribeFrom, served from the relay's replay window); on the first
// connect it tails the live edge. Gaps past the relay's retained window are
// covered by cross-relay redelivery and the idempotent handlers, exactly as for
// the best-effort WebSocket cursor.
func (atsync *ATProtoSynchronizer) connectRelayMoq(ctx context.Context, relay string, cursor *relayCursor) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sess, err := atmoq.Dial(streamCtx, relay, &atmoq.Options{})
	if err != nil {
		return fmt.Errorf("dialing moq relay: %w", err)
	}
	defer sess.Close()

	// Resume from the last group seen on a reconnect; tail the live edge on the
	// first connect. If that group has aged out of the relay's replay window the
	// relay jumps forward to the oldest it still retains, leaving a gap we accept
	// here (deep recovery is a PDS re-sync, tracked separately).
	var sub *atmoq.Subscription
	if g, ok := cursor.groupStart(); ok {
		sub, err = sess.SubscribeFrom(streamCtx, atmoq.DefaultBroadcast, atmoq.DefaultTrack, g)
	} else {
		sub, err = sess.Subscribe(streamCtx, atmoq.DefaultBroadcast, atmoq.DefaultTrack)
	}
	if err != nil {
		return fmt.Errorf("subscribing to moq firehose: %w", err)
	}
	defer sub.Close()

	protocol := relayProtocol(relay)
	spmetrics.FirehoseRelaysConnected.WithLabelValues(relay, protocol).Set(1)
	defer spmetrics.FirehoseRelaysConnected.WithLabelValues(relay, protocol).Set(0)

	rsc := atsync.repoStreamCallbacks(ctx, relay, cursor, cancel)
	scheduler := parallel.NewScheduler(10, 100, relay, rsc.EventHandler)
	defer scheduler.Shutdown()

	log.Log(ctx, "connected to relay firehose (moq)", "version", sess.Version())
	for {
		raw, group, err := sub.ReadFrame(streamCtx)
		if err != nil {
			if streamCtx.Err() != nil {
				return streamCtx.Err()
			}
			return fmt.Errorf("moq firehose read: %w", err)
		}
		// Track the group on every frame (before dedup) so a reconnect resumes
		// from here; replayed frames are absorbed by the commit-CID deduper.
		cursor.observeGroup(group)
		if err := atsync.dispatchMoqFrame(streamCtx, raw, scheduler); err != nil {
			return err
		}
	}
}

// dispatchMoqFrame decodes one MoQ firehose frame — a DAG-CBOR EventHeader
// followed by the event payload, the same bytes a subscribeRepos WebSocket
// message carries — and hands the events we index (commit, identity) to the
// scheduler. This mirrors HandleRepoStream's decode switch, but only the
// message types whose callbacks repoStreamCallbacks registers; other types
// (#sync/#account/#info/#labels) are decoded-but-ignored just as they are over
// WebSocket (no matching callback => dropped).
func (atsync *ATProtoSynchronizer) dispatchMoqFrame(ctx context.Context, raw []byte, scheduler *parallel.Scheduler) error {
	r := bytes.NewReader(raw)

	var header events.EventHeader
	if err := header.UnmarshalCBOR(r); err != nil {
		return fmt.Errorf("reading moq frame header: %w", err)
	}

	switch header.Op {
	case events.EvtKindMessage:
		switch header.MsgType {
		case "#commit":
			var evt comatproto.SyncSubscribeRepos_Commit
			if err := evt.UnmarshalCBOR(r); err != nil {
				return fmt.Errorf("reading moq commit event: %w", err)
			}
			return scheduler.AddWork(ctx, evt.Repo, &events.XRPCStreamEvent{RepoCommit: &evt})
		case "#identity":
			var evt comatproto.SyncSubscribeRepos_Identity
			if err := evt.UnmarshalCBOR(r); err != nil {
				return fmt.Errorf("reading moq identity event: %w", err)
			}
			return scheduler.AddWork(ctx, evt.Did, &events.XRPCStreamEvent{RepoIdentity: &evt})
		default:
			return nil
		}
	case events.EvtKindErrorFrame:
		var errframe events.ErrorFrame
		if err := errframe.UnmarshalCBOR(r); err != nil {
			return fmt.Errorf("reading moq error frame: %w", err)
		}
		return fmt.Errorf("moq firehose error frame: %s: %s", errframe.Error, errframe.Message)
	default:
		return fmt.Errorf("unrecognized moq event op: %d", header.Op)
	}
}
