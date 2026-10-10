package moq

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
)

// ErrNotFound refuses a SUBSCRIBE or TRACK request: the broadcast or track
// is not here. The peer sees a NOT_FOUND stream reset.
var ErrNotFound = errors.New("moq: not found")

// ErrClosed is the cause a session's context carries after a local Close.
var ErrClosed = errors.New("moq: session closed")

// Publisher serves tracks to the subscribers of a session.
type Publisher interface {
	// Track resolves a track by broadcast path and name. ErrNotFound
	// refuses the request; any other error is reported as internal.
	Track(ctx context.Context, broadcast, name string) (Track, error)
}

// Track is one subscribable series of groups.
type Track interface {
	Info() TrackInfo
	// Next blocks for the next group, or returns io.EOF once the track has
	// ended. ctx ends when the subscriber goes away.
	Next(ctx context.Context) (*Group, error)
	Close()
}

// TrackInfo is a track's immutable publisher properties (TRACK_INFO).
type TrackInfo struct {
	Priority uint8
	// MaxAge bounds how long a non-latest group stays available after a
	// newer one arrives.
	MaxAge time.Duration
	// Timescale is frame-timestamp units per second; must be non-zero.
	Timescale uint64
}

// Group is a run of frames delivered reliably and in order on one stream.
type Group struct {
	Sequence uint64
	Frames   []Frame
}

// Frame is one payload within a group.
type Frame struct {
	// Group is the sequence of the group the frame arrived in (set on receive).
	Group uint64
	// Timestamp is in the track's timescale.
	Timestamp int64
	Payload   []byte
}

// Session is one moq-lite session, in either role: with a Publisher it
// serves the peer's subscriptions; Subscribe pulls tracks from the peer.
type Session struct {
	conn   conn
	pub    Publisher
	ctx    context.Context
	cancel context.CancelCauseFunc
	// hop is this endpoint's Hop ID, stamped on announcements.
	hop uint64

	mu     sync.Mutex
	subs   map[uint64]*Subscription
	nextID uint64
}

// newSession sends SETUP and starts accepting the peer's streams. Both roles
// run the same loops; what is accepted differs by whether pub is set.
func newSession(c conn, pub Publisher, setup []byte) (*Session, error) {
	ctx, cancel := context.WithCancelCause(c.Context())
	var hop [8]byte
	if _, err := rand.Read(hop[:]); err != nil {
		cancel(err)
		return nil, err
	}
	s := &Session{
		conn:   c,
		pub:    pub,
		ctx:    ctx,
		cancel: cancel,
		hop:    binary.BigEndian.Uint64(hop[:])&maxVarint | 1,
		subs:   map[uint64]*Subscription{},
	}
	st, err := c.OpenUniStreamSync(ctx)
	if err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("opening setup stream: %w", err))
		return nil, err
	}
	if err := writeAll(st, setup); err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("writing setup: %w", err))
		return nil, err
	}
	if err := st.Close(); err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("closing setup stream: %w", err))
		return nil, err
	}
	go s.acceptUni()
	go s.acceptBidi()
	return s, nil
}

// setupMessage encodes a SETUP with the given parameters, each a raw value.
func setupMessage(params map[uint64][]byte) []byte {
	m := msg(nil).varint(uint64(len(params)))
	for id, v := range params {
		m = m.varint(id).varint(uint64(len(v)))
		m = append(m, v...)
	}
	return m.frame(streamSetup)
}

// Close ends the session; every subscription fails and every served track
// is closed.
func (s *Session) Close() error {
	s.cancel(ErrClosed)
	return s.conn.CloseWithError(sessionNoError, "")
}

// Done ends when the session does, for either reason.
func (s *Session) Done() <-chan struct{} { return s.ctx.Done() }

// Err is why the session ended: ErrClosed after Close, the transport's
// error when the peer or network ended it, or nil while it is live.
func (s *Session) Err() error {
	if s.ctx.Err() == nil {
		return nil
	}
	return context.Cause(s.ctx)
}

// fail ends the session over a peer error.
func (s *Session) fail(code uint64, err error) {
	s.cancel(err)
	_ = s.conn.CloseWithError(code, err.Error())
}

