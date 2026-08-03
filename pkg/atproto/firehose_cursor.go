package atproto

import (
	"context"
	"sync/atomic"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

// cursorFlushInterval bounds how often a relay's progress is written to the
// index DB. The firehose is high-volume, so we persist on a timer rather than
// per-event; between flushes the latest seq lives in memory and already covers
// in-process reconnects.
const cursorFlushInterval = 5 * time.Second

// highWater is a concurrency-safe monotonically-increasing maximum.
type highWater struct{ v atomic.Int64 }

// observe raises the mark to seq if it is higher. Safe for concurrent callers.
func (h *highWater) observe(seq int64) {
	for {
		cur := h.v.Load()
		if seq <= cur {
			return
		}
		if h.v.CompareAndSwap(cur, seq) {
			return
		}
	}
}

func (h *highWater) get() int64 { return h.v.Load() }

// set overwrites the mark unconditionally, including downward. Only for
// initialization and reset; everything else goes through observe.
func (h *highWater) set(seq int64) { h.v.Store(seq) }

// relayCursor tracks how far we've consumed one relay's firehose so we can
// resume after a disconnect or restart instead of re-tailing from live (which
// would leave a gap). It keeps the high-water sequence number in memory,
// updated on every frame, and persists it periodically and once on shutdown.
//
// Because the parallel scheduler may surface frames slightly out of sequence
// order, the persisted cursor is the highest seq observed — on an unclean crash
// a handful of in-flight frames just below it can be skipped on resume. That is
// safe here: downstream handlers are idempotent, and with several relays plus a
// cold deduper after restart, those commits get re-delivered and re-indexed.
//
// Alongside the sequence it tracks the time of the newest event seen, which is
// what makes the cursor's age knowable and so gives [relayCursor.dropIfStale]
// something to judge. A sequence number on its own says nothing about how much
// history resuming from it would ask the relay to re-send.
type relayCursor struct {
	host  string
	model model.Model

	// latest is the high-water cursor: the upstream at-sequence for a WebSocket
	// relay, or the high-water MoQ group sequence for a moqt:// relay (used to
	// resume replay via SubscribeFrom — see connectRelayMoq). A host is one
	// transport or the other, so a single value covers both — but the two
	// sequence spaces must never mix: upstream at-seqs are orders of magnitude
	// larger than group sequences, so on a moq host only observeGroup may feed
	// this (repoStreamCallbacks keeps the upstream seq out of it and in the
	// metrics gauge only). 0 = nothing seen yet (tail from live). Persisted
	// periodically so a Streamplace restart resumes from here — the relay
	// assigns durable ids across its own restarts, so a stored cursor stays
	// valid (it just ages out of the relay's replay window if we are down too
	// long, which is the gap PDS re-sync covers).
	latest highWater
	// lastEvent is the unix-seconds stamp of the newest event seen on this
	// relay, high-watered the same way and for the same reason: event times
	// jitter slightly out of order across a relay's own workers, and the newest
	// one is the only one that says anything about how current we are. Unlike
	// latest it is transport-independent — every relay stamps its events with a
	// wall-clock time — so it is fed on both the WebSocket and moq paths.
	lastEvent highWater

	flushed      int64 // last persisted cursor; only the flush loop touches it
	flushedEvent int64 // last persisted event time; likewise
}

func (atsync *ATProtoSynchronizer) newRelayCursor(ctx context.Context, host string) *relayCursor {
	rc := &relayCursor{host: host, model: atsync.Model}
	stored, err := atsync.Model.GetRelayCursor(host)
	if err != nil {
		log.Error(ctx, "failed to load relay cursor; tailing from live", "err", err)
		return rc
	}
	if stored != nil {
		rc.latest.set(stored.Cursor)
		rc.flushed = stored.Cursor
		rc.lastEvent.set(stored.LastEventTime)
		rc.flushedEvent = stored.LastEventTime
		log.Log(ctx, "resuming relay from stored cursor", "cursor", stored.Cursor,
			"lastEventTime", stored.LastEventTime)
	}
	return rc
}

// observe advances the high-water mark. Safe for concurrent callers (the
// scheduler runs several event workers).
func (rc *relayCursor) observe(seq int64) {
	rc.latest.observe(seq)
}

// observeTime records the time stamped on an event we just received, which is
// what [relayCursor.dropIfStale] later measures the cursor's age against. Safe
// for concurrent callers; max-wins, so a slightly out-of-order stamp cannot
// walk the mark backwards.
func (rc *relayCursor) observeTime(t time.Time) {
	rc.lastEvent.observe(t.Unix())
}

// lastEventTime is the unix-seconds stamp of the newest event seen so far, or 0
// if we have never seen one (and never loaded one from the index).
func (rc *relayCursor) lastEventTime() int64 { return rc.lastEvent.get() }

// stale reports whether resuming from this cursor would ask the relay to replay
// more than window of history.
//
// A zero cursor is never stale: there is nothing to replay from, so the connect
// tails live regardless. A nonzero cursor with NO event time is always stale —
// that is a row written before this column existed, and it is exactly the state
// that melted production: unknown age, unbounded replay. A non-positive window
// disables the whole check.
func (rc *relayCursor) stale(window time.Duration, now time.Time) bool {
	if window <= 0 {
		return false
	}
	if rc.latest.get() == 0 {
		return false
	}
	last := rc.lastEvent.get()
	if last == 0 {
		return true
	}
	return now.Sub(time.Unix(last, 0)) > window
}

// dropIfStale abandons the cursor when its last observed event is older than
// window, so the connect tails live instead of replaying a backlog the sweep
// can heal more cheaply. Returns true if it dropped.
//
// Called at the top of every connect attempt rather than once at load, because
// a relay also goes stale mid-process: a long disconnect-and-backoff stretch
// leaves a cursor that was fine when we loaded it pointing hours into the past
// by the time we get back in.
func (rc *relayCursor) dropIfStale(ctx context.Context, window time.Duration) bool {
	if !rc.stale(window, time.Now()) {
		return false
	}
	seq := rc.latest.get()
	age := "unknown"
	if last := rc.lastEvent.get(); last != 0 {
		age = time.Since(time.Unix(last, 0)).String()
	}
	log.Warn(ctx, "abandoning stale relay cursor; tailing live and leaving the gap to the sweep",
		"relay", rc.host, "cursor", seq, "lastEventAge", age, "replayWindow", window)
	rc.reset()
	return true
}

// param returns the cursor to dial with and whether to send one at all. With no
// progress yet we send none, so a fresh external relay tails from live instead
// of backfilling its entire history.
func (rc *relayCursor) param() (int64, bool) {
	v := rc.latest.get()
	return v, v > 0
}

// highSeq returns the high-water upstream sequence number observed so far.
func (rc *relayCursor) highSeq() int64 { return rc.latest.get() }

// observeGroup advances the high-water cursor from a MoQ group sequence (the
// moqt:// transport's flavour of a cursor). Called on every frame received from
// a moqt:// relay (concurrency-safe).
func (rc *relayCursor) observeGroup(seq uint64) {
	rc.observe(int64(seq))
}

// groupStart returns the MoQ group to resume replay from and whether to request
// replay at all. Before any frame is seen we tail the live edge; after a
// reconnect we resume from the last group seen so the relay replays from there
// (already-seen frames in that group are deduped downstream). Group 0 (a
// brand-new relay's very first group) reads as "none" and tails live — harmless
// and self-healing, and in practice relay group ids are large (seeded for
// durability across restarts).
func (rc *relayCursor) groupStart() (uint64, bool) {
	v := rc.latest.get()
	if v <= 0 {
		return 0, false
	}
	return uint64(v), true
}

// reset abandons the cursor so the next connect tails the live edge. For when
// a resume proves the stored value can't be trusted — e.g. a row poisoned by
// upstream at-seqs before the group/at-seq split in repoStreamCallbacks, or one
// so old that replaying it would cost more than the sweep — which observe()'s
// max-wins semantics could otherwise never walk back. The flush loop persists
// the zeroes, healing the stored row too.
//
// The event time goes with it: keeping a fresh timestamp next to a zero cursor
// would describe a relay we are current with, which is the opposite of what a
// reset means.
func (rc *relayCursor) reset() {
	rc.latest.set(0)
	rc.lastEvent.set(0)
}

// flush persists the high-water marks if either has advanced since the last
// write. Only ever called from the single flush goroutine, so the flushed
// values are unsynchronized.
func (rc *relayCursor) flush(ctx context.Context) {
	v := rc.latest.get()
	t := rc.lastEvent.get()
	if v == rc.flushed && t == rc.flushedEvent {
		return
	}
	if err := rc.model.UpsertRelayCursor(rc.host, v, t); err != nil {
		log.Error(ctx, "failed to persist relay cursor", "err", err, "cursor", v, "lastEventTime", t)
		return
	}
	rc.flushed = v
	rc.flushedEvent = t
}
