package atproto

import (
	"sync"
	"time"
)

// firehoseDeduper collapses duplicate firehose events that arrive when we
// subscribe to more than one relay (and to our own PDS). The same repo commit
// is forwarded by every relay that carries it; its commit CID is content
// addressed, so it is byte-identical regardless of which relay delivered it.
// That makes the commit CID a perfect cross-relay dedup key.
//
// It is a two-generation rotating set: keys live in cur until a window elapses,
// then cur becomes prev and a fresh cur is started. A lookup checks both
// generations, so any key is remembered for at least one window and at most
// two. Expiry is free (drop the old map) and there are no per-entry timers.
//
// Relays run within seconds of each other, so a window of a few minutes
// absorbs all realistic skew. If a duplicate ever does slip past (e.g. a relay
// lagging longer than the window), the downstream handlers are idempotent, so
// the worst case is wasted work — never lost or corrupted data.
type firehoseDeduper struct {
	mu       sync.Mutex
	cur      map[string]struct{}
	prev     map[string]struct{}
	window   time.Duration
	lastSwap time.Time
}

func newFirehoseDeduper(window time.Duration) *firehoseDeduper {
	return &firehoseDeduper{
		cur:      make(map[string]struct{}),
		prev:     make(map[string]struct{}),
		window:   window,
		lastSwap: time.Now(),
	}
}

// seen reports whether key has been observed within the dedup window. The first
// call for a key returns false and records it; subsequent calls return true
// until the key ages out. It is safe for concurrent use, and two callers racing
// on the same key are serialized so exactly one sees false.
func (d *firehoseDeduper) seen(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if time.Since(d.lastSwap) >= d.window {
		d.prev = d.cur
		d.cur = make(map[string]struct{}, len(d.prev))
		d.lastSwap = time.Now()
	}

	if _, ok := d.cur[key]; ok {
		return true
	}
	if _, ok := d.prev[key]; ok {
		// Promote into cur so a steadily-recurring key doesn't age out from
		// under us at a generation boundary.
		d.cur[key] = struct{}{}
		return true
	}

	d.cur[key] = struct{}{}
	return false
}
