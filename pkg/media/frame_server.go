package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/log"
)

// workerFrameBuffer bounds how many frames a worker holds while main is
// disconnected. LL-HLS parts and metadata consume entries alongside signed
// segments, so this is a frame bound rather than a fixed time guarantee.
const workerFrameBuffer = 600

const workerFrameWriteTimeout = 100 * time.Millisecond

// workerDrainGrace bounds how long a worker lingers after its stream ends waiting
// for main to drain the buffer. Generous enough for a main restart/upgrade; an
// orphaned worker (main never returns) exits after it.
const workerDrainGrace = 60 * time.Second

// FrameWriter is the worker's segment sink. Stage 1 uses a direct framed pipe
// (*ingestframe.Writer); the zero-downtime path uses *frameServer, which buffers
// across a disconnected main and replays on reconnect. Structurally satisfied by
// *ingestframe.Writer, so RunMP4IngestWorker is agnostic to which it gets.
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
	mu           sync.Mutex
	llInits      []bufferedFrame
	llInitTracks map[string]int
	pending      []bufferedFrame
	conn         net.Conn
	w            *ingestframe.Writer
	maxBuf       int
	dropped      int
	everAttached bool
}

// newFrameServer creates a server that buffers up to maxBuf frames while no
// client is attached.
func newFrameServer(maxBuf int) *frameServer {
	return &frameServer{maxBuf: maxBuf, llInitTracks: make(map[string]int)}
}

func (s *frameServer) push(typ ingestframe.Type, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rememberedInit := false
	if typ == ingestframe.LLInit {
		if frame, err := ingestframe.DecodeLLFrame(payload); err == nil && frame.Track != "" {
			if s.llInitTracks == nil {
				s.llInitTracks = make(map[string]int)
			}
			buffered := bufferedFrame{typ, bytes.Clone(payload)}
			if index, ok := s.llInitTracks[frame.Track]; ok {
				s.llInits[index] = buffered
			} else {
				s.llInitTracks[frame.Track] = len(s.llInits)
				s.llInits = append(s.llInits, buffered)
			}
			rememberedInit = true
		}
	}
	if s.w != nil {
		if err := writeWorkerFrame(s.conn, s.w, typ, payload); err == nil {
			return
		}
		// Client gone; drop it and buffer this frame instead.
		conn := s.conn
		s.conn, s.w = nil, nil
		if conn != nil {
			_ = conn.Close()
		}
	}
	if typ != ingestframe.LLInit || !rememberedInit {
		s.pending = append(s.pending, bufferedFrame{typ, bytes.Clone(payload)})
	}
	for len(s.pending) > s.maxBuf {
		s.pending = s.pending[1:]
		s.dropped++
	}
}

func writeWorkerFrame(conn net.Conn, writer *ingestframe.Writer, typ ingestframe.Type, payload []byte) error {
	if conn == nil || writer == nil {
		return fmt.Errorf("missing frame connection")
	}
	if err := conn.SetWriteDeadline(time.Now().Add(workerFrameWriteTimeout)); err != nil {
		return err
	}
	defer conn.SetWriteDeadline(time.Time{})
	return writer.WriteFrame(typ, payload)
}

func (s *frameServer) Segment(seg []byte) error { s.push(ingestframe.Segment, seg); return nil }
func (s *frameServer) End() error               { s.push(ingestframe.End, nil); return nil }
func (s *frameServer) Error(msg string) error   { s.push(ingestframe.Error, []byte(msg)); return nil }
func (s *frameServer) Answer(sdp string) error  { s.push(ingestframe.Answer, []byte(sdp)); return nil }

func (s *frameServer) writeLL(typ ingestframe.Type, frame ingestframe.LLFrame) error {
	payload, err := ingestframe.EncodeLLFrame(frame)
	if err != nil {
		return err
	}
	s.push(typ, payload)
	return nil
}

func (s *frameServer) LLInit(frame ingestframe.LLFrame) error {
	return s.writeLL(ingestframe.LLInit, frame)
}

func (s *frameServer) LLPart(frame ingestframe.LLFrame) error {
	return s.writeLL(ingestframe.LLPart, frame)
}

func (s *frameServer) LLSegmentComplete(frame ingestframe.LLFrame) error {
	return s.writeLL(ingestframe.LLSegmentComplete, frame)
}

