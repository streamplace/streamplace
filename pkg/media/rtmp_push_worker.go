package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// RTMPPushWorkerConfig is the startup handshake main hands an rtmp-push worker
// over a dedicated pipe fd. TargetURL embeds the destination stream key, so it
// rides fd 3 and is kept off argv/env — only the (public) streamer DID goes on
// the command line, for process-listing identification.
type RTMPPushWorkerConfig struct {
	StreamerDID string `json:"streamer_did"`
	// TargetURL is the rtmp(s):// destination including its stream key. Sensitive
	// — fd-3 only.
	TargetURL string `json:"target_url"`
}

// pushEvent is the worker→main status payload carried in an ingestframe.Event.
// Main writes it verbatim as a multistream event against the target's AT-URI
// (the URI lives main-side; the worker only knows status semantics).
type pushEvent struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// RunRTMPPushWorker is the body of the `rtmp-push-worker` subcommand. It reads
// the assembled fMP4 source stream from `source` (the worker's stdin, fed by
// main) and runs the native RTMP egress pipeline, reporting status back to main
// as Event frames instead of writing the DB directly (the worker has no DB).
// Returns when the pipeline ends (source EOS) or errors; the caller frames
// End/Error accordingly.
func RunRTMPPushWorker(ctx context.Context, cfg RTMPPushWorkerConfig, source io.Reader, events *ingestframe.Writer) error {
	gstinit.InitGST()
	// Minimal manager: the push pipeline + forwarder need no model/DB/bus, only a
	// CLI shell.
	mm := &MediaManager{cli: &config.CLI{}}
	report := func(status, message string) {
		payload, err := json.Marshal(pushEvent{Status: status, Message: message})
		if err != nil {
			log.Error(ctx, "rtmp push worker: marshal event", "error", err)
			return
		}
		if err := events.Event(payload); err != nil {
			log.Error(ctx, "rtmp push worker: emit event", "error", err)
		}
	}
	return mm.runRTMPPushPipeline(ctx, source, cfg.TargetURL, report)
}

// RTMPPushIsolated is the process-isolated counterpart to RTMPPush: the
// crash-prone native egress pipeline (qtdemux/flvmux/rtmp2sink) runs in a
// dedicated `rtmp-push-worker` subprocess, so a gst fault there kills only the
// worker — the node survives. The bus subscription, segment assembly, and DB
// status writes stay here in main; only the native pipeline moves.
//
// Lifecycle maps onto the existing multistream control loop: the worker runs
// under exec.CommandContext(ctx), so the cancel HandleMultistreamTargets fires
// when a target is disabled tears the worker down, and a crash surfaces as a
// non-zero exit that StartMultistreamTarget turns into an "error" event + retry.
func (mm *MediaManager) RTMPPushIsolated(ctx context.Context, user string, rendition string, targetView *placestream.MultistreamDefs_TargetView) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithLogValues(ctx, "mediafunc", "RTMPPushIsolated")
	rec, ok := targetView.Record.Val.(*placestream.MultistreamTarget)
	if !ok {
		return fmt.Errorf("failed to convert target view to multistream target")
	}

	cfgJSON, err := json.Marshal(RTMPPushWorkerConfig{StreamerDID: user, TargetURL: rec.Url})
	if err != nil {
		return fmt.Errorf("marshal push worker config: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	// The streamer DID rides argv (public) so a worker is identifiable in a
	// process listing; the target URL (with its key) stays on fd 3.
	cmd := exec.CommandContext(ctx, exe, "rtmp-push-worker", user)

	// fd 3 carries the config in; fd 4 carries the status-event stream out. Off
	// stdout so stray gst/log noise can't corrupt the frames.
	cfgR, cfgW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("config pipe: %w", err)
	}
	defer cfgW.Close()
	eventsR, eventsW, err := os.Pipe()
	if err != nil {
		cfgR.Close()
		return fmt.Errorf("events pipe: %w", err)
	}
	defer eventsR.Close()
	cmd.ExtraFiles = []*os.File{cfgR, eventsW} // → child fd 3, fd 4

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cfgR.Close()
		eventsW.Close()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cfgR.Close()
		eventsW.Close()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cfgR.Close()
		eventsW.Close()
		return err
	}

	if err := cmd.Start(); err != nil {
		cfgR.Close()
		eventsW.Close()
		return fmt.Errorf("start rtmp push worker: %w", err)
	}
	cfgR.Close()    // the child holds its own copy now
	eventsW.Close() // ditto; the parent only reads eventsR

	go func() {
		_, _ = cfgW.Write(cfgJSON)
		cfgW.Close() // EOF so the worker's config read completes
	}()

	// Feed the worker the continuous fMP4 source stream (bus → AAC select →
	// init+concat). Closing stdin on source EOF/cancel is the worker's EOS.
	go func() {
		defer stdin.Close()
		if serr := mm.writeRTMPSource(ctx, user, rendition, stdin); serr != nil && ctx.Err() == nil {
			log.Error(ctx, "rtmp push source ended", "error", serr)
		}
	}()

	// Forward stdout + stderr to the node logger. Drain both fully before Wait.
	var logsWG sync.WaitGroup
	logsWG.Add(2)
	go func() { defer logsWG.Done(); streamWorkerLogs(ctx, stdout, user) }()
	go func() { defer logsWG.Done(); streamWorkerLogs(ctx, stderr, user) }()

	// Read status events from the worker and write each as a multistream event
	// against this target — the DB side the worker can't reach itself.
	report := func(status, message string) {
		if cerr := mm.atsync.StatefulDB.CreateMultistreamEvent(targetView.Uri, message, status); cerr != nil {
			log.Error(ctx, "failed to create multistream event", "error", cerr)
		}
	}
	workerErr := consumePushEvents(ctx, eventsR, report)
	logsWG.Wait()
	werr := cmd.Wait()

	switch {
	case ctx.Err() != nil:
		// Target disabled (cancel) — a clean stop, mirroring in-process RTMPPush
		// returning ctx.Err() from the bus handler.
		return ctx.Err()
	case workerErr != nil:
		return fmt.Errorf("rtmp push worker stream: %w", workerErr)
	case werr != nil:
		// A non-zero exit without a clean end means the worker died — contained to
		// the subprocess; StartMultistreamTarget records it and retries.
		return fmt.Errorf("rtmp push worker exited: %w", werr)
	}
	return nil
}

// consumePushEvents reads status frames from the worker and hands each to
// report (which main wires to the multistream-event DB write). Returns nil on a
// clean End/EOF, the worker's reported message on an Error frame, or the
// terminal read error if the frame stream tore mid-frame (worker died).
func consumePushEvents(ctx context.Context, r io.Reader, report func(status, message string)) error {
	fr := ingestframe.NewReader(r)
	for {
		typ, payload, err := fr.ReadFrame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch typ {
		case ingestframe.Event:
			var ev pushEvent
			if uerr := json.Unmarshal(payload, &ev); uerr != nil {
				log.Error(ctx, "rtmp push: bad event frame", "error", uerr)
				continue
			}
			report(ev.Status, ev.Message)
		case ingestframe.Error:
			return fmt.Errorf("%s", string(payload))
		case ingestframe.End:
			return nil
		}
	}
}
