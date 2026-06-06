package media

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/muxl"
)

// TestWorkerServesFramesOverSocket drives the zero-downtime transport end-to-end
// with a REAL ingest: ServeMKVIngestWorkerSocket runs the full mux+sign+transcode
// pipeline and serves the resulting signed dual-codec segments over a unix
// socket; a client connects and reads them through to a clean End. This proves
// the socket path carries real signed media (the frameServer reconnect tests
// cover the buffer-across-restart behavior deterministically).
func TestWorkerServesFramesOverSocket(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)

	sock := filepath.Join(t.TempDir(), "ingest.sock")
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		NodeCertPEM:     ms.Cert,
		NodeKeyPEM:      keyPEM,
		BroadcasterHost: "test.example.com",
		SocketPath:      sock,
	}

	mkv := makeH264AACMKV(t, ctx, getFixture("5sec.mp4"))

	serveDone := make(chan error, 1)
	go func() { serveDone <- ServeMKVIngestWorkerSocket(ctx, cfg, bytes.NewReader(mkv)) }()

	// Connect once the worker's listener is up (retry the dial briefly).
	var conn net.Conn
	for i := 0; i < 100; i++ {
		if conn, err = net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err, "connect to worker frame socket")
	defer conn.Close()

	r := ingestframe.NewReader(conn)
	var segs int
	var sawEnd bool
	for {
		typ, payload, rerr := r.ReadFrame()
		if rerr != nil {
			break // EOF after the worker exits
		}
		switch typ {
		case ingestframe.Segment:
			require.False(t, sawEnd, "no segments after End")
			out, verr := muxl.RunMuxlVerify(ctx, bytes.NewReader(payload))
			require.NoError(t, verr, "segment %d verify", segs)
			require.NotContains(t, out, `"validation_state":"Invalid"`, "segment %d valid", segs)
			segs++
		case ingestframe.End:
			sawEnd = true
		case ingestframe.Error:
			t.Fatalf("worker error frame: %s", payload)
		}
	}

	require.GreaterOrEqual(t, segs, 1, "worker served at least one signed segment over the socket")
	require.True(t, sawEnd, "worker served a clean End over the socket")

	select {
	case serveErr := <-serveDone:
		require.NoError(t, serveErr)
	case <-time.After(30 * time.Second):
		t.Fatal("ServeMKVIngestWorkerSocket did not return after the stream drained")
	}
	t.Logf("worker served %d signed segments + End over the socket", segs)
}
