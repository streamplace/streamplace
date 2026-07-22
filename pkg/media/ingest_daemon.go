package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// ingestReconnectBackoff paces redials when a worker is up but main's connection
// dropped (a restart in progress).
const ingestReconnectBackoff = 250 * time.Millisecond

// workerConnectGrace bounds the INITIAL connect to a worker's socket. A freshly
// spawned worker takes a moment to listen; a socket that never accepts within
// this window is a dead/never-started worker — or a stale socket a SIGKILLed
// worker left behind — so we give up and unlink it rather than spin forever.
const workerConnectGrace = 15 * time.Second

// SpawnIngestWorkerDetached launches an ingest worker in its OWN session
// (Setsid) so it outlives a main restart, fd-passing the (already authed) ingest
// connection as the worker's media input (fd 4) and having it serve signed
// segments over cfg.SocketPath with buffered reconnect. The config rides a pipe
// on fd 3, written synchronously so nothing has to outlive a restarting main.
//
// The worker is deliberately NOT tied to main's context and NOT waited on here:
// it self-terminates after its stream drains (or its watchdog/orphan-grace
// fires), and is reaped by init once main exits. The returned process lets a
// caller that stays alive reap it; a restarting main just lets it go.
func SpawnIngestWorkerDetached(cfg IngestWorkerConfig, media *os.File) (*os.Process, error) {
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("SpawnIngestWorkerDetached: empty socket path")
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	cfgR, cfgW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer cfgR.Close()
	defer cfgW.Close()

	// The streamer DID rides argv (public, non-sensitive) so a worker is
	// identifiable in a process listing; key material stays on fd 3.
	cmd := exec.Command(exe, "ingest-worker", cfg.StreamerDID)
	setDetached(cmd) // own session, survives a main restart (Linux)
	// fd 3 = config; fd 4 = the fd-passed media connection (the Mist fMP4 pull, or an fMP4 push). WHIP owns
	// its own PeerConnection, so it passes no media fd.
	cmd.ExtraFiles = []*os.File{cfgR}
	if media != nil {
		cmd.ExtraFiles = append(cmd.ExtraFiles, media)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn ingest worker: %w", err)
	}
	// Small, synchronous write: the bytes sit in the pipe buffer for the worker to
	// read even after we close + return (and even if main then exits/restarts).
	if _, err := cfgW.Write(cfgJSON); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("send worker config: %w", err)
	}
	// Resume sidecar: a restarting main discovers sockets by UUID alone, which
	// carries no identity. Record the streamer DID + PID next to the socket so the
	// new main can re-arm ban enforcement (watchKeyRevocation) and, since it won't
	// hold this worker's process handle, kill it by PID if needed. Best-effort:
	// without it the worker still runs, just without restart-surviving ban
	// enforcement.
	if err := writeWorkerMeta(cfg.SocketPath, workerMeta{StreamerDID: cfg.StreamerDID, PID: cmd.Process.Pid}); err != nil {
		log.Warn(context.Background(), "ingest worker: write resume metadata failed", "socket", cfg.SocketPath, "error", err)
	}
	return cmd.Process, nil
}

// workerMeta is the resume sidecar written next to a detached worker's socket:
// enough for a restarting main to enforce bans on a worker it didn't spawn.
type workerMeta struct {
	StreamerDID string `json:"streamer_did"`
	PID         int    `json:"pid"`
}

func workerMetaPath(socketPath string) string {
	return strings.TrimSuffix(socketPath, ".sock") + ".json"
}

func writeWorkerMeta(socketPath string, meta workerMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(workerMetaPath(socketPath), b, 0o644)
}

func readWorkerMeta(socketPath string) (workerMeta, error) {
	var m workerMeta
	b, err := os.ReadFile(workerMetaPath(socketPath))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(b, &m)
}

// removeWorkerFiles unlinks a worker's socket and its resume sidecar together,
// so the sidecar never outlives the socket it describes.
func removeWorkerFiles(socketPath string) {
	_ = os.Remove(socketPath)
	_ = os.Remove(workerMetaPath(socketPath))
}

