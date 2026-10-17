package director

import (
	"context"
	"sync"
	"time"
)

// publishPatience bounds how long a segment waits for the segments before
// it to publish. Work behind a segment (the transcoder round trip, the
// packetizer) normally finishes within a second or two; a segment stuck
// far longer than that is abandoned by the ones behind it, which then
// publish, and it publishes late if it ever finishes.
const publishPatience = 15 * time.Second

// A lane hands out tickets in the order segments arrive and lets each
// segment's publication take its turn in that order, whatever order the
// work behind it finishes in. Segments are worked on concurrently (two
// transcodes in flight, a packetizer per segment), and without this the
// rendition playlists, the WebRTC feeds and the peers pulling the stream
// all received segments in completion order: out of order in every burst.
type lane struct {
	mu      sync.Mutex
	next    uint64
	head    uint64 // the ticket whose turn it is
	done    map[uint64]bool
	waiters map[uint64]chan struct{}
}

func newLane() *lane {
	return &lane{done: map[uint64]bool{}, waiters: map[uint64]chan struct{}{}}
}

// turn claims the next ticket. Every claimed turn must be released.
func (l *lane) turn() *turn {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := l.next
	l.next++
	return &turn{lane: l, ticket: t}
}

// wait blocks until it is ticket's turn. It reports false when the turn is
// gone: the segments behind it gave up waiting (see publishPatience) and
// published past it, so its work must be discarded, not published late
// and out of order; or ctx ended.
func (l *lane) wait(ctx context.Context, ticket uint64, patience time.Duration) bool {
	l.mu.Lock()
	if l.head > ticket {
		l.mu.Unlock()
		return false
	}
	if l.head == ticket {
		l.mu.Unlock()
		return true
	}
	ch, ok := l.waiters[ticket]
	if !ok {
		ch = make(chan struct{})
		l.waiters[ticket] = ch
	}
	l.mu.Unlock()
	timer := time.NewTimer(patience)
	defer timer.Stop()
	select {
	case <-ch:
		l.mu.Lock()
		defer l.mu.Unlock()
		// Woken either because it is our turn or because a later ticket
		// abandoned us.
		return l.head == ticket
	case <-ctx.Done():
		return false
	case <-timer.C:
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.head < ticket {
			// Give up on the segments before it: they are abandoned (their
			// wait reports false), and everything from here on stays in
			// order.
			for t := l.head; t < ticket; t++ {
				delete(l.done, t)
				if w, ok := l.waiters[t]; ok {
					close(w)
					delete(l.waiters, t)
				}
			}
			l.head = ticket
			delete(l.waiters, ticket)
		}
		return l.head == ticket
	}
}

func (l *lane) release(ticket uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ticket < l.head {
		return // abandoned by the segments behind it
	}
	l.done[ticket] = true
	for l.done[l.head] {
		delete(l.done, l.head)
		l.head++
	}
	if w, ok := l.waiters[l.head]; ok {
		close(w)
		delete(l.waiters, l.head)
	}
}

// A turn is one segment's place in a lane. wait blocks until the segments
// before it have published (or were given up on, see publishPatience);
// release lets the next one go. Both are safe on a nil turn, so callers without a
// lane need no branches.
type turn struct {
	lane   *lane
	ticket uint64
	once   sync.Once
}

// wait blocks for the turn and reports whether it is still worth taking:
// false means the work was abandoned by the segments behind it and must be
// discarded. A nil turn is always worth taking.
func (t *turn) wait(ctx context.Context) bool {
	if t == nil {
		return true
	}
	return t.lane.wait(ctx, t.ticket, publishPatience)
}

func (t *turn) release() {
	if t == nil {
		return
	}
	t.once.Do(func() { t.lane.release(t.ticket) })
}
