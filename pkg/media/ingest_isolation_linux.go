//go:build linux

package media

import (
	"os/exec"
	"syscall"
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