// ConsumeWorkerSocket connects to a worker's frame socket and feeds its segments
// to onSegment, reconnecting across transient disconnects. A worker that
// outlived a main restart keeps buffering and replays on reconnect; ValidateMP4's
// dedup makes any replayed overlap idempotent. Returns nil on a clean End, or an
// error if the socket vanishes without one (worker crashed — contained).
//
// manifestSource, when non-nil, is polled to refresh the worker's C2PA manifest
// over the same socket (pushManifestUpdates) — so a pre-live → live transition
// reaches a worker that has no model of its own. It's re-armed per connection.
func (mm *MediaManager) ConsumeWorkerSocket(ctx context.Context, socketPath, streamer string, onSegment func([]byte) error, manifestSource func() ([]byte, error)) error {
	connectedOnce := false
	giveUp := time.Now().Add(workerConnectGrace)
	for {
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			if !connectedOnce {
				if time.Now().After(giveUp) {
					// Never came up within the grace: a dead/never-started worker, or a
					// stale socket a SIGKILLed worker left behind. Unlink it so it can't
					// linger or re-spin a consumer on the next restart.
					removeWorkerFiles(socketPath)
					return fmt.Errorf("ingest worker never came up at %s: %w", socketPath, err)
				}
				select { // worker may still be coming up
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(ingestReconnectBackoff):
					continue
				}
			}
			// Connected before, now the socket's gone: the worker exited. A clean
			// exit already unlinked its socket; a SIGKILL leaves it behind, so remove
			// the leftover (no-op if already gone).
			removeWorkerFiles(socketPath)
			return fmt.Errorf("ingest worker socket gone before End: %w", err)
		}
		connectedOnce = true
		// Push manifest refreshes to the worker over this connection (re-armed per
		// connect); stop the pusher when the connection ends.
		connCtx, connCancel := context.WithCancel(ctx)
		if manifestSource != nil {
			go pushManifestUpdates(connCtx, conn, manifestSource)
		}
		// Fresh Reader per connection: a reconnect is a new stream where the worker
		// replays its buffer from the start.
		sawEnd, _ := mm.consumeWorkerFrames(ctx, ingestframe.NewReader(conn), streamer, onSegment, nil)
		connCancel()
		conn.Close()
		if sawEnd {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Transient disconnect: reconnect and let the worker replay its buffer.
		log.Log(ctx, "ingest worker connection dropped; reconnecting", "streamer", streamer)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(ingestReconnectBackoff):
		}
	}
}

// manifestRefreshInterval bounds how stale an isolated worker's manifest can be —
// roughly how long after a pre-live → live transition before the worker starts
// signing published segments. Var (not const) so tests can shorten it.
var manifestRefreshInterval = 2 * time.Second

