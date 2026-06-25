package atproto

import (
	"bytes"
	"context"
	"fmt"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
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
// MoQ has no cursor/replay (subscriptions always start at the publisher's latest
// group), so the stored cursor is observed for liveness but never used to
// resume. Gaps across a reconnect are covered by cross-relay redelivery and the
// idempotent handlers, exactly as for the best-effort WebSocket cursor.
func (atsync *ATProtoSynchronizer) connectRelayMoq(ctx context.Context, relay string, cursor *relayCursor) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sess, err := atmoq.Dial(streamCtx, relay, &atmoq.Options{})
	if err != nil {
		return fmt.Errorf("dialing moq relay: %w", err)
	}
	defer sess.Close()

	sub, err := sess.Subscribe(streamCtx, atmoq.DefaultBroadcast, atmoq.DefaultTrack)
	if err != nil {
		return fmt.Errorf("subscribing to moq firehose: %w", err)
	}
	defer sub.Close()

	spmetrics.FirehoseRelaysConnected.WithLabelValues(relay).Set(1)
	defer spmetrics.FirehoseRelaysConnected.WithLabelValues(relay).Set(0)

	rsc := atsync.repoStreamCallbacks(ctx, cursor, cancel)
	scheduler := parallel.NewScheduler(10, 100, relay, rsc.EventHandler)
	defer scheduler.Shutdown()

	log.Log(ctx, "connected to relay firehose (moq)", "version", sess.Version())
	for {
		raw, _, err := sub.ReadFrame(streamCtx)
		if err != nil {
			if streamCtx.Err() != nil {
				return streamCtx.Err()
			}
			return fmt.Errorf("moq firehose read: %w", err)
		}
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
