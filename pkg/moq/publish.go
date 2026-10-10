package moq

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"stream.place/streamplace/pkg/log"
)

// serveSubscribe serves one subscription: resolve the track, acknowledge
// the first group's sequence, then ship every group on its own
// unidirectional stream until the track ends or the subscriber lets go.
func (s *Session) serveSubscribe(br *bufio.Reader, st bidiStream) {
	m, err := readMessage(br, maxControlMessage)
	if err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("reading subscribe: %w", err))
		return
	}
	b := body{b: m}
	id := b.varint()
	broadcast, name := b.str(), b.str()
	b.u8()                                                        // subscriber priority
	_, groupStart, groupEnd := b.varint(), b.varint(), b.varint() // max age, group floor, group end
	frameStart, frameEnd := b.varint(), b.varint()                // frame bounds
	if err := b.done(); err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("decoding subscribe: %w", err))
		return
	}
	if frameStart != 0 || frameEnd != 0 {
		// Live groups are whole here; a partial-group request would be
		// answered from a later group anyway (draft §3.6), so refuse it.
		reset(st, errNotFound)
		return
	}
	ctx := log.WithLogValues(s.ctx, "broadcast", broadcast, "track", name)
	track, err := s.pub.Track(ctx, broadcast, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			reset(st, errNotFound)
		} else {
			log.Warn(ctx, "moq: track lookup failed", "error", err)
			reset(st, errInternal)
		}
		return
	}
	defer track.Close()

	// The subscriber ending its side (FIN or reset) ends the subscription.
	// SUBSCRIBE_UPDATE is the only other thing it can send, and the
	// delivery preferences it carries change nothing about a track that
	// is served whole and in order.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer cancel()
		for {
			if _, err := readVarint(br); err != nil {
				return
			}
			if _, err := readMessage(br, maxControlMessage); err != nil {
				return
			}
		}
	}()

	first := true
	var next uint64
	for {
		g, err := track.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Every group below next has been sent, so END is also
				// the moment the stream can be finished (draft §5.1.2).
				if werr := writeAll(st, msg(nil).varint(next).frame(subscribeEnd)); werr != nil {
					reset(st, errInternal)
					return
				}
				_ = st.Close()
				return
			}
			if ctx.Err() != nil {
				// The session (or the subscriber) is going away: this is
				// not the track ending, so no clean FIN (draft §4.4.2).
				reset(st, errSessionClosed)
			} else {
				log.Warn(ctx, "moq: track failed", "error", err)
				reset(st, errInternal)
			}
			return
		}
		if g.Sequence < groupStart {
			continue
		}
		// A bounded subscription ends at its Group End, the first sequence
		// it does not want (draft §7.9); a track that has already passed
		// it owes nothing, not even an OK.
		if groupEnd != 0 && g.Sequence >= groupEnd {
			if werr := writeAll(st, msg(nil).varint(groupEnd).frame(subscribeEnd)); werr != nil {
				reset(st, errInternal)
				return
			}
			_ = st.Close()
			return
		}
		if first {
			if err := writeAll(st, msg(nil).varint(g.Sequence).frame(subscribeOK)); err != nil {
				reset(st, errInternal)
				return
			}
			first = false
		}
		next = g.Sequence + 1
		if err := s.sendGroup(ctx, id, g); err != nil {
			if ctx.Err() == nil {
				log.Warn(ctx, "moq: sending group failed", "group", g.Sequence, "error", err)
				reset(st, errInternal)
			} else {
				reset(st, errSessionClosed)
			}
			return
		}
		if groupEnd != 0 && next >= groupEnd {
			if werr := writeAll(st, msg(nil).varint(groupEnd).frame(subscribeEnd)); werr != nil {
				reset(st, errInternal)
				return
			}
			_ = st.Close()
			return
		}
	}
}

// sendGroup ships one group on a fresh unidirectional stream: STREAM_TYPE,
// GROUP, then each FRAME as a zigzag timestamp delta, a length, and the
// payload. The payload is written as-is rather than copied into the header.
func (s *Session) sendGroup(ctx context.Context, id uint64, g *Group) error {
	st, err := s.conn.OpenUniStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("opening group stream: %w", err)
	}
	hdr := msg(nil).varint(id).varint(g.Sequence).varint(0).frame(streamGroup)
	var prev int64
	for _, f := range g.Frames {
		if uint64(len(f.Payload)) > MaxFrame {
			st.CancelWrite(errInternal)
			return fmt.Errorf("frame of %d bytes exceeds MaxFrame", len(f.Payload))
		}
		hdr = appendVarint(hdr, zigzag(f.Timestamp-prev))
		hdr = appendVarint(hdr, uint64(len(f.Payload)))
		prev = f.Timestamp
		if err := writeAll(st, hdr); err != nil {
			st.CancelWrite(errInternal)
			return err
		}
		if err := writeAll(st, f.Payload); err != nil {
			st.CancelWrite(errInternal)
			return err
		}
		hdr = hdr[:0]
	}
	if len(hdr) > 0 {
		if err := writeAll(st, hdr); err != nil {
			st.CancelWrite(errInternal)
			return err
		}
	}
	return st.Close()
}

// serveTrack answers a TRACK request with the track's TRACK_INFO.
func (s *Session) serveTrack(br *bufio.Reader, st bidiStream) {
	m, err := readMessage(br, maxControlMessage)
	if err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("reading track: %w", err))
		return
	}
	b := body{b: m}
	broadcast, name := b.str(), b.str()
	if err := b.done(); err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("decoding track: %w", err))
		return
	}
	track, err := s.pub.Track(s.ctx, broadcast, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			reset(st, errNotFound)
		} else {
			reset(st, errInternal)
		}
		return
	}
	info := track.Info()
	track.Close()
	st.CancelRead(errCancelled)
	resp := msg(nil).u8(info.Priority).varint(uint64(info.MaxAge.Milliseconds())).varint(info.Timescale).frame()
	if err := writeAll(st, resp); err != nil {
		st.CancelWrite(errInternal)
		return
	}
	_ = st.Close()
}

// serveAnnounce advertises one route covering the whole requested prefix:
// a claim of capability, not inventory (draft §5.1.1). Whether a path
// under it names a broadcast is answered by SUBSCRIBE. The stream stays
// open, as the peer treats its end as every route going away.
func (s *Session) serveAnnounce(br *bufio.Reader, st bidiStream) {
	m, err := readMessage(br, maxControlMessage)
	if err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("reading announce request: %w", err))
		return
	}
	b := body{b: m}
	b.str() // prefix
	if err := b.done(); err != nil {
		s.fail(sessionProtocolViolation, fmt.Errorf("decoding announce request: %w", err))
		return
	}
	ok := msg(nil).varint(s.hop).varint(1).frame()
	start := msg(nil).str("").varint(0).varint(0).varint(0).frame(announceStart)
	if err := writeAll(st, append(ok, start...)); err != nil {
		st.CancelWrite(errInternal)
		return
	}
	_, _ = io.Copy(io.Discard, br)
	_ = st.Close()
}
