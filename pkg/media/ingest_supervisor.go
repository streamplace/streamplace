package media

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/log"
)

// ingestWorkerWatchdog bounds how long an isolated worker may go without
// producing a segment frame before it's presumed wedged (a native pipeline gst
// can't drain — e.g. a pathological stream) and killed. A healthy stream emits a
// segment every GoP (~1–2s), so this is generous. Var (not const) so tests can
// shorten it.
var ingestWorkerWatchdog = 30 * time.Second

// MKVIngestIsolated is the process-isolated counterpart to MKVIngest. Instead of
// running the demux + sign pipeline in this process — where a native gst fault,
// OOM, or deadlock would take the whole node down — it spawns a dedicated
// `ingest-worker` subprocess that owns the pipeline and streams signed canonical
// .m4s segments back. This process only reads frames and runs ValidateMP4
// (memory-safe Go + wasm), so a single failing stream can at worst kill its own
// worker; the node survives.
//
// Per the locked design the worker signs everything, so main hands it the
// streamer key + cert + a once-built manifest over a dedicated config fd (kept
// off argv/env). See buildWorkerConfig for the interim key-custody note.
func (mm *MediaManager) MKVIngestIsolated(ctx context.Context, input io.Reader, ms MediaSigner) error {
	cfg, err := mm.buildWorkerConfig(ctx, ms)
	if err != nil {
		return err
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal worker config: %w", err)
	}

	// Optional recording stays in main: tee the raw input before it reaches the
	// worker, so the worker needs no data-dir access.
	if shouldRecord, rerr := mm.shouldRecord(ctx, ms.Streamer()); rerr == nil && shouldRecord {
		log.Log(ctx, "recording RTMP stream to file", "streamer", ms.Streamer())
		pr, pw := io.Pipe()
		input = io.TeeReader(input, pw)
		go func() {
			if derr := mm.dumpToFile(ctx, pr, ms.Streamer(), ".rtmp.mkv"); derr != nil {
				log.Error(ctx, "error dumping to file", "error", derr)
			}
		}()
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, "ingest-worker")

	// Dedicated pipes: fd 3 carries the config in, fd 4 carries the frame stream
	// out. Keeping frames off stdout means nothing the worker (or gst, or the
	// self-test) writes to stdout/stderr can corrupt them; those stay plain logs.
	cfgR, cfgW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("config pipe: %w", err)
	}
	defer cfgW.Close()
	framesR, framesW, err := os.Pipe()
	if err != nil {
		cfgR.Close()
		return fmt.Errorf("frames pipe: %w", err)
	}
	defer framesR.Close()
	cmd.ExtraFiles = []*os.File{cfgR, framesW} // → child fd 3, fd 4

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cfgR.Close()
		framesW.Close()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cfgR.Close()
		framesW.Close()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cfgR.Close()
		framesW.Close()
		return err
	}

	if err := cmd.Start(); err != nil {
		cfgR.Close()
		framesW.Close()
		return fmt.Errorf("start ingest worker: %w", err)
	}
	cfgR.Close()    // the child holds its own copy now
	framesW.Close() // ditto; the parent only reads framesR

	go func() {
		_, _ = cfgW.Write(cfgJSON)
		cfgW.Close() // EOF so the worker's config read completes
	}()

	// Pump the media to the worker; closing stdin on input EOF is the worker's
	// end-of-stream. Managed here (not cmd.Stdin) so a worker exit can't wedge
	// cmd.Wait on a still-blocked body read.
	go func() {
		defer stdin.Close()
		_, _ = io.Copy(stdin, input)
	}()

	// Forward stdout + stderr to the node logger. Drain both fully before Wait.
	var logsWG sync.WaitGroup
	logsWG.Add(2)
	go func() { defer logsWG.Done(); streamWorkerLogs(ctx, stdout, ms.Streamer()) }()
	go func() { defer logsWG.Done(); streamWorkerLogs(ctx, stderr, ms.Streamer()) }()

	// Watchdog: a worker that stops producing frames is presumed wedged and
	// killed (cancel → CommandContext SIGKILLs it), so a single bad stream can't
	// hang its session forever. Reset on every frame.
	watchdog := time.AfterFunc(ingestWorkerWatchdog, func() {
		log.Warn(ctx, "ingest worker watchdog fired (no frames); killing worker",
			"streamer", ms.Streamer(), "timeout", ingestWorkerWatchdog)
		cancel()
	})
	defer watchdog.Stop()

	// Read signed-segment frames and feed each into the normal chokepoint.
	sawEnd, readErr := mm.consumeWorkerFrames(ctx, framesR, ms.Streamer(), func() {
		watchdog.Reset(ingestWorkerWatchdog)
	})
	logsWG.Wait()
	werr := cmd.Wait()

	switch {
	case readErr != nil:
		return fmt.Errorf("ingest worker stream: %w", readErr)
	case werr != nil && !sawEnd:
		// A non-zero exit without a clean End frame means the worker died — that
		// failure is contained to the subprocess; the node is unaffected.
		return fmt.Errorf("ingest worker exited: %w", werr)
	case werr != nil:
		log.Warn(ctx, "ingest worker signalled clean end but exited nonzero", "streamer", ms.Streamer(), "error", werr)
	}
	return nil
}

