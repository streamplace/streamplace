package rtmps

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"stream.place/streamplace/pkg/log"
)

const (
	shadowTag = "duplicate-mist-test"
	// Hidden subcommand that runs the shadow parser; reads the raw client
	// byte stream on stdin.
	shadowWorkerCommand = "duplicate-mist-worker"

	// Upper bound on client bytes buffered for a worker that is not keeping
	// up (~10s of a 6Mbps stream). Overflow aborts the shadow, never Mist.
	defaultShadowRingSize = 8 << 20
	// How long a worker gets to finish and exit after the client stream ends.
	defaultShadowExitGrace = 30 * time.Second
	// Bounds Wait when a worker's grandchildren keep its output pipes open.
	shadowWaitDelay = 5 * time.Second

	shadowWriteChunk = 64 << 10
	shadowMaxLogLine = 64 << 10
)

type shadowOptions struct {
	// Worker command; defaults to this executable running duplicate-mist-worker.
	exe  string
	args []string
	// Defaults to defaultShadowRingSize.
	ringSize int
	// Defaults to defaultShadowExitGrace.
	exitGrace time.Duration
	// Called exactly once when the worker and its goroutines are gone, or
	// immediately if it never started.
	release func()
}

// shadow is a best-effort copy of the client's byte stream, fed to a worker
// subprocess. Nothing here may ever block or fail the primary connection:
// tee never blocks, and any problem aborts only the shadow.
type shadow struct {
	ctx       context.Context
	cancel    context.CancelFunc
	ring      *ring
	exitGrace time.Duration

	teed     atomic.Int64
	finished atomic.Bool
	aborted  atomic.Bool
	done     chan struct{}
}

// startShadow spawns the worker. It returns nil (after logging) if the worker
// can't be started; a nil *shadow is a valid no-op.
func startShadow(ctx context.Context, opts shadowOptions, connID uint64, remote string) *shadow {
	ctx = log.WithLogValues(ctx, "component", shadowTag, "conn_id", strconv.FormatUint(connID, 10), "remote", remote)
	release := opts.release
	if release == nil {
		release = func() {}
	}
	exe := opts.exe
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			log.Error(ctx, shadowTag+" failed: cannot locate executable for shadow worker, shadow disabled for this connection", "error", err)
			release()
			return nil
		}
	}
	args := opts.args
	if args == nil {
		args = []string{shadowWorkerCommand}
	}
	ringSize := opts.ringSize
	if ringSize == 0 {
		ringSize = defaultShadowRingSize
	}
	grace := opts.exitGrace
	if grace == 0 {
		grace = defaultShadowExitGrace
	}

	wctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(wctx, exe, args...)
	cmd.Env = workerEnv(os.Environ())
	cmd.WaitDelay = shadowWaitDelay
	stdout := &lineLogger{ctx: ctx, stream: "stdout"}
	stderr := &lineLogger{ctx: ctx, stream: "stderr"}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		log.Error(ctx, shadowTag+" failed: cannot create shadow worker stdin, shadow disabled for this connection", "error", err)
		release()
		return nil
	}
	if err := cmd.Start(); err != nil {
		cancel()
		log.Error(ctx, shadowTag+" failed: cannot start shadow worker, shadow disabled for this connection", "error", err)
		release()
		return nil
	}
	s := &shadow{
		ctx:       wctx,
		cancel:    cancel,
		ring:      newRing(ringSize),
		exitGrace: grace,
		done:      make(chan struct{}),
	}
	log.Log(ctx, shadowTag+": shadow worker started", "pid", cmd.Process.Pid)

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		s.pump(ctx, stdin)
	}()
	go func() {
		defer release()
		err := cmd.Wait()
		stdout.flush()
		stderr.flush()
		if err != nil || !s.finished.Load() {
			s.fail(ctx, "shadow worker exited unexpectedly", "error", err)
		}
		// Process is gone; unblock the pump if it is stuck in a write.
		s.cancel()
		<-writerDone
		close(s.done)
		log.Log(ctx, shadowTag+": shadow worker exited", "error", err, "aborted", s.aborted.Load(), "bytes_teed", s.teed.Load())
	}()
	return s
}

// pump moves buffered client bytes into the worker's stdin; a stall in the
// worker blocks only this goroutine, until fail() kills the worker.
func (s *shadow) pump(ctx context.Context, stdin io.WriteCloser) {
	defer stdin.Close()
	for {
		b, closed := s.ring.peek(shadowWriteChunk)
		if len(b) == 0 {
			if closed {
				return
			}
			select {
			case <-s.ring.wake:
			case <-s.ctx.Done():
				return
			}
			continue
		}
		if _, err := stdin.Write(b); err != nil {
			s.fail(ctx, "writing to shadow worker failed", "error", err)
			return
		}
		s.ring.advance(len(b))
	}
}

