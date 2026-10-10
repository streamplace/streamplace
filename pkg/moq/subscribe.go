package moq

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
)

// subscribeMaxAge is the Subscriber Max Age we ask for: a non-latest group
// may stay in flight this long (ms) before the publisher may drop it. A
// node pulling a stream wants every segment, so this is well past any
// hiccup a segment should survive rather than the live-only default of 0.
const subscribeMaxAge = 60_000

// gapTimeout bounds how long delivery waits for a group that has not
// arrived while a later one has. Groups travel on separate streams and may
// arrive in any order (draft §6.1.2), so a hole is usually a straggler;
// one that stays open this long is treated as dropped and skipped.
var gapTimeout = 5 * time.Second

// maxDrops bounds the SUBSCRIBE_DROP ranges a subscription remembers. A
// publisher announces a dropped range once; one that keeps announcing
// ranges far ahead of delivery is not describing a track, and is cut off
// rather than allowed to grow our state.
const maxDrops = 64

// Subscribe pulls a track from the peer and returns a Subscription to read
// its frames from. It does not wait for the publisher's answer: a
// publisher may withhold SUBSCRIBE_OK until a live track's first group
// exists (draft §5.1.2), and a refusal (a reset) surfaces from Next.
func (s *Session) Subscribe(ctx context.Context, broadcast, track string) (*Subscription, error) {
	return s.subscribe(ctx, broadcast, track, 0)
}

// subscribe is Subscribe with a Group End: the last group wanted plus one,
// or 0 for an unbounded subscription (draft §7.9).
func (s *Session) subscribe(ctx context.Context, broadcast, track string, groupEnd uint64) (*Subscription, error) {
	sub := &Subscription{s: s, done: make(chan struct{}), frames: make(chan *Frame, 16), notify: make(chan struct{}, 1)}
	s.mu.Lock()
	sub.id = s.nextID
	s.nextID++
	s.subs[sub.id] = sub
	s.mu.Unlock()

	st, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		s.remove(sub.id)
		return nil, fmt.Errorf("opening subscribe stream: %w", err)
	}
	req := msg(nil).varint(sub.id).str(broadcast).str(track).u8(0).
		varint(subscribeMaxAge).varint(0).varint(groupEnd).varint(0).varint(0).
		frame(streamSubscribe)
	if err := writeAll(st, req); err != nil {
		reset(st, errCancelled)
		s.remove(sub.id)
		return nil, fmt.Errorf("writing subscribe: %w", err)
	}
	sub.ctrl = st
	go sub.watchControl(bufio.NewReader(st))
	go sub.run()
	return sub, nil
}

func (s *Session) remove(id uint64) {
	s.mu.Lock()
	delete(s.subs, id)
	s.mu.Unlock()
}

// groupStream is an accepted group data stream, header already consumed.
type groupStream struct {
	seq uint64
	r   *bufio.Reader
	st  recvStream
}

// Subscription is a stream of frames for one subscribed track, delivered
// in group order however the groups arrive.
type Subscription struct {
	s      *Session
	id     uint64
	ctrl   bidiStream
	frames chan *Frame
	done   chan struct{}
	once   sync.Once
	notify chan struct{}

	mu sync.Mutex
	// queue holds accepted group streams by ascending sequence; next is
	// the sequence delivered next (once started, by SUBSCRIBE_OK or the
	// first group), so queue[0].seq > next is a hole. active is the
	// stream being read, so Close can cut a read short.
	queue   []groupStream
	active  recvStream
	next    uint64
	started bool
	// drops are the ranges the publisher said will never arrive, each
	// inclusive, disjoint, and ahead of next.
	drops [][2]uint64
	// ended once the publisher sent SUBSCRIBE_END (end is then the
	// exclusive last sequence) or the control stream is over (end 0).
	ended bool
	end   uint64
	err   error
}