func (s *Session) acceptUni() {
	for {
		st, err := s.conn.AcceptUniStream(s.ctx)
		if err != nil {
			s.cancel(err)
			return
		}
		// The header is read here, in accept order, which is what keeps a
		// subscription's groups in the order the publisher opened them.
		// Only the header: a group's frames are drained by the subscription,
		// so a slow one cannot stall acceptance.
		s.handleUni(st)
	}
}

func (s *Session) acceptBidi() {
	for {
		st, err := s.conn.AcceptStream(s.ctx)
		if err != nil {
			s.cancel(err)
			return
		}
		go s.handleBidi(st)
	}
}

// handleUni routes a data stream: the peer's SETUP (read and, carrying no
// capability we act on, discarded) or a group for one of our subscriptions.
func (s *Session) handleUni(st recvStream) {
	br := bufio.NewReader(st)
	typ, err := readVarint(br)
	if err != nil {
		st.CancelRead(errInternal)
		return
	}
	switch typ {
	case streamSetup:
		if _, err := readMessage(br, maxControlMessage); err != nil {
			s.fail(sessionProtocolViolation, fmt.Errorf("reading setup: %w", err))
		}
	case streamGroup:
		m, err := readMessage(br, maxControlMessage)
		if err != nil {
			s.fail(sessionProtocolViolation, fmt.Errorf("reading group header: %w", err))
			return
		}
		b := body{b: m}
		id, seq, frameStart := b.varint(), b.varint(), b.varint()
		if err := b.done(); err != nil {
			s.fail(sessionProtocolViolation, fmt.Errorf("decoding group header: %w", err))
			return
		}
		if frameStart != 0 {
			// Only a subscriber that asked for a partial group is ever
			// sent one, and we never ask (draft §3.6).
			s.fail(sessionProtocolViolation, fmt.Errorf("unsolicited partial group %d", seq))
			return
		}
		s.mu.Lock()
		sub := s.subs[id]
		s.mu.Unlock()
		if sub == nil {
			st.CancelRead(errCancelled)
			return
		}
		sub.enqueue(groupStream{seq: seq, r: br, st: st})
	default:
		// Unknown stream types are reset, never fatal (draft §7.2).
		st.CancelRead(errCancelled)
	}
}

// handleBidi routes a control stream. Fetch and Probe are not served, and
// a session without a Publisher serves nothing: all are refused with a
// reset, which is how a subscriber distinguishes refusal from pending.
func (s *Session) handleBidi(st bidiStream) {
	br := bufio.NewReader(st)
	typ, err := readVarint(br)
	if err != nil {
		reset(st, errInternal)
		return
	}
	switch typ {
	case streamGoaway:
		s.serveGoaway(br, st)
	case streamSubscribe:
		if s.pub == nil {
			reset(st, errNotFound)
			return
		}
		s.serveSubscribe(br, st)
	case streamTrack:
		if s.pub == nil {
			reset(st, errNotFound)
			return
		}
		s.serveTrack(br, st)
	case streamAnnounce:
		if s.pub == nil {
			reset(st, errUnroutable)
			return
		}
		s.serveAnnounce(br, st)
	case streamFetch:
		reset(st, errNotFound)
	default:
		reset(st, errCancelled)
	}
}

// serveGoaway reads a GOAWAY and waits for the peer to end the stream. No
// migration: the peer closes the session when it is done draining, and the
// subscriptions fail with it, which is what a subscriber reconnects on.
func (s *Session) serveGoaway(br *bufio.Reader, st bidiStream) {
	m, err := readMessage(br, maxControlMessage)
	if err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("reading goaway: %w", err))
		return
	}
	b := body{b: m}
	uri := b.str()
	if err := b.done(); err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("decoding goaway: %w", err))
		return
	}
	if s.pub != nil && uri != "" {
		// A client cannot redirect a server (draft §7.18).
		s.fail(sessionProtocolViolation, errors.New("client sent a goaway uri"))
		return
	}
	log.Log(s.ctx, "moq: peer is going away", "uri", uri)
	_, _ = io.Copy(io.Discard, br)
	_ = st.Close()
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}
