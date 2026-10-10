// Package moq implements the moq-lite-06 transport
// (https://datatracker.ietf.org/doc/html/draft-lcurley-moq-lite-06) over raw
// QUIC and WebTransport: the publisher role for serving tracks, and the
// subscriber role for pulling them. The payload is opaque here; Streamplace
// carries MUXL segments (see docs/moq.md).
//
// The subset is what a live media origin needs: SUBSCRIBE and TRACK are
// served, ANNOUNCE advertises a single covering route, FETCH and PROBE are
// refused with a stream reset, and datagrams are never sent.
package moq

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ALPN is the moq-lite version this package speaks, negotiated as the QUIC
// ALPN on a raw connection and as the WebTransport subprotocol otherwise.
const ALPN = "moq-lite-06"

// Bidirectional (control) stream types: the first varint on a stream.
const (
	streamAnnounce  = 0x1
	streamSubscribe = 0x2
	streamFetch     = 0x3
	streamProbe     = 0x4
	streamGoaway    = 0x5
	streamTrack     = 0x6
)

// Unidirectional (data) stream types.
const (
	streamGroup = 0x0
	streamSetup = 0x1
)

// SETUP parameter ids.
const (
	setupParamPath = 0x2
	setupParamRole = 0x3
)

const roleSubscriber = 2

// Subscribe stream response types (publisher → subscriber).
const (
	subscribeOK   = 0x0
	subscribeEnd  = 0x1
	subscribeDrop = 0x2
)

// Announce stream message types (publisher → subscriber).
const announceStart = 0x0

// Session error codes (CONNECTION_CLOSE).
const (
	sessionNoError           = 0x0
	sessionProtocolViolation = 0x3
)

// Stream error codes (RESET_STREAM / STOP_SENDING).
const (
	errInternal       = 0x0
	errCancelled      = 0x1
	errSessionClosed  = 0x3
	errMalformedTrack = 0x12
	errNotFound       = 0x33
	errUnroutable     = 0x36
)

// Wire-size limits. Every size on the wire is a varint chosen by the peer, so
// each read is capped before anything is allocated.
const (
	maxControlMessage = 64 * 1024
	maxString         = 4096
	// MaxFrame bounds a single frame payload. A MUXL GoP at any sane
	// bitrate is a few megabytes; this is the reference implementation's
	// cap as well.
	MaxFrame = 64 * 1024 * 1024
)

var errMessageTooLarge = errors.New("moq: message exceeds size limit")

// --- varints ---------------------------------------------------------------

const maxVarint = 1<<62 - 1

// appendVarint appends v as a QUIC variable-length integer (RFC 9000 §16).
func appendVarint(b []byte, v uint64) []byte {
	switch {
	case v < 1<<6:
		return append(b, byte(v))
	case v < 1<<14:
		return append(b, byte(v>>8)|0x40, byte(v))
	case v < 1<<30:
		return append(b, byte(v>>24)|0x80, byte(v>>16), byte(v>>8), byte(v))
	default:
		if v > maxVarint {
			panic("moq: varint out of range")
		}
		return binary.BigEndian.AppendUint64(b, v|0xc0<<56)
	}
}

// readVarint reads one QUIC varint.
func readVarint(r io.ByteReader) (uint64, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	n := 1 << (first >> 6)
	v := uint64(first & 0x3f)
	for i := 1; i < n; i++ {
		b, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		v = v<<8 | uint64(b)
	}
	return v, nil
}

// zigzag maps a signed timestamp delta onto the unsigned varint space so
// small magnitudes of either sign stay short (draft §7.20).
func zigzag(v int64) uint64 { return uint64(v<<1) ^ uint64(v>>63) }

func unzigzag(v uint64) int64 { return int64(v>>1) ^ -int64(v&1) }

// --- message building -------------------------------------------------------

// msg accumulates one length-prefixed message body.
type msg []byte

func (m msg) varint(v uint64) msg { return appendVarint(m, v) }
func (m msg) u8(v uint8) msg      { return append(m, v) }
func (m msg) str(s string) msg    { return append(appendVarint(m, uint64(len(s))), s...) }

// frame prefixes the body with its length, optionally after a leading type
// varint (the subscribe/announce response discriminator sits outside the
// length prefix).
func (m msg) frame(typ ...uint64) []byte {
	var out []byte
	for _, t := range typ {
		out = appendVarint(out, t)
	}
	out = appendVarint(out, uint64(len(m)))
	return append(out, m...)
}

// --- message reading --------------------------------------------------------

// readMessage reads one length-prefixed message body, refusing one larger
// than max before allocating it.
func readMessage(r *bufio.Reader, max uint64) ([]byte, error) {
	size, err := readVarint(r)
	if err != nil {
		return nil, err
	}
	if size > max {
		return nil, fmt.Errorf("%w: %d > %d", errMessageTooLarge, size, max)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return body, nil
}

// body decodes fields out of a message that has been read in full.
type body struct {
	b   []byte
	err error
}

func (p *body) varint() uint64 {
	if p.err != nil {
		return 0
	}
	if len(p.b) == 0 {
		p.err = io.ErrUnexpectedEOF
		return 0
	}
	n := 1 << (p.b[0] >> 6)
	if len(p.b) < n {
		p.err = io.ErrUnexpectedEOF
		return 0
	}
	v := uint64(p.b[0] & 0x3f)
	for i := 1; i < n; i++ {
		v = v<<8 | uint64(p.b[i])
	}
	p.b = p.b[n:]
	return v
}

func (p *body) u8() uint8 {
	if p.err != nil {
		return 0
	}
	if len(p.b) == 0 {
		p.err = io.ErrUnexpectedEOF
		return 0
	}
	v := p.b[0]
	p.b = p.b[1:]
	return v
}

func (p *body) str() string {
	n := p.varint()
	if p.err != nil {
		return ""
	}
	if n > maxString || n > uint64(len(p.b)) {
		p.err = fmt.Errorf("moq: string of %d bytes exceeds message", n)
		return ""
	}
	s := string(p.b[:n])
	p.b = p.b[n:]
	return s
}

// done reports a decode error, or leftover bytes: a message must be consumed
// exactly (draft §7.1).
func (p *body) done() error {
	if p.err != nil {
		return p.err
	}
	if len(p.b) != 0 {
		return fmt.Errorf("moq: %d trailing bytes in message", len(p.b))
	}
	return nil
}
