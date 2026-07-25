//go:build linux

package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"stream.place/streamplace/pkg/log"
)

// IngestIsolationSupported reports whether per-stream ingest isolation (worker
// subprocesses with fd-passing + Setsid detachment) is available on this
// platform. It relies on Unix fd inheritance (exec.Cmd.ExtraFiles — unsupported
// on Windows) and POSIX sessions (Setsid), so it is gated to Linux, the platform
// Streamplace targets in production. Elsewhere --isolated-ingest falls back to
// in-process ingest. macOS has these primitives too and could be enabled by
// widening the build tag to `unix` once verified there.
func IngestIsolationSupported() bool { return true }

// setDetached puts a spawned worker in its own session so it survives a main
// restart (the zero-downtime path).
func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// killWorkerPID SIGKILLs a resumed detached worker by PID. A restarting main
// doesn't hold the process handle of a worker a previous main spawned, so it
// kills by the number recorded in the resume sidecar — but only after confirming
// via /proc/<pid>/cmdline that the PID is still OUR `ingest-worker <did>` (the
// DID rides argv). That guards against PID reuse: if the worker already died and
// its number was recycled, the cmdline won't match and we leave the new process
// alone.
func killWorkerPID(ctx context.Context, pid int, did string) {
	if pid <= 0 {
		return
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return // process already gone
	}
	args := bytes.Split(cmdline, []byte{0}) // argv is NUL-separated
	ours := false
	for i := 0; i+1 < len(args); i++ {
		if string(args[i]) == "ingest-worker" && string(args[i+1]) == did {
			ours = true
			break
		}
	}
	if !ours {
		log.Warn(ctx, "resumed worker PID is not our ingest-worker (reused?); not killing", "pid", pid, "did", did)
		return
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		log.Error(ctx, "failed to kill resumed ingest worker", "pid", pid, "error", err)
	}
}
