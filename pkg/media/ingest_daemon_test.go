//go:build linux

package media

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
)

func TestDiscoverWorkerSockets(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600)) // ignored
	for _, n := range []string{"a.sock", "b.sock"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), nil, 0o600))
	}
	socks, err := DiscoverWorkerSockets(dir)
	require.NoError(t, err)
	require.Len(t, socks, 2)

	socks, err = DiscoverWorkerSockets(filepath.Join(dir, "nope")) // missing dir → empty, no error
	require.NoError(t, err)
	require.Empty(t, socks)
}

// TestDetachedWorkerZeroDowntime exercises the main-side lifecycle end to end:
// spawn a worker DETACHED (its own session, so it survives a main restart),
// fd-passing its media; discover its socket the way a restarting main would;
// consume via the reconnecting consumer; and verify it served valid signed
// dual-codec segments through a clean End. The session check confirms the
// detachment that lets the worker outlive main.
func TestDetachedWorkerZeroDowntime(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)

	dir := t.TempDir()
	sock := filepath.Join(dir, "stream.sock")
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		NodeCertPEM:     ms.Cert,
		NodeKeyPEM:      keyPEM,
		BroadcasterHost: "test.example.com",
		SocketPath:      sock,
		InputFD:         4,
	}

	mp4 := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))
	mediaR, mediaW, err := os.Pipe()
	require.NoError(t, err)

	proc, err := SpawnIngestWorkerDetached(cfg, mediaR)
	require.NoError(t, err)
	mediaR.Close() // the worker holds its own dup
	go func() {
		_, _ = mediaW.Write(mp4)
		mediaW.Close()
	}()

	// A (re)starting main discovers the running worker by scanning the socket dir.
	var socks []string
	require.Eventually(t, func() bool {
		socks, _ = DiscoverWorkerSockets(dir)
		return len(socks) == 1
	}, 15*time.Second, 100*time.Millisecond, "worker socket appears for discovery")

	// The worker runs in its own session/process group (Setsid) — the property
	// that lets it outlive a main restart.
	wpgid, err := syscall.Getpgid(proc.Pid)
	require.NoError(t, err)
	require.Equal(t, proc.Pid, wpgid, "worker leads its own process group (Setsid detached)")
	require.NotEqual(t, syscall.Getpgrp(), wpgid, "worker group differs from the test's")

	// Consume through the reconnecting consumer; verify dual-codec signed output.
	var segs int
	onSegment := func(s []byte) error {
		out, verr := muxl.RunMuxlVerify(ctx, bytes.NewReader(s))
		require.NoError(t, verr)
		require.NotContains(t, out, `"validation_state":"Invalid"`)
		segs++
		return nil
	}
	require.NoError(t, (&MediaManager{}).ConsumeWorkerSocket(ctx, socks[0], ms.Streamer(), onSegment, nil))
	require.GreaterOrEqual(t, segs, 1, "detached worker served signed segments")

	_, _ = proc.Wait() // reap the detached worker
	t.Logf("detached worker (pgid=%d) served %d dual-codec segments via discovery+reconnect", wpgid, segs)
}
