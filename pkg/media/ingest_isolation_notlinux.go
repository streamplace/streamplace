//go:build !linux

package media

import (
	"context"
	"os/exec"
)

// IngestIsolationSupported is false off Linux: the worker-subprocess transport
// relies on Unix fd-passing (exec.Cmd.ExtraFiles, unsupported on Windows) and
// POSIX Setsid, so --isolated-ingest falls back to in-process ingest on these
// platforms (see the gate in runMain). This keeps the build green everywhere;
// Linux is where Streamplace runs in production.
func IngestIsolationSupported() bool { return false }

// setDetached is a no-op off Linux — the detached path is gated off there, so
// this only needs to keep the build compiling (no Setsid field exists on
// Windows' syscall.SysProcAttr).
func setDetached(cmd *exec.Cmd) {}

// killWorkerPID is a no-op off Linux: the detached/resume path is gated off
// there (ResumeDetachedWorkers never runs), so this only keeps the build green.
func killWorkerPID(ctx context.Context, pid int, did string) {}
