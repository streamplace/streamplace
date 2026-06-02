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
	"syscall"
	"time"

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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach into its own session
	cmd.ExtraFiles = []*os.File{cfgR, media}             // → child fd 3 (config), fd 4 (media)
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
