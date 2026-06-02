package media

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"

	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/log"
)

// FrameWriter is the worker's segment sink. Stage 1 uses a direct framed pipe
// (*ingestframe.Writer); the zero-downtime path uses *frameServer, which buffers
// across a disconnected main and replays on reconnect. Structurally satisfied by
// *ingestframe.Writer, so RunMKVIngestWorker is agnostic to which it gets.
type FrameWriter interface {
	Segment(seg []byte) error
	End() error
	Error(msg string) error
}

type bufferedFrame struct {
	typ     ingestframe.Type
	payload []byte
}

// frameServer delivers worker frames to the main process over a reconnectable
// transport (a per-session unix socket), buffering — bounded, drop-oldest —
// whenever no client is attached. This is the heart of the zero-downtime upgrade
// path: main disconnects for a restart, the worker keeps signing segments into
// the buffer, and the reconnecting main drains the buffer before going live, so
// segments produced during a brief restart are not lost. Drops are bounded and
// counted (a main outage longer than the buffer window loses the oldest tail,
// loudly, rather than growing without limit).
//
// Safe for concurrent push (the worker) vs attach/detach (the socket accept
// loop). A push to an attached-but-dead client fails the write, auto-detaches,
// and re-buffers that frame, so a hard main disconnect degrades to buffering.
type frameServer struct {
	mu      sync.Mutex
	pending []bufferedFrame
	conn    net.Conn
	w       *ingestframe.Writer
	maxBuf  int
	dropped int
}

// newFrameServer creates a server that buffers up to maxBuf frames while no
// client is attached.
func newFrameServer(maxBuf int) *frameServer {
	return &frameServer{maxBuf: maxBuf}
}

func (s *frameServer) push(typ ingestframe.Type, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.w != nil {
		if err := s.w.WriteFrame(typ, payload); err == nil {
			return
		}
		// Client gone; drop it and buffer this frame instead.
		s.conn, s.w = nil, nil
	}
	s.pending = append(s.pending, bufferedFrame{typ, bytes.Clone(payload)})
	for len(s.pending) > s.maxBuf {
		s.pending = s.pending[1:]
		s.dropped++
	}
}

func (s *frameServer) Segment(seg []byte) error { s.push(ingestframe.Segment, seg); return nil }
func (s *frameServer) End() error               { s.push(ingestframe.End, nil); return nil }
func (s *frameServer) Error(msg string) error   { s.push(ingestframe.Error, []byte(msg)); return nil }

// dropped reports how many buffered frames were discarded because the buffer
// overflowed (main was disconnected longer than the buffer window).
func (s *frameServer) droppedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// attach binds a freshly-connected client, replaying buffered frames in order
// before going live. On a replay error the client is dropped and the buffer kept
// intact for the next reconnect.
func (s *frameServer) attach(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := ingestframe.NewWriter(conn)
	for _, f := range s.pending {
		if err := w.WriteFrame(f.typ, f.payload); err != nil {
			return
		}
	}
	s.pending = nil
	s.conn, s.w = conn, w
}

// detachConn drops the named client if it's still the current one (a stale
// connection's teardown must not clobber a newer one that already reattached).
// The buffer and drop count are preserved.
func (s *frameServer) detachConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == conn {
		s.conn, s.w = nil, nil
	}
}

// serveFrameSocket accepts client connections on ln and attaches each to the
// server, replacing any prior client (main reconnecting after a restart). Each
// connection is watched for close so the server reverts to buffering. Returns
// when ctx is cancelled or the listener is closed.
func serveFrameSocket(ctx context.Context, ln net.Listener, s *frameServer) {
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		log.Log(ctx, "ingest worker: main attached to frame socket")
		s.attach(conn)
		go func(c net.Conn) {
			// Main only reads frames; this drains anything it sends (nothing today)
			// and unblocks when it disconnects, at which point we revert to buffering.
			_, _ = io.Copy(io.Discard, c)
			s.detachConn(c)
			log.Log(ctx, "ingest worker: main detached from frame socket")
		}(conn)
	}
}