// pushManifestUpdates periodically rebuilds the streamer's manifest and sends it
// to the worker over conn whenever it changes, so a pre-live → live transition
// (or a mid-stream title/metadata change) reaches the isolated worker — which
// has no model to notice it — without reconnecting. manifestSource builds with a
// fixed start, so the bytes change only on real content changes (muxl stamps the
// per-segment timestamp). Returns on ctx cancel or a write error (the connection
// dropped; the consume loop reconnects and re-arms the pusher).
func pushManifestUpdates(ctx context.Context, conn net.Conn, manifestSource func() ([]byte, error)) {
	w := ingestframe.NewWriter(conn)
	var last []byte
	tick := time.NewTicker(manifestRefreshInterval)
	defer tick.Stop()
	for {
		manifest, err := manifestSource()
		if err != nil {
			log.Error(ctx, "ingest worker: build manifest for refresh", "error", err)
		} else if !bytes.Equal(manifest, last) {
			if werr := w.Manifest(manifest); werr != nil {
				return
			}
			last = manifest
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// streamerManifest builds the current C2PA manifest for streamerDID from live
// model state (title, pre-live → live c2pa.published) — what an isolated worker
// needs but can't compute itself (no model). It needs only the model + cli, not
// the streamer's signing key, so it works the same for a live worker (DID from
// the signer) and a resumed one (DID from the sidecar). start seeds a placeholder
// dc:date muxl overwrites per segment, so a fixed value keeps bytes stable for
// change detection.
func (mm *MediaManager) streamerManifest(ctx context.Context, streamerDID string, start int64) ([]byte, error) {
	return NewManifestBuilder(mm.model, mm.cli).BuildManifest(ctx, streamerDID, start)
}

// ingestWorkerSocketDir returns (creating it) the directory of per-session
// worker frame sockets — the set a restarting main scans to resume.
func (mm *MediaManager) ingestWorkerSocketDir() (string, error) {
	dir := mm.cli.DataFilePath([]string{"ingest-workers"})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// MP4IngestDetached is the production zero-downtime entry: main has authed the
// push and hijacked its connection; this fd-passes that connection to a DETACHED
// worker (own session, survives a main restart) which ingests the media directly
// and serves signed segments over a per-session unix socket, and then consumes
// those frames into ValidateMP4 with reconnect. prebuf is any body bytes main
// already read past the headers; chunked says the push body is chunked.
//
// Because the worker owns the connection and is detached, a main restart neither
// breaks the ingest nor loses output: the worker keeps signing into its buffer,
// and the restarted main rediscovers the socket (DiscoverWorkerSockets) and
// drains it.
func (mm *MediaManager) MP4IngestDetached(ctx context.Context, conn net.Conn, prebuf []byte, chunked bool, ms MediaSigner) error {
	cfg, err := mm.buildWorkerConfig(ctx, ms)
	if err != nil {
		return err
	}
	dir, err := mm.ingestWorkerSocketDir()
	if err != nil {
		return err
	}
	cfg.SocketPath = filepath.Join(dir, uuid.NewString()+".sock")
	cfg.InputFD = 4
	cfg.Prebuf = prebuf
	cfg.Chunked = chunked

	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("isolated ingest requires a TCP connection, got %T", conn)
	}
	connFile, err := tcp.File() // dup the fd to hand to the worker
	if err != nil {
		return fmt.Errorf("dup ingest connection: %w", err)
	}

	proc, err := SpawnIngestWorkerDetached(cfg, connFile)
	connFile.Close() // the worker holds its own dup
	conn.Close()     // main is out of the media path now
	if err != nil {
		return fmt.Errorf("spawn detached worker: %w", err)
	}
	spmetrics.IngestWorkerStarts.WithLabelValues("mp4").Inc()

	// Ban / key revocation: the detached worker can't notice it itself (no
	// bus/model), so main watches and kills it. proc.Kill (not ctx cancel) so the
	// reap below still fires — a ctx cancel means "main shutting down, leave it
	// running", which is the opposite of what a ban wants.
	go mm.watchKeyRevocation(ctx, ms.Streamer(), ms.DID(), func(reason string) {
		log.Warn(ctx, "detached ingest worker: ending stream", "reason", reason, "streamer", ms.Streamer())
		_ = proc.Kill()
	})

	// Refresh the worker's manifest from live model state (pre-live → live), since
	// it signs with a frozen one otherwise. Fixed start for stable change detection.
	start := time.Now().UnixMilli()
	manifestSource := func() ([]byte, error) { return mm.streamerManifest(ctx, ms.Streamer(), start) }
	err = mm.ConsumeWorkerSocket(ctx, cfg.SocketPath, ms.Streamer(), mm.validateSegment(ctx), manifestSource)
	recordWorkerExit("mp4", err, ctx.Err())
	// Reap the worker unless we're deliberately leaving it running across a main
	// restart (ctx cancel). On a clean end OR a crash the worker has exited, so
	// Wait() clears the zombie; only on main shutdown do we let it stay detached
	// for a restarting main to rediscover via DiscoverWorkerSockets.
	if ctx.Err() == nil {
		go func() { _, _ = proc.Wait() }()
	}
	return err
}

// whipAnswerTimeout bounds how long main waits for the worker to produce the SDP
// answer (worker startup + ICE gathering) before giving up on the WHIP request.
const whipAnswerTimeout = 20 * time.Second

// dialWorkerSocket connects to a worker's frame socket, retrying until it's up or
// ctx is done (a freshly-spawned worker takes a moment to start listening).
func dialWorkerSocket(ctx context.Context, socketPath string) (net.Conn, error) {
	for {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			return conn, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(ingestReconnectBackoff):
		}
	}
}

// readWHIPAnswer reads frames until the worker's Answer frame and returns its
// SDP. An Error/End/EOF before the answer is a setup failure. It reads through
// the caller's Reader so the same decoder (and its buffered read-ahead) carries
// on to the segment stream.
func readWHIPAnswer(fr *ingestframe.Reader) (string, error) {
	for {
		typ, payload, err := fr.ReadFrame()
		if err != nil {
			return "", fmt.Errorf("read whip answer: %w", err)
		}
		switch typ {
		case ingestframe.Answer:
			return string(payload), nil
		case ingestframe.Error:
			return "", fmt.Errorf("whip worker error: %s", payload)
		case ingestframe.End:
			return "", fmt.Errorf("whip worker ended before sending an answer")
		}
		// A Segment before the Answer shouldn't happen; ignore it defensively.
	}
}

// WHIPIngestDetached is the WHIP zero-downtime entry. Main has authed the WHIP
// request; this spawns a DETACHED worker that owns the PeerConnection (built from
// offerSDP, binding its own UDP sockets) and serves signed segments over a
// per-session socket. It reads the worker's SDP answer (the first frame) to
// return to the client, then consumes segments into ValidateMP4 in the
// background with reconnect. Because the worker owns the WebRTC session and is
// detached, both the session and its buffered output survive a main restart (the
// restarted main reconnects via discovery).
func (mm *MediaManager) WHIPIngestDetached(ctx context.Context, offerSDP string, ms MediaSigner) (string, error) {
	cfg, err := mm.buildWorkerConfig(ctx, ms)
	if err != nil {
		return "", err
	}
	dir, err := mm.ingestWorkerSocketDir()
	if err != nil {
		return "", err
	}
	cfg.SocketPath = filepath.Join(dir, uuid.NewString()+".sock")
	cfg.Transport = IngestTransportWHIP
	cfg.OfferSDP = offerSDP

	proc, err := SpawnIngestWorkerDetached(cfg, nil) // worker owns the PeerConnection
	if err != nil {
		return "", fmt.Errorf("spawn detached whip worker: %w", err)
	}
	spmetrics.IngestWorkerStarts.WithLabelValues("whip").Inc()

	// Connect + read the SDP answer (the worker's first frame), bounded so a
	// wedged setup can't hang the WHIP client.
	answerCtx, answerCancel := context.WithTimeout(ctx, whipAnswerTimeout)
	defer answerCancel()
	conn, err := dialWorkerSocket(answerCtx, cfg.SocketPath)
	if err != nil {
		_ = proc.Kill()
		return "", fmt.Errorf("connect to whip worker: %w", err)
	}
	// One Reader owns this connection for its whole lifetime: the streaming CBOR
	// decoder reads ahead, so the Answer and the segments that follow must come
	// through the SAME Reader (a second one would lose buffered read-ahead).
	fr := ingestframe.NewReader(conn)
	if dl, ok := answerCtx.Deadline(); ok {
		_ = conn.SetReadDeadline(dl)
	}
	answer, err := readWHIPAnswer(fr)
	if err != nil {
		conn.Close()
		_ = proc.Kill()
		return "", err
	}
	_ = conn.SetReadDeadline(time.Time{}) // clear; streaming has no deadline

	// Consume the signed segments in the background; the HTTP handler returns the
	// answer now and the WebRTC media establishes directly to the worker.
	go func() {
		// Ban / key revocation: watch on the detached worker's behalf and kill it.
		// Scoped to this consume's lifetime so it doesn't outlive the stream.
		wctx, wcancel := context.WithCancel(ctx)
		defer wcancel()
		go mm.watchKeyRevocation(wctx, ms.Streamer(), ms.DID(), func(reason string) {
			log.Warn(wctx, "whip ingest worker: ending stream", "reason", reason, "streamer", ms.Streamer())
			_ = proc.Kill()
		})

		// Refresh the worker's manifest (pre-live → live) over this connection,
		// scoped to the consume; the reconnect fallback re-arms its own.
		start := time.Now().UnixMilli()
		manifestSource := func() ([]byte, error) { return mm.streamerManifest(ctx, ms.Streamer(), start) }
		go pushManifestUpdates(wctx, conn, manifestSource)

		sawEnd, _ := mm.consumeWorkerFrames(ctx, fr, ms.Streamer(), mm.validateSegment(ctx), nil)
		conn.Close()
		var exitErr error
		if !sawEnd && ctx.Err() == nil {
			// Connection dropped but the detached worker lives on — reconnect and
			// drain its buffer. Its terminal result is the worker's true outcome.
			exitErr = mm.ConsumeWorkerSocket(ctx, cfg.SocketPath, ms.Streamer(), mm.validateSegment(ctx), manifestSource)
		}
		recordWorkerExit("whip", exitErr, ctx.Err())
		go func() { _, _ = proc.Wait() }()
	}()
	return answer, nil
}

// ResumeDetachedWorkers reconnects to any ingest workers still running from
// before a main restart and resumes consuming their frames (draining whatever
// they buffered while main was down). Intended to run once at main startup.
func (mm *MediaManager) ResumeDetachedWorkers(ctx context.Context) {
	dir, err := mm.ingestWorkerSocketDir()
	if err != nil {
		log.Error(ctx, "resume ingest workers: socket dir", "error", err)
		return
	}
	socks, err := DiscoverWorkerSockets(dir)
	if err != nil {
		log.Error(ctx, "resume ingest workers: discover", "error", err)
		return
	}
	for _, sock := range socks {
		sock := sock
		meta, merr := readWorkerMeta(sock)
		streamer := "resumed"
		if merr == nil && meta.StreamerDID != "" {
			streamer = meta.StreamerDID
		}
		log.Log(ctx, "resuming detached ingest worker", "socket", sock, "streamer", streamer)
		go func() {
			wctx, wcancel := context.WithCancel(ctx)
			defer wcancel()
			// Re-arm ban enforcement for a worker we didn't spawn. We have no process
			// handle, so kill by the PID in the sidecar (guarded against PID reuse).
			// Without metadata we can still drain the worker, just not enforce bans.
			var manifestSource func() ([]byte, error)
			if merr != nil || meta.StreamerDID == "" {
				log.Warn(ctx, "resumed ingest worker: no resume metadata; cannot enforce bans or refresh manifest on it", "socket", sock, "error", merr)
			} else {
				go mm.watchKeyRevocation(wctx, meta.StreamerDID, meta.StreamerDID, func(reason string) {
					log.Warn(wctx, "resumed ingest worker: ending stream", "reason", reason, "streamer", meta.StreamerDID)
					killWorkerPID(wctx, meta.PID, meta.StreamerDID)
				})
				// Keep refreshing the resumed worker's manifest too (no signer needed —
				// streamerManifest only uses the model + cli + the sidecar DID).
				start := time.Now().UnixMilli()
				manifestSource = func() ([]byte, error) { return mm.streamerManifest(ctx, meta.StreamerDID, start) }
			}
			cerr := mm.ConsumeWorkerSocket(wctx, sock, streamer, mm.validateSegment(ctx), manifestSource)
			if cerr != nil {
				log.Error(ctx, "resumed ingest worker ended", "socket", sock, "error", cerr)
			}
			recordWorkerExit("resumed", cerr, ctx.Err())
		}()
	}
}

// DiscoverWorkerSockets lists the worker frame sockets under dir — the running
// workers a restarting main should reconnect to and resume consuming.
func DiscoverWorkerSockets(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var socks []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sock") {
			socks = append(socks, filepath.Join(dir, e.Name()))
		}
	}
	return socks, nil
}