// tee offers client bytes to the worker without ever blocking. If the worker
// is too far behind, the whole shadow is aborted; the stream is never
// continued after a gap.
func (s *shadow) tee(ctx context.Context, p []byte) {
	if s == nil || s.aborted.Load() {
		return
	}
	if !s.ring.write(p) {
		s.fail(ctx, "shadow worker fell too far behind, aborting shadow only", "queue_bytes", len(s.ring.buf))
		return
	}
	s.teed.Add(int64(len(p)))
}

// finish marks the end of the client stream. The worker gets exitGrace to
// consume what is buffered and exit. Never blocks.
func (s *shadow) finish(ctx context.Context) {
	if s == nil || !s.finished.CompareAndSwap(false, true) {
		return
	}
	s.ring.close()
	time.AfterFunc(s.exitGrace, func() {
		select {
		case <-s.done:
		default:
			s.fail(ctx, "shadow worker did not exit after stream end, killing it", "grace", s.exitGrace.String())
		}
	})
}

// fail logs once and kills the worker. Only the shadow is affected.
func (s *shadow) fail(ctx context.Context, msg string, args ...any) {
	if ctx.Err() != nil {
		s.cancel()
		return
	}
	if s.aborted.CompareAndSwap(false, true) {
		log.Error(ctx, shadowTag+" failed: "+msg, append(args, "bytes_teed", s.teed.Load())...)
		s.cancel()
	}
}

// forwardClientToMist is io.Copy from the client to Mist that also tees every
// byte, handshake included, to sh (which may be nil).
func forwardClientToMist(ctx context.Context, mist io.Writer, client io.Reader, sh *shadow) (int64, error) {
	if sh == nil {
		return io.Copy(mist, client)
	}
	defer sh.finish(ctx)
	buf := make([]byte, 32<<10)
	var total int64
	for {
		n, rerr := client.Read(buf)
		if n > 0 {
			wn, werr := mist.Write(buf[:n])
			total += int64(wn)
			if werr == nil && wn != n {
				werr = io.ErrShortWrite
			}
			if werr != nil {
				return total, werr
			}
			sh.tee(ctx, buf[:n])
		}
		if rerr != nil {
			if rerr == io.EOF {
				rerr = nil
			}
			return total, rerr
		}
	}
}

// workerEnv drops the Streamplace configuration (SP_*) so the worker can't
// pick up production keys, storage or auth settings.
func workerEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "SP_") {
			out = append(out, kv)
		}
	}
	return out
}

// ring is a bounded byte FIFO: one writer that never blocks, one reader.
type ring struct {
	mu     sync.Mutex
	buf    []byte
	r, n   int
	closed bool
	wake   chan struct{}
}

func newRing(size int) *ring {
	return &ring{buf: make([]byte, size), wake: make(chan struct{}, 1)}
}

func (r *ring) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// write appends all of p, or nothing and false if it doesn't fit.
func (r *ring) write(p []byte) bool {
	r.mu.Lock()
	if len(p) > len(r.buf)-r.n {
		r.mu.Unlock()
		return false
	}
	w := (r.r + r.n) % len(r.buf)
	c := copy(r.buf[w:], p)
	copy(r.buf, p[c:])
	r.n += len(p)
	r.mu.Unlock()
	r.signal()
	return true
}

// peek returns up to max contiguous buffered bytes, valid until advance. The
// bool reports that the writer has closed the ring.
func (r *ring) peek(limit int) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := min(r.n, len(r.buf)-r.r, limit)
	return r.buf[r.r : r.r+n], r.closed
}

func (r *ring) advance(n int) {
	r.mu.Lock()
	r.r = (r.r + n) % len(r.buf)
	r.n -= n
	r.mu.Unlock()
}

func (r *ring) close() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.signal()
}

// lineLogger forwards each line of worker output to the log.
type lineLogger struct {
	ctx    context.Context
	stream string
	buf    []byte
}

func (l *lineLogger) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)
	consumed := 0
	for {
		i := bytes.IndexByte(l.buf[consumed:], '\n')
		if i < 0 {
			break
		}
		l.emit(l.buf[consumed : consumed+i])
		consumed += i + 1
	}
	l.buf = l.buf[:copy(l.buf, l.buf[consumed:])]
	if len(l.buf) > shadowMaxLogLine {
		l.emit(l.buf)
		l.buf = l.buf[:0]
	}
	return len(p), nil
}

func (l *lineLogger) flush() {
	l.emit(l.buf)
	l.buf = l.buf[:0]
}

func (l *lineLogger) emit(line []byte) {
	line = bytes.TrimRight(line, "\r")
	if len(line) == 0 {
		return
	}
	log.Log(l.ctx, shadowTag+": "+string(line), "stream", l.stream)
}