func (s *frameServer) LLDiscontinuity(frame ingestframe.LLFrame) error {
	return s.writeLL(ingestframe.LLDiscontinuity, frame)
}

func (s *frameServer) LLSessionEnd(frame ingestframe.LLFrame) error {
	return s.writeLL(ingestframe.LLSessionEnd, frame)
}

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
	for _, f := range s.llInits {
		if err := writeWorkerFrame(conn, w, f.typ, f.payload); err != nil {
			_ = conn.Close()
			return
		}
	}
	for _, f := range s.pending {
		if err := writeWorkerFrame(conn, w, f.typ, f.payload); err != nil {
			_ = conn.Close()
			return
		}
	}
	s.pending = nil
	s.conn, s.w = conn, w
	s.everAttached = true
}

// waitDrained blocks until a client has attached and the buffer is fully written
// out to it (so main has the whole stream, including the trailing End), or until
// ctx is cancelled or grace elapses. The worker calls this after the stream ends
// so it doesn't exit — discarding the in-memory buffer — before a reconnecting
// main has drained it. grace bounds an orphaned worker whose main never returns.
func (s *frameServer) waitDrained(ctx context.Context, grace time.Duration) {
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		s.mu.Lock()
		drained := s.everAttached && len(s.pending) == 0
		s.mu.Unlock()
		if drained {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			s.mu.Lock()
			pending := len(s.pending)
			s.mu.Unlock()
			log.Warn(ctx, "ingest worker: main never drained the frame buffer; exiting", "pending", pending)
			return
		case <-tick.C:
		}
	}
}

// closeConn closes the current client connection, giving main a clean EOF after
// the trailing End — the signal that the worker is done and won't reconnect.
func (s *frameServer) closeConn() {
	s.mu.Lock()
	c := s.conn
	s.conn, s.w = nil, nil
	s.mu.Unlock()
	if c != nil {
		c.Close()
	}
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

// ServeMP4IngestWorkerSocket runs the ingest worker, delivering its signed
// segments to main over a per-session unix socket at cfg.SocketPath with
// buffered reconnect — the zero-downtime path. It listens, serves the frame
// stream (buffering across any main disconnect), runs the ingest, frames a
// trailing End/Error, then lingers until main has drained the buffer before
// removing the socket and returning.
func ServeMP4IngestWorkerSocket(ctx context.Context, cfg IngestWorkerConfig, stdin io.Reader) error {
	if cfg.SocketPath == "" {
		return fmt.Errorf("ServeMP4IngestWorkerSocket: empty socket path")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_ = os.Remove(cfg.SocketPath) // clear any stale socket from a prior worker
	ln, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.SocketPath, err)
	}
	defer func() {
		ln.Close()
		removeWorkerFiles(cfg.SocketPath)
	}()

	srv := newFrameServer(workerFrameBuffer)
	manifest := newManifestHolder(cfg.Manifest)
	go serveFrameSocket(ctx, ln, srv, manifest)

	runErr := RunMP4IngestWorker(ctx, cfg, stdin, srv, manifest.get)
	if runErr != nil {
		_ = srv.Error(runErr.Error())
	} else {
		_ = srv.End()
	}

	// Don't exit (and drop the in-memory buffer) until main has the whole stream,
	// including the trailing End — so a brief main restart loses nothing.
	srv.waitDrained(ctx, workerDrainGrace)
	// Close the connection so main reads End then a clean EOF (worker is done).
	srv.closeConn()
	return runErr
}

// serveFrameSocket accepts client connections on ln and attaches each to the
// server, replacing any prior client (main reconnecting after a restart). The
// reverse direction carries control frames from main: it reads them and applies
// a Manifest update to the holder; any other frame is ignored. The read also
// unblocks when main disconnects, at which point the server reverts to
// buffering. Returns when ctx is cancelled or the listener is closed.
func serveFrameSocket(ctx context.Context, ln net.Listener, s *frameServer, manifest *manifestHolder) {
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
			fr := ingestframe.NewReader(c)
			for {
				typ, payload, rerr := fr.ReadFrame()
				if rerr != nil {
					break // main disconnected (or sent garbage); revert to buffering
				}
				if typ == ingestframe.Manifest {
					manifest.set(payload)
					log.Log(ctx, "ingest worker: manifest updated by main", "bytes", len(payload))
				}
			}
			s.detachConn(c)
			log.Log(ctx, "ingest worker: main detached from frame socket")
		}(conn)
	}
}
