//go:build linux

package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
)

// spawnFakeWorker re-execs the test binary as a worker-shaped process that just
// sleeps — same argv as a real worker (".../test ingest-worker <did>") so
// /proc/<pid>/cmdline matches killWorkerPID's safety check, without needing a gst
// pipeline.
func spawnFakeWorker(t *testing.T, did string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(exe, "ingest-worker", did, "__test_sleep__")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	// Wait for exec() to complete so /proc/<pid>/cmdline reflects the worker argv —
	// the property killWorkerPID's safety check reads.
	require.Eventually(t, func() bool {
		b, rerr := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", cmd.Process.Pid))
		return rerr == nil && bytes.Contains(b, []byte("ingest-worker\x00"+did))
	}, 5*time.Second, 20*time.Millisecond, "fake worker did not exec with the expected cmdline")
	return cmd
}

// TestKillWorkerPIDRefusesMismatch is the PID-reuse guard: a PID whose
// /proc cmdline is NOT our ingest-worker for this DID must be left alone.
func TestKillWorkerPIDRefusesMismatch(t *testing.T) {
	cmd := spawnFakeWorker(t, "did:plc:realstreamer")
	pid := cmd.Process.Pid

	killWorkerPID(context.Background(), pid, "did:plc:someone-else")

	time.Sleep(300 * time.Millisecond)
	// Signal 0 probes liveness; the process wasn't killed, so it's running (not a
	// zombie), and this returns nil.
	require.NoError(t, syscall.Kill(pid, 0), "killWorkerPID must not kill a PID whose cmdline DID doesn't match")
}

// TestKillWorkerPIDKillsMatch: the matching worker is killed.
func TestKillWorkerPIDKillsMatch(t *testing.T) {
	did := "did:plc:realstreamer"
	cmd := spawnFakeWorker(t, did)

	died := make(chan struct{})
	go func() { _, _ = cmd.Process.Wait(); close(died) }()

	killWorkerPID(context.Background(), cmd.Process.Pid, did)

	select {
	case <-died:
	case <-time.After(5 * time.Second):
		t.Fatal("killWorkerPID did not kill the matching worker")
	}
}

// TestResumeDetachedWorkerBanContained closes the resumed-worker gap end to end:
// a detached worker that outlived a main restart (here a worker-shaped sleeper
// plus its socket + resume sidecar) gets torn down when its streamer is banned,
// even though the new main never spawned it and holds no process handle.
func TestResumeDetachedWorkerBanContained(t *testing.T) {
	did := "did:plc:resumed-streamer"
	mm := &MediaManager{bus: bus.NewBus(), cli: &config.CLI{DataDir: t.TempDir()}}

	dir, err := mm.ingestWorkerSocketDir()
	require.NoError(t, err)

	// Stand in for a worker left behind by a previous main: a worker-shaped
	// sleeper, plus a discovered socket file and the resume sidecar pointing at it.
	worker := spawnFakeWorker(t, did)
	sockPath := filepath.Join(dir, uuid.NewString()+".sock")
	require.NoError(t, os.WriteFile(sockPath, nil, 0o644)) // discovered; not a live listener
	require.NoError(t, writeWorkerMeta(sockPath, workerMeta{StreamerDID: did, PID: worker.Process.Pid}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Resume reads the sidecar and re-arms ban enforcement while it (fails to)
	// drain the dead socket within the connect grace.
	mm.ResumeDetachedWorkers(ctx)

	died := make(chan struct{})
	go func() { _, _ = worker.Process.Wait(); close(died) }()

	banned := &comatproto.LabelDefs_Label{Val: atproto.LabelDMCAViolation, Uri: did}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case <-died:
			return // the resumed worker was killed by the ban — gap closed
		case <-deadline:
			t.Fatal("resumed worker was not killed after its streamer was banned")
		case <-tick.C:
			mm.bus.Publish(did, banned) // re-publish until the watcher's subscription is live
		}
	}
}