// Next returns the next frame, blocking until one arrives, the track ends
// (io.EOF), the subscription or session ends, or ctx does.
func (sub *Subscription) Next(ctx context.Context) (*Frame, error) {
	select {
	case f, ok := <-sub.frames:
		if !ok {
			return nil, sub.finalErr()
		}
		return f, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (sub *Subscription) finalErr() error {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.err != nil {
		return sub.err
	}
	return io.EOF
}

// Close ends the subscription and releases its streams: the control
// stream, every group stream still queued, and the one being read, whose
// pending read then fails instead of waiting on the peer.
func (sub *Subscription) Close() {
	sub.once.Do(func() {
		close(sub.done)
		sub.s.remove(sub.id)
		if sub.ctrl != nil {
			reset(sub.ctrl, errCancelled)
		}
		sub.mu.Lock()
		queued, active := sub.queue, sub.active
		sub.queue, sub.active = nil, nil
		sub.mu.Unlock()
		for _, gs := range queued {
			gs.st.CancelRead(errCancelled)
		}
		if active != nil {
			active.CancelRead(errCancelled)
		}
	})
}

// enqueue queues an accepted group stream for the reader, or cancels it if
// the subscription is over. The queue is unbounded on purpose: the peer's
// stream limit and flow control already bound what it can have in flight,
// and a bounded queue would turn a slow reader into dropped groups.
func (sub *Subscription) enqueue(gs groupStream) {
	sub.mu.Lock()
	select {
	case <-sub.done:
		sub.mu.Unlock()
		gs.st.CancelRead(errCancelled)
		return
	default:
	}
	i, _ := slices.BinarySearchFunc(sub.queue, gs.seq, func(g groupStream, seq uint64) int {
		return int(g.seq - seq) // no overflow: sequences are varints, below 2^62
	})
	sub.queue = slices.Insert(sub.queue, i, gs)
	sub.mu.Unlock()
	sub.signal()
}

// addDrop records a SUBSCRIBE_DROP range, merging it into what is already
// known. A range already behind delivery is nothing to remember; too many
// ranges ahead of it is a protocol error.
func (sub *Subscription) addDrop(first, last uint64) error {
	if first > last {
		return fmt.Errorf("moq: drop range %d-%d is inverted", first, last)
	}
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if last < sub.next {
		return nil
	}
	merged := [2]uint64{first, last}
	kept := sub.drops[:0]
	for _, r := range sub.drops {
		// Overlapping or adjacent ranges fold into one.
		if r[0] <= merged[1]+1 && merged[0] <= r[1]+1 {
			merged[0] = min(merged[0], r[0])
			merged[1] = max(merged[1], r[1])
			continue
		}
		kept = append(kept, r)
	}
	sub.drops = append(kept, merged)
	if len(sub.drops) > maxDrops {
		return fmt.Errorf("moq: more than %d dropped ranges outstanding", maxDrops)
	}
	return nil
}

func (sub *Subscription) signal() {
	select {
	case sub.notify <- struct{}{}:
	default:
	}
}

func (sub *Subscription) setErr(err error) {
	sub.mu.Lock()
	if sub.err == nil {
		sub.err = err
	}
	sub.mu.Unlock()
}

// run delivers group streams in sequence order and closes the frame
// channel once the subscription is over. Locked state is consulted in
// pick; the group itself is read without the lock.
func (sub *Subscription) run() {
	defer close(sub.frames)
	defer sub.Close()
	var gap <-chan time.Time
	for {
		gs, state := sub.pick()
		switch state {
		case pickGroup:
			gap = nil
			ok := sub.readGroup(gs)
			sub.mu.Lock()
			sub.active = nil
			sub.mu.Unlock()
			if !ok {
				return
			}
			continue
		case pickOver:
			return
		case pickGap:
			if gap == nil {
				gap = time.After(gapTimeout)
			}
		case pickWait:
			gap = nil
		}
		select {
		case <-sub.notify:
		case <-gap:
			sub.skipGap()
			gap = nil
		case <-sub.done:
			return
		case <-sub.s.ctx.Done():
			sub.setErr(sub.s.Err())
			return
		}
	}
}

type pickState int

const (
	pickWait  pickState = iota // nothing to deliver yet
	pickGroup                  // the next group is here
	pickGap                    // a later group is here but next is not
	pickOver                   // the track has ended and all of it is delivered
)

// pick decides what delivery does next.
func (sub *Subscription) pick() (groupStream, pickState) {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	// SUBSCRIBE_OK and the first group travel on different streams; if
	// the group wins the race it resolves the start just as well.
	if !sub.started && len(sub.queue) > 0 {
		sub.next, sub.started = sub.queue[0].seq, true
	}
	// Groups the publisher dropped are not waited for; ranges may chain.
	for advanced := true; advanced; {
		advanced = false
		for _, r := range sub.drops {
			if r[0] <= sub.next && sub.next <= r[1] {
				sub.next = r[1] + 1
				advanced = true
			}
		}
	}
	sub.drops = slices.DeleteFunc(sub.drops, func(r [2]uint64) bool { return r[1] < sub.next })
	// Nor are groups delivery already moved past.
	for len(sub.queue) > 0 && sub.queue[0].seq < sub.next {
		sub.queue[0].st.CancelRead(errCancelled)
		sub.queue = sub.queue[1:]
	}
	if len(sub.queue) > 0 && sub.queue[0].seq == sub.next {
		gs := sub.queue[0]
		sub.queue = sub.queue[1:]
		sub.next = gs.seq + 1
		sub.active = gs.st
		return gs, pickGroup
	}
	if sub.ended && len(sub.queue) == 0 && (sub.end == 0 || sub.next >= sub.end) {
		return groupStream{}, pickOver
	}
	if len(sub.queue) > 0 || (sub.ended && sub.end > 0) {
		return groupStream{}, pickGap
	}
	return groupStream{}, pickWait
}

// skipGap gives up on the group(s) delivery is waiting for: the next
// queued group becomes next, or the track ends if nothing is queued.
func (sub *Subscription) skipGap() {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if len(sub.queue) > 0 {
		log.Warn(sub.s.ctx, "moq: groups never arrived, skipping", "from", sub.next, "to", sub.queue[0].seq-1)
		sub.next = sub.queue[0].seq
		return
	}
	if sub.ended {
		log.Warn(sub.s.ctx, "moq: groups never arrived before the track ended", "from", sub.next, "to", sub.end-1)
		sub.next = sub.end
	}
}

// readGroup delivers a group's frames until its FIN. A group the publisher
// reset is a gap, not an error: the next group is read (draft §6.3.2).
// Returns false once the subscription is over.
func (sub *Subscription) readGroup(gs groupStream) bool {
	defer gs.st.CancelRead(errCancelled)
	var ts int64
	for {
		delta, err := readVarint(gs.r)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Warn(sub.s.ctx, "moq: group ended early", "group", gs.seq, "error", err)
			}
			return true
		}
		size, err := readVarint(gs.r)
		if err != nil {
			log.Warn(sub.s.ctx, "moq: group ended early", "group", gs.seq, "error", err)
			return true
		}
		if size > MaxFrame {
			sub.s.fail(sessionProtocolViolation, fmt.Errorf("frame of %d bytes exceeds MaxFrame", size))
			return false
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(gs.r, payload); err != nil {
			log.Warn(sub.s.ctx, "moq: group ended early", "group", gs.seq, "error", err)
			return true
		}
		ts += unzigzag(delta)
		select {
		case sub.frames <- &Frame{Group: gs.seq, Timestamp: ts, Payload: payload}:
		case <-sub.done:
			return false
		case <-sub.s.ctx.Done():
			sub.setErr(sub.s.Err())
			return false
		}
	}
}

// watchControl reads the subscribe stream for its lifetime: SUBSCRIBE_OK
// resolves the start group, SUBSCRIBE_END bounds the groups still to come,
// SUBSCRIBE_DROP names ones that never will, and the stream ending (FIN or
// reset) ends the subscription.
func (sub *Subscription) watchControl(br *bufio.Reader) {
	for {
		typ, err := readVarint(br)
		if err != nil {
			sub.mu.Lock()
			if !errors.Is(err, io.EOF) && sub.err == nil {
				sub.err = fmt.Errorf("subscription reset: %w", err)
			}
			sub.ended = true
			sub.mu.Unlock()
			sub.signal()
			return
		}
		m, err := readMessage(br, maxControlMessage)
		if err != nil {
			sub.s.fail(sessionProtocolViolation, fmt.Errorf("reading subscribe response: %w", err))
			return
		}
		b := body{b: m}
		switch typ {
		case subscribeOK:
			group := b.varint()
			if err := b.done(); err != nil {
				sub.s.fail(sessionProtocolViolation, err)
				return
			}
			sub.mu.Lock()
			if !sub.started {
				sub.next, sub.started = group, true
			}
			sub.mu.Unlock()
			sub.signal()
		case subscribeEnd:
			end := b.varint()
			if err := b.done(); err != nil {
				sub.s.fail(sessionProtocolViolation, err)
				return
			}
			sub.mu.Lock()
			sub.end, sub.ended = end, true
			if !sub.started {
				// Nothing was ever resolved, so nothing is owed below end.
				sub.next, sub.started = end, true
			}
			sub.mu.Unlock()
			sub.signal()
		case subscribeDrop:
			first, last, code := b.varint(), b.varint(), b.varint()
			if err := b.done(); err != nil {
				sub.s.fail(sessionProtocolViolation, err)
				return
			}
			log.Warn(sub.s.ctx, "moq: publisher dropped groups", "from", first, "to", last, "code", code)
			if err := sub.addDrop(first, last); err != nil {
				sub.s.fail(sessionProtocolViolation, err)
				return
			}
			sub.signal()
		default:
			// Something newer: nothing to act on.
		}
	}
}
