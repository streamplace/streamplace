//go:build !windows

package cmd

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"stream.place/streamplace/pkg/log"
)

// setNodeProcessGroup puts the forked e2e node in its own process group, so
// the harness can take down the node and everything it spawns with a single
// group signal instead of killing one process and orphaning its children.
func setNodeProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killNode tears the forked node down: first its process group, then the
// isolated ingest workers, which put themselves in their own session (see
// setDetached in pkg/media) precisely so they survive the death of a main.
// They carry the streamer DID in argv, and the DID is a throwaway created by
// this run, so matching on it cannot catch another node's worker.
//
// The errgroup context is not enough on its own: nothing downstream of the
// node watches it, and a SIGKILLed main never gets to reap what it spawned.
func killNode(ctx context.Context, cmd *exec.Cmd, did string) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// A negative pid addresses the whole process group.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		// No group (already gone, or never got one): at least take the node.
		_ = cmd.Process.Kill() //nolint:errcheck
	}
	killIngestWorkers(ctx, did)
}

// killIngestWorkers SIGKILLs any `ingest-worker <did>` left behind, matching
// on the full command line. Best-effort: pgrep is absent outside a procps
// environment, and finding nothing is the normal case.
func killIngestWorkers(ctx context.Context, did string) {
	if did == "" {
		return
	}
	out, err := exec.CommandContext(ctx, "pgrep", "-f", "ingest-worker "+did).Output()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			log.Log(ctx, "e2e: could not look for leftover ingest workers", "err", err)
		}
		return
	}
	for _, field := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil {
			log.Log(ctx, "e2e: killed leftover ingest worker", "pid", pid)
		}
	}
}
