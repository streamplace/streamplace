package media

import (
	"context"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/ingestframe"
)

// TestPushManifestUpdatesOnChange checks the main side: pushManifestUpdates sends
// a Manifest frame for the initial manifest and again whenever it changes (e.g.
// a pre-live → live transition), but not while it's unchanged.
func TestPushManifestUpdatesOnChange(t *testing.T) {
	old := manifestRefreshInterval
	manifestRefreshInterval = 20 * time.Millisecond
	defer func() { manifestRefreshInterval = old }()

	mainConn, workerConn := net.Pipe()
	defer mainConn.Close()
	defer workerConn.Close()

	var mu sync.Mutex
	cur := []byte("manifest-prelive")
	source := func() ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		return append([]byte(nil), cur...), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pushManifestUpdates(ctx, mainConn, source)

	fr := ingestframe.NewReader(workerConn)
	readManifest := func() string {
		t.Helper()
		typ, payload, err := fr.ReadFrame()
		require.NoError(t, err)
		require.Equal(t, ingestframe.Manifest, typ)
		return string(payload)
	}

	require.Equal(t, "manifest-prelive", readManifest(), "initial manifest is pushed")

	mu.Lock()
	cur = []byte("manifest-live-published")
	mu.Unlock()
	// The next frame is the changed manifest — unchanged polls in between don't
	// emit a frame, so this read can only return the new content.
	require.Equal(t, "manifest-live-published", readManifest(), "a changed manifest is pushed")
}

// TestServeFrameSocketAppliesManifestUpdate checks the worker side: a Manifest
// control frame from main swaps the worker's manifest holder, so the signer
// picks it up on the next GoP.
func TestServeFrameSocketAppliesManifestUpdate(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "w.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	defer ln.Close()

	holder := newManifestHolder([]byte("prelive"))
	srv := newFrameServer(workerFrameBuffer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go serveFrameSocket(ctx, ln, srv, holder)

	client, err := net.Dial("unix", sock)
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, ingestframe.NewWriter(client).Manifest([]byte("live-published")))

	require.Eventually(t, func() bool {
		return string(holder.get()) == "live-published"
	}, 3*time.Second, 20*time.Millisecond, "worker swaps to the manifest main pushed")
}
