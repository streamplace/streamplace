//go:build windows

package cmd

import (
	"context"
	"os/exec"
)

// setNodeProcessGroup is a no-op on Windows: syscall.SysProcAttr has no
// Setpgid field there. Isolated ingest — the only thing that detaches — is
// gated to Linux anyway (see pkg/media/ingest_isolation_notlinux.go), so
// killing the node process is enough.
func setNodeProcessGroup(cmd *exec.Cmd) {}

// killNode terminates the forked node. There is nothing detached to sweep.
func killNode(ctx context.Context, cmd *exec.Cmd, did string) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill() //nolint:errcheck
}
