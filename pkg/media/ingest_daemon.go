package media

import (
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
)

// ingestReconnectBackoff paces redials when a worker is up but main's connection
// dropped (a restart in progress).
const ingestReconnectBackoff = 250 * time.Millisecond

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

	cmd := exec.Command(exe, "ingest-worker")
	setDetached(cmd) // own session, survives a main restart (Linux)
	// fd 3 = config; fd 4 = the fd-passed media connection (MKV/RTMP). WHIP owns
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
	return cmd.Process, nil
}

// ConsumeWorkerSocket connects to a worker's frame socket and feeds its segments
// to onSegment, reconnecting across transient disconnects. A worker that
// outlived a main restart keeps buffering and replays on reconnect; ValidateMP4's
// dedup makes any replayed overlap idempotent. Returns nil on a clean End, or an
// error if the socket vanishes without one (worker crashed — contained).
func (mm *MediaManager) ConsumeWorkerSocket(ctx context.Context, socketPath, streamer string, onSegment func([]byte) error) error {
	connectedOnce := false
	for {
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			if !connectedOnce {
				select { // worker may still be coming up
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(ingestReconnectBackoff):
					continue
				}
			}
			return fmt.Errorf("ingest worker socket gone before End: %w", err)
		}
		connectedOnce = true
		sawEnd, _ := mm.consumeWorkerFrames(ctx, conn, streamer, onSegment, nil)
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

// ingestWorkerSocketDir returns (creating it) the directory of per-session
// worker frame sockets — the set a restarting main scans to resume.
func (mm *MediaManager) ingestWorkerSocketDir() (string, error) {
	dir := mm.cli.DataFilePath([]string{"ingest-workers"})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// MKVIngestDetached is the production zero-downtime entry: main has authed the
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
func (mm *MediaManager) MKVIngestDetached(ctx context.Context, conn net.Conn, prebuf []byte, chunked bool, ms MediaSigner) error {
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

	err = mm.ConsumeWorkerSocket(ctx, cfg.SocketPath, ms.Streamer(), mm.validateSegment(ctx))
	if err == nil {
		go func() { _, _ = proc.Wait() }() // clean end: reap the exiting worker
	}
	// On ctx cancel (main shutting down) we deliberately leave the detached worker
	// running; a restarting main reconnects via discovery.
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

// readWHIPAnswer reads frames on conn until the worker's Answer frame and returns
// its SDP. An Error/End/EOF before the answer is a setup failure.
func readWHIPAnswer(conn net.Conn) (string, error) {
	fr := ingestframe.NewReader(conn)
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

	// Connect + read the SDP answer (the worker's first frame), bounded so a
	// wedged setup can't hang the WHIP client.
	answerCtx, answerCancel := context.WithTimeout(ctx, whipAnswerTimeout)
	defer answerCancel()
	conn, err := dialWorkerSocket(answerCtx, cfg.SocketPath)
	if err != nil {
		_ = proc.Kill()
		return "", fmt.Errorf("connect to whip worker: %w", err)
	}
	if dl, ok := answerCtx.Deadline(); ok {
		_ = conn.SetReadDeadline(dl)
	}
	answer, err := readWHIPAnswer(conn)
	if err != nil {
		conn.Close()
		_ = proc.Kill()
		return "", err
	}
	_ = conn.SetReadDeadline(time.Time{}) // clear; streaming has no deadline

	// Consume the signed segments in the background; the HTTP handler returns the
	// answer now and the WebRTC media establishes directly to the worker.
	go func() {
		sawEnd, _ := mm.consumeWorkerFrames(ctx, conn, ms.Streamer(), mm.validateSegment(ctx), nil)
		conn.Close()
		if !sawEnd && ctx.Err() == nil {
			// Connection dropped but the detached worker lives on — reconnect and
			// drain its buffer.
			_ = mm.ConsumeWorkerSocket(ctx, cfg.SocketPath, ms.Streamer(), mm.validateSegment(ctx))
		}
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
		log.Log(ctx, "resuming detached ingest worker", "socket", sock)
		go func() {
			if cerr := mm.ConsumeWorkerSocket(ctx, sock, "resumed", mm.validateSegment(ctx)); cerr != nil {
				log.Error(ctx, "resumed ingest worker ended", "socket", sock, "error", cerr)
			}
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
