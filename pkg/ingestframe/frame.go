// Package ingestframe defines the wire protocol a per-stream ingest worker uses
// to stream canonical MUXL fragments (and status) back to the main streamplace
// process.
//
// Each incoming live stream is handled by an isolated worker subprocess that
// owns the socket, muxes + transcodes + signs the media, and emits the resulting
// signed canonical .m4s segments to the main process as a sequence of typed
// messages. The same channel carries control messages (a clean end, a fatal
// error, a WHIP SDP answer, a status event).
//
// The wire format is a stream of concatenated DRISL CBOR items — the same codec
// muxl uses for its own stdio protocol. CBOR data items are self-delimiting (the
// length/count lives in each item's head), so no separate length prefix is
// needed, and a decoder reads exactly one item per call. Crucially this PRESERVES
// the crash-vs-clean-end signal the supervisor relies on: the decoder returns
// io.EOF at an item boundary (the stream ended cleanly between messages) and
// io.ErrUnexpectedEOF mid-item (a worker that died). A garbage/desynced stream
// fails to decode rather than being mis-parsed.
//
// The format is transport-agnostic: today it rides the worker's stdout pipe or a
// per-session unix socket (the zero-downtime detach/reattach path, where workers
// keep buffering signed segments across a main restart). Nothing above this
// package cares which.
package ingestframe

import (
	"fmt"
	"io"
	"sync"

	"github.com/hyphacoop/go-dasl/drisl"
)

// Type identifies a message's payload.
type Type uint8

const (
	// Segment carries one signed canonical .m4s segment — the unit main's
	// ValidateMP4 ingests. Payload: the bare canonical segment bytes.
	Segment Type = 1
	// End signals the worker finished the stream cleanly (graceful EOS). No
	// payload. It's the in-band "done" marker; its absence before EOF (together
	// with the worker's exit code) is how main tells a crash from a clean end.
	End Type = 2
	// Error carries a worker-side fatal error message (UTF-8). The worker emits
	// it just before exiting so main can log a cause, not a bare "worker exited".
	Error Type = 3
	// Answer carries an SDP answer (UTF-8). The WHIP worker owns the
	// PeerConnection, so it generates the answer and emits it as the FIRST message
	// on the socket; main reads it and returns it to the WHIP client before
	// consuming segments. Payload: the answer SDP.
	Answer Type = 4
	// Event carries a worker status update (UTF-8 JSON) on the reverse channel.
	// The RTMP push worker uses it to report multistream status (e.g. "active"
	// with bytes acked) back to main, which writes it to the DB — the worker has
	// no DB access of its own. Payload: JSON {status, message}.
	Event Type = 5
)

func (t Type) String() string {
	switch t {
	case Segment:
		return "segment"
	case End:
		return "end"
	case Error:
		return "error"
	case Answer:
		return "answer"
	case Event:
		return "event"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(t))
	}
}

// message is the on-wire DRISL CBOR item: one self-delimiting map per frame.
// Payload is a CBOR byte string (raw for Segment, UTF-8/JSON for the rest) and is
// omitted entirely for an empty body (e.g. End), so a bodyless frame is just
// {"type": N}.
type message struct {
	Type    Type   `cbor:"type"`
	Payload []byte `cbor:"payload,omitempty"`
}

// frameDecoder is the streaming-decode surface we need (satisfied by drisl's
// *cbor.Decoder). Kept as an interface so this package needn't import the cbor
// module directly.
type frameDecoder interface {
	Decode(v any) error
}

// Writer serializes frames to an underlying stream. Safe for concurrent use: a
// worker emits segments from more than one goroutine (the source signer and the
// transcoder's completion callback), and frames must never interleave.
type Writer struct {
	mu sync.Mutex
	w  io.Writer
}

// NewWriter wraps w. w is typically the worker's frame fd or a socket conn.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteFrame writes one whole frame atomically with respect to other WriteFrame
// calls on the same Writer. The CBOR item is encoded up front, then written under
// the lock, so concurrent writers never interleave a frame's bytes.
func (fw *Writer) WriteFrame(t Type, payload []byte) error {
	b, err := drisl.Marshal(message{Type: t, Payload: payload})
	if err != nil {
		return fmt.Errorf("ingestframe: encode %s: %w", t, err)
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	_, err = fw.w.Write(b)
	return err
}

// Segment frames a signed canonical .m4s segment.
func (fw *Writer) Segment(seg []byte) error { return fw.WriteFrame(Segment, seg) }

// End frames a clean end-of-stream marker.
func (fw *Writer) End() error { return fw.WriteFrame(End, nil) }

// Error frames a fatal worker-side error message.
func (fw *Writer) Error(msg string) error { return fw.WriteFrame(Error, []byte(msg)) }

// Answer frames the WHIP SDP answer (emitted first, before any segments).
func (fw *Writer) Answer(sdp string) error { return fw.WriteFrame(Answer, []byte(sdp)) }

// Event frames a worker status update (JSON payload).
func (fw *Writer) Event(payload []byte) error { return fw.WriteFrame(Event, payload) }

// Reader decodes frames from an underlying stream. The decoder buffers/reads
// ahead, so a Reader OWNS its stream for the stream's lifetime — don't create a
// second Reader on the same connection (it would lose the first's buffered
// read-ahead).
type Reader struct {
	dec frameDecoder
}

// NewReader wraps r, typically the worker's frame fd or a socket conn.
func NewReader(r io.Reader) *Reader {
	return &Reader{dec: drisl.NewDecoder(r)}
}

// ReadFrame decodes the next frame. It returns io.EOF only at a clean item
// boundary (the stream ended between frames); a stream that dies mid-item
// surfaces as io.ErrUnexpectedEOF, so an abrupt worker death is distinguishable
// from a clean close. A malformed/desynced item surfaces as a decode error.
func (fr *Reader) ReadFrame() (Type, []byte, error) {
	var m message
	if err := fr.dec.Decode(&m); err != nil {
		return 0, nil, err
	}
	return m.Type, m.Payload, nil
}