// consumeWorkerFrames reads framed segments from the worker and runs ValidateMP4
// over each. It returns whether a clean End frame was seen and the terminal read
// error: nil on a clean close (End then EOF), or io.ErrUnexpectedEOF / a desync
// error when the worker died mid-frame.
func (mm *MediaManager) consumeWorkerFrames(ctx context.Context, stdout io.Reader, streamer string, onProgress func()) (sawEnd bool, _ error) {
	fr := ingestframe.NewReader(stdout)
	for {
		typ, payload, err := fr.ReadFrame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return sawEnd, nil
			}
			return sawEnd, err
		}
		if onProgress != nil {
			onProgress()
		}
		switch typ {
		case ingestframe.Segment:
			if verr := mm.ValidateMP4(ctx, bytes.NewReader(payload), true); verr != nil {
				log.Error(ctx, "ingest worker: validate segment failed", "streamer", streamer, "error", verr)
			}
		case ingestframe.End:
			sawEnd = true
		case ingestframe.Error:
			log.Error(ctx, "ingest worker: reported error", "streamer", streamer, "error", string(payload))
		}
	}
}

// streamWorkerLogs forwards the worker's stderr lines into the node logger.
func streamWorkerLogs(ctx context.Context, stderr io.Reader, streamer string) {
	scan := bufio.NewScanner(stderr)
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scan.Scan() {
		if line := scan.Text(); line != "" {
			log.Log(ctx, "[ingest-worker] "+line, "streamer", streamer)
		}
	}
}

// buildWorkerConfig extracts the handshake the worker needs to sign on main's
// behalf. INTERIM key custody: requires a software MediaSignerLocal — the
// MKV/RTMP push path always provides one; anything else errors so the caller can
// fall back to the in-process path.
func (mm *MediaManager) buildWorkerConfig(ctx context.Context, ms MediaSigner) (IngestWorkerConfig, error) {
	local, ok := ms.(*MediaSignerLocal)
	if !ok {
		return IngestWorkerConfig{}, fmt.Errorf("isolated ingest requires a local signer, got %T", ms)
	}
	if _, ok := local.Signer.(*ecdsa.PrivateKey); !ok {
		return IngestWorkerConfig{}, fmt.Errorf("isolated ingest requires a software signer, got %T", local.Signer)
	}
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(local.Signer)
	if err != nil {
		return IngestWorkerConfig{}, fmt.Errorf("marshal streamer key: %w", err)
	}
	manifest, err := local.buildManifest(ctx, time.Now().UnixMilli())
	if err != nil {
		return IngestWorkerConfig{}, fmt.Errorf("build manifest: %w", err)
	}
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         local.Cert,
		Manifest:        manifest,
		BroadcasterHost: mm.cli.BroadcasterHost,
	}
	// Node transcode signer lets the worker complete to dual-codec itself. If it's
	// unavailable, the worker emits single-codec (the node doesn't re-transcode the
	// worker's output) — an acceptable, logged fallback rather than a hard failure.
	if nodeCert, nodeKeyPEM, serr := mm.transcodeSigner(); serr == nil {
		cfg.NodeCertPEM = nodeCert
		cfg.NodeKeyPEM = nodeKeyPEM
	} else {
		log.Warn(ctx, "node transcode signer unavailable; isolated worker will emit single-codec", "error", serr)
	}
	return cfg, nil
}
