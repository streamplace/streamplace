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
type relayCursor struct {
	host  string
	model model.Model

	latest  atomic.Int64 // highest seq seen; 0 = nothing yet (tail from live)
	flushed int64        // last persisted value; only the flush loop touches it

	// group is the highest MoQ group sequence seen on a moqt:// relay, used to
	// resume replay across reconnects (see connectRelayMoq). -1 = none yet, tail
	// from the live edge. In-memory only: it resumes within a process run, not
	// across restarts (the relay's RAM replay window is lost on its restart
	// anyway, and group ids aren't yet durable server-side).
	group atomic.Int64
}

func (atsync *ATProtoSynchronizer) newRelayCursor(ctx context.Context, host string) *relayCursor {
	rc := &relayCursor{host: host, model: atsync.Model}
	rc.group.Store(-1) // -1 = no MoQ group seen yet (tail from live edge)
	stored, err := atsync.Model.GetRelayCursor(host)
	if err != nil {
		log.Error(ctx, "failed to load relay cursor; tailing from live", "err", err)
		return rc
	}
	if stored != nil {
		rc.latest.Store(stored.Cursor)
		rc.flushed = stored.Cursor
		log.Log(ctx, "resuming relay from stored cursor", "cursor", stored.Cursor)
	}
	return rc
}

// observe advances the high-water mark. Safe for concurrent callers (the
// scheduler runs several event workers).
func (rc *relayCursor) observe(seq int64) {
	for {
		cur := rc.latest.Load()
		if seq <= cur {
			return
		}
		if rc.latest.CompareAndSwap(cur, seq) {
			return
		}
	}
}

// param returns the cursor to dial with and whether to send one at all. With no
// progress yet we send none, so a fresh external relay tails from live instead
// of backfilling its entire history.
func (rc *relayCursor) param() (int64, bool) {
	v := rc.latest.Load()
	return v, v > 0
}

// observeGroup advances the high-water MoQ group sequence. Called on every
// frame received from a moqt:// relay (concurrency-safe).
func (rc *relayCursor) observeGroup(seq uint64) {
	s := int64(seq)
	for {
		cur := rc.group.Load()
		if s <= cur {
			return
		}
		if rc.group.CompareAndSwap(cur, s) {
			return
		}
	}
}

// groupStart returns the MoQ group to resume replay from and whether to request
// replay at all. Before any frame is seen we tail the live edge; after a
// reconnect we resume from the last group seen so the relay replays from there
// (already-seen frames in that group are deduped downstream).
func (rc *relayCursor) groupStart() (uint64, bool) {
	v := rc.group.Load()
	if v < 0 {
		return 0, false
	}
	return uint64(v), true
}

// flush persists the high-water mark if it has advanced since the last write.
// Only ever called from the single flush goroutine, so flushed is unsynchronized.
func (rc *relayCursor) flush(ctx context.Context) {
	v := rc.latest.Load()
	if v == rc.flushed {
		return
	}
	if err := rc.model.UpsertRelayCursor(rc.host, v); err != nil {
		log.Error(ctx, "failed to persist relay cursor", "err", err, "cursor", v)
		return
	}
	rc.flushed = v
}
