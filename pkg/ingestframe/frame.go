// Package ingestframe defines the wire protocol a per-stream ingest worker uses
// to stream canonical MUXL fragments back to the main streamplace process.
//
// Each incoming live stream is handled by an isolated worker subprocess that
// owns the socket, muxes + transcodes the media, and signs each GoP. It emits
// the resulting signed canonical .m4s segments to the main process as a sequence
// of typed, length-prefixed frames.
//
// The framing is deliberately transport-agnostic: today it rides the worker's
// stdout pipe, but a detached / reattachable worker (the zero-downtime-upgrade
// path, where workers keep buffering signed segments across a main restart) can
// carry the identical frames over a unix socket. Nothing above this package
// cares which.
package ingestframe

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

// Type identifies a frame's payload.
type Type uint8

const (
	// Segment carries one signed canonical .m4s segment — the unit main's
	// ValidateMP4 ingests. Payload: the bare canonical segment bytes.
	Segment Type = 1
	// End signals the worker finished the stream cleanly (graceful EOS). No
	// payload. Its ABSENCE before EOF is how main tells a crash from a clean end.
	End Type = 2
	// Error carries a worker-side fatal error message (UTF-8). The worker emits
	// it just before exiting so main can log a cause, not a bare "worker exited".
	Error Type = 3
	// Answer carries an SDP answer (UTF-8). The WHIP worker owns the
	// PeerConnection, so it generates the answer and emits it as the FIRST frame
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

// magic prefixes every frame so a desynced/corrupt stream is caught immediately
// rather than mis-parsed as a length.
var magic = [4]byte{'S', 'P', 'F', '1'}

// MaxPayload bounds a single frame so a corrupt or hostile length can't make the
// reader allocate unboundedly. Canonical GoP segments are well under this.
const MaxPayload = 64 << 20 // 64 MiB

const headerSize = 4 + 1 + 4 // magic + type + uint32 length

// Writer serializes frames to an underlying stream. Safe for concurrent use: a
// worker emits segments from more than one goroutine (the source signer and the
// transcoder's completion callback), and frames must never interleave.
type Writer struct {
	mu sync.Mutex
	w  io.Writer
}

// NewWriter wraps w. w is typically the worker's os.Stdout.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteFrame writes one whole frame atomically with respect to other WriteFrame
// calls on the same Writer.
func (fw *Writer) WriteFrame(t Type, payload []byte) error {
	if len(payload) > MaxPayload {
		return fmt.Errorf("ingestframe: payload %d exceeds max %d", len(payload), MaxPayload)
	}
	var hdr [headerSize]byte
	copy(hdr[0:4], magic[:])
	hdr[4] = byte(t)
	binary.BigEndian.PutUint32(hdr[5:9], uint32(len(payload)))

	fw.mu.Lock()
	defer fw.mu.Unlock()
	if _, err := fw.w.Write(hdr[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := fw.w.Write(payload); err != nil {
			return err
		}
	}
	return nil
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

// Reader decodes frames from an underlying stream.
type Reader struct {
	r io.Reader
}

// NewReader wraps r, typically the worker's stdout pipe.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// ReadFrame decodes the next frame. It returns io.EOF only at a clean frame
// boundary (the stream ended between frames); a stream that dies mid-frame
// surfaces as io.ErrUnexpectedEOF, so an abrupt worker death is distinguishable
// from a clean close.
func (fr *Reader) ReadFrame() (Type, []byte, error) {
	var hdr [headerSize]byte
	if _, err := io.ReadFull(fr.r, hdr[:]); err != nil {
		// io.EOF here = clean boundary. io.ReadFull maps a partial read to
		// ErrUnexpectedEOF, which we keep: a torn header is an abrupt death.
		return 0, nil, err
	}
	if [4]byte(hdr[0:4]) != magic {
		return 0, nil, fmt.Errorf("ingestframe: bad magic %q (stream desynced)", hdr[0:4])
	}
	t := Type(hdr[4])
	n := binary.BigEndian.Uint32(hdr[5:9])
	if n > MaxPayload {
		return 0, nil, fmt.Errorf("ingestframe: frame length %d exceeds max %d", n, MaxPayload)
	}
	if n == 0 {
		return t, nil, nil
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(fr.r, payload); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return 0, nil, err
	}
	return t, payload, nil
}
