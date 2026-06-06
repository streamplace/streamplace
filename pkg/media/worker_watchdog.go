package media

import (
	"context"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
)

// workerWatchdog self-terminates a wedged ingest worker. A healthy stream emits
// an output frame every GoP (~1–2s); going ingestWorkerWatchdog with no frame
// means the native pipeline is wedged (gst can't drain — e.g. a pathological
// stream that leaves demux pads unlinked) and won't recover. The watchdog then
// fires onWedge — the worker's context cancel — tearing the pipeline down so the
// process exits and the fault stays contained to this subprocess.
//
// This is the worker-side counterpart to MKVIngestIsolated's main-side watchdog,
// and the ONLY wedge containment on the detached and WHIP paths: those workers
// are detached (not tied to main's context), so main can't kill a stuck one —
// the worker has to notice and exit itself.
type workerWatchdog struct {
	mu    sync.Mutex
	timer *time.Timer
	to    time.Duration
}

func newWorkerWatchdog(ctx context.Context, timeout time.Duration, onWedge func(), streamer string) *workerWatchdog {
	return &workerWatchdog{
		to: timeout,
		timer: time.AfterFunc(timeout, func() {
			log.Warn(ctx, "ingest worker watchdog fired (no output frames); self-terminating",
				"streamer", streamer, "timeout", timeout)
			onWedge()
		}),
	}
}

// kick resets the no-output timer. Safe for concurrent use — a worker emits
// frames from more than one goroutine (the signer event loop and the
// transcoder's completion callback).
func (w *workerWatchdog) kick() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.timer.Reset(w.to)
}

// stop halts the watchdog; call it once the pipeline has ended so it can't fire
// during the post-stream drain.
func (w *workerWatchdog) stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.timer.Stop()
}

// wrap returns a FrameWriter that kicks the watchdog on every frame it writes,
// so any output the worker produces counts as liveness.
func (w *workerWatchdog) wrap(inner FrameWriter) FrameWriter {
	return watchdogFrameWriter{FrameWriter: inner, wd: w}
}

type watchdogFrameWriter struct {
	FrameWriter
	wd *workerWatchdog
}

func (w watchdogFrameWriter) Segment(seg []byte) error {
	w.wd.kick()
	return w.FrameWriter.Segment(seg)
}
func (w watchdogFrameWriter) End() error             { w.wd.kick(); return w.FrameWriter.End() }
func (w watchdogFrameWriter) Error(msg string) error { w.wd.kick(); return w.FrameWriter.Error(msg) }
