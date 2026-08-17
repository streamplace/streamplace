package atproto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	"github.com/fxamacker/cbor/v2"
	atmoq "github.com/streamplace/atmoq/go"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// moqResumeProbeTimeout bounds how long a resumed subscription may stay silent
// before we conclude the cursor is bad and re-tail live. The firehose delivers
// many events per second, so a healthy resume answers near-instantly; this only
// needs to be generous enough to cover a slow disk backfill starting up.
const moqResumeProbeTimeout = 30 * time.Second

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
	//
	// A cursor too old to be worth replaying at all has already been dropped:
	// connectRelay runs the staleness check before it dispatches here, and it is
	// the only caller, so groupStart below already reflects that decision.
	var sub *atmoq.Subscription
	resumedFrom, resumed := cursor.groupStart()
	if resumed {
		sub, err = sess.SubscribeFrom(streamCtx, atmoq.DefaultBroadcast, atmoq.DefaultTrack, resumedFrom)
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
		// The relay accepts a SubscribeFrom for any group, including one past
		// its live edge that it will never produce (a stale or corrupted
		// cursor), and just serves silence. So probe the first frame of a
		// resume: if replay yields nothing, abandon the cursor and reconnect
		// at the live edge — the skipped replay is a gap of the kind we
		// already accept (idempotent handlers + cross-relay redelivery).
		rctx := streamCtx
		var cancelProbe context.CancelFunc
		if resumed {
			rctx, cancelProbe = context.WithTimeout(streamCtx, moqResumeProbeTimeout)
		}
		raw, group, err := sub.ReadFrame(rctx)
		if cancelProbe != nil {
			cancelProbe()
		}
		if err != nil {
			if streamCtx.Err() != nil {
				return streamCtx.Err()
			}
			if resumed && errors.Is(err, context.DeadlineExceeded) {
				cursor.reset()
				return fmt.Errorf("moq resume from group %d served nothing for %s; cursor reset, tailing live on reconnect", resumedFrom, moqResumeProbeTimeout)
			}
			return fmt.Errorf("moq firehose read: %w", err)
		}
		resumed = false
		// Track the group on every frame (before dedup) so a reconnect resumes
		// from here; replayed frames are absorbed by the commit-CID deduper.
		cursor.observeGroup(group)
		if err := atsync.dispatchMoqFrame(streamCtx, raw, group, relay, protocol, scheduler); err != nil {
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
//
// A frame that fails strict decoding is logged (with whatever DID/seq context
// a lenient decode can salvage) and SKIPPED, not returned as an error: one
// undecodable event must not take down the whole relay connection, and — per
// the 2026-07-15 investigation — such frames have so far been relay-side
// serving corruption, not user data, so there is nothing to do but drop them
// and let cross-relay redelivery / PDS re-sync repair the gap. Only an
// explicit protocol-level ErrorFrame (or a scheduler/ctx failure) is fatal.
func (atsync *ATProtoSynchronizer) dispatchMoqFrame(ctx context.Context, raw []byte, group uint64, relay, protocol string, scheduler *parallel.Scheduler) error {
	skip := func(stage string, err error) error {
		spmetrics.FirehoseBadFramesTotal.WithLabelValues(relay, protocol).Inc()
		fields := append([]any{"stage", stage, "err", err, "group", group, "len", len(raw)}, badFrameContext(raw)...)
		log.Warn(ctx, "skipping undecodable firehose frame", fields...)
		return nil
	}

	r := bytes.NewReader(raw)

	var header events.EventHeader
	if err := header.UnmarshalCBOR(r); err != nil {
		return skip("header", err)
	}

	switch header.Op {
	case events.EvtKindMessage:
		switch header.MsgType {
		case "#commit":
			var evt indigoatproto.SyncSubscribeRepos_Commit
			if err := evt.UnmarshalCBOR(r); err != nil {
				return skip("commit", err)
			}
			return scheduler.AddWork(ctx, evt.Repo, &events.XRPCStreamEvent{RepoCommit: &evt})
		case "#identity":
			var evt indigoatproto.SyncSubscribeRepos_Identity
			if err := evt.UnmarshalCBOR(r); err != nil {
				return skip("identity", err)
			}
			return scheduler.AddWork(ctx, evt.Did, &events.XRPCStreamEvent{RepoIdentity: &evt})
		default:
			return nil
		}
	case events.EvtKindErrorFrame:
		var errframe events.ErrorFrame
		if err := errframe.UnmarshalCBOR(r); err != nil {
			return skip("errorframe", err)
		}
		return fmt.Errorf("moq firehose error frame: %s: %s", errframe.Error, errframe.Message)
	default:
		return skip("op", fmt.Errorf("unrecognized moq event op: %d", header.Op))
	}
}

// badFrameContext best-effort-decodes a frame that failed strict decoding so
// the skip log can say whose event it was. Lenient on purpose (invalid UTF-8,
// unknown simple values, and truncation all still yield whatever fields sit in
// the valid prefix); returns log key/value pairs, possibly none.
func badFrameContext(raw []byte) []any {
	dm, err := cbor.DecOptions{UTF8: cbor.UTF8DecodeInvalid}.DecMode()
	if err != nil {
		return nil
	}
	dec := dm.NewDecoder(bytes.NewReader(raw))
	var hdr, pay map[string]any
	_ = dec.Decode(&hdr)
	_ = dec.Decode(&pay)
	var fields []any
	if t, ok := hdr["t"].(string); ok {
		fields = append(fields, "msgType", t)
	}
	// #commit events carry the repo in "repo"; #identity/#account in "did".
	for _, k := range []string{"repo", "did"} {
		if v, ok := pay[k].(string); ok {
			fields = append(fields, "did", v)
			break
		}
	}
	if seq, ok := pay["seq"]; ok {
		fields = append(fields, "seq", seq)
	}
	if rev, ok := pay["rev"].(string); ok {
		fields = append(fields, "rev", rev)
	}
	return fields
}
