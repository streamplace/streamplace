package moq

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
)

// Announced is the peer's answer to an ANNOUNCE_REQUEST: the broadcasts
// under a prefix as the peer advertises and retracts them.
type Announced struct {
	s      *Session
	st     bidiStream
	prefix string

	mu     sync.Mutex
	err    error
	paths  []string
	out    chan Announcement
	done   chan struct{}
	closed chan struct{}
	once   sync.Once
}

// Announced asks the peer for the broadcasts under prefix. It does not
// wait for the answer: a refusal (a reset) surfaces from Next.
func (s *Session) Announced(ctx context.Context, prefix string) (*Announced, error) {
	st, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening announce stream: %w", err)
	}
	if err := writeAll(st, msg(nil).str(prefix).frame(streamAnnounce)); err != nil {
		reset(st, errCancelled)
		return nil, fmt.Errorf("writing announce request: %w", err)
	}
	a := &Announced{s: s, st: st, prefix: prefix, out: make(chan Announcement, 16), done: make(chan struct{}), closed: make(chan struct{})}
	go a.read(bufio.NewReader(st))
	return a, nil
}

// Next returns the next broadcast appearing or going away, blocking until
// one does, the peer ends the announcements (io.EOF: every route is then
// gone), the session ends, or ctx does.
func (a *Announced) Next(ctx context.Context) (Announcement, error) {
	select {
	case ann := <-a.out:
		return ann, nil
	case <-a.done:
		// Drain what was queued before the end was noticed.
		select {
		case ann := <-a.out:
			return ann, nil
		default:
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		return Announcement{}, a.err
	case <-a.s.ctx.Done():
		return Announcement{}, a.s.Err()
	case <-ctx.Done():
		return Announcement{}, ctx.Err()
	}
}

// Close ends the request; the peer sees the stream go away.
func (a *Announced) Close() {
	a.once.Do(func() {
		close(a.closed)
		reset(a.st, errCancelled)
	})
}

// emit hands an announcement to Next, unless nobody will call it again.
func (a *Announced) emit(ann Announcement) bool {
	select {
	case a.out <- ann:
		return true
	case <-a.closed:
		return false
	case <-a.s.ctx.Done():
		return false
	}
}

func (a *Announced) finish(err error) {
	a.mu.Lock()
	a.err = err
	a.mu.Unlock()
	close(a.done)
}

// read consumes the response side: ANNOUNCE_OK, then announcements until
// the peer ends the stream.
func (a *Announced) read(br *bufio.Reader) {
	m, err := readMessage(br, maxControlMessage)
	if err != nil {
		a.finish(fmt.Errorf("announce refused: %w", err))
		return
	}
	b := body{b: m}
	peerHop, _ := b.varint(), b.varint() // hop id, active count
	if err := b.done(); err != nil {
		a.s.fail(sessionProtocolViolation, fmt.Errorf("decoding announce ok: %w", err))
		a.finish(err)
		return
	}
	for {
		typ, err := readVarint(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				a.finish(io.EOF)
			} else {
				a.finish(fmt.Errorf("announce stream reset: %w", err))
			}
			return
		}
		m, err := readMessage(br, maxControlMessage)
		if err != nil {
			a.s.fail(sessionProtocolViolation, fmt.Errorf("reading announcement: %w", err))
			a.finish(err)
			return
		}
		b := body{b: m}
		switch typ {
		case announceStart:
			suffix := b.str()
			hops, err := a.hops(&b)
			if err != nil {
				a.s.fail(sessionProtocolViolation, err)
				a.finish(err)
				return
			}
			path := a.prefix + suffix
			// A route advertised twice is a protocol violation; one that
			// looped back through us is discarded, id and all
			// (draft §5.1.1.1).
			if slices.Contains(a.paths, path) {
				err := fmt.Errorf("route %q announced twice", path)
				a.s.fail(sessionProtocolViolation, err)
				a.finish(err)
				return
			}
			if slices.Contains(hops, a.s.hop) || peerHop == a.s.hop {
				a.paths = append(a.paths, "")
				continue
			}
			a.paths = append(a.paths, path)
			if !a.emit(Announcement{Path: path, Active: true}) {
				return
			}
		case announceEnd:
			id := b.varint()
			if err := b.done(); err != nil {
				a.s.fail(sessionProtocolViolation, err)
				a.finish(err)
				return
			}
			if id >= uint64(len(a.paths)) {
				err := fmt.Errorf("announce end for unknown id %d", id)
				a.s.fail(sessionProtocolViolation, err)
				a.finish(err)
				return
			}
			path := a.paths[id]
			a.paths[id] = ""
			if path != "" && !a.emit(Announcement{Path: path}) {
				return
			}
		case announceUpdate:
			// New hops or cost for a route we already have: nothing to
			// act on, but it must still decode.
			b.varint()
			if _, err := a.hops(&b); err != nil {
				a.s.fail(sessionProtocolViolation, err)
				a.finish(err)
				return
			}
		default:
			// Something newer: nothing to act on.
		}
	}
}

// hops decodes the Hop Count, Hop IDs and the two Route Costs that end an
// ANNOUNCE_START or ANNOUNCE_UPDATE, checking the message is consumed.
func (a *Announced) hops(b *body) ([]uint64, error) {
	n := b.varint()
	if n > uint64(len(b.b)) {
		return nil, fmt.Errorf("announcement claims %d hops", n)
	}
	hops := make([]uint64, 0, n)
	for i := uint64(0); i < n; i++ {
		h := b.varint()
		if h != 0 && slices.Contains(hops, h) {
			return nil, fmt.Errorf("hop %d repeated in announcement", h)
		}
		hops = append(hops, h)
	}
	b.varint() // warm route cost
	b.varint() // cold route cost
	if err := b.done(); err != nil {
		return nil, fmt.Errorf("decoding announcement: %w", err)
	}
	return hops, nil
}
