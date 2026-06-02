package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/muxl"
)

// runIngestWorkerHelper is what the test binary becomes when re-exec'd with the
// `ingest-worker` arg (see TestMain). It mirrors makeIngestWorkerCommand exactly:
// config on fd 3, frames on fd 4, MKV on stdin; clean run ends with End, a fatal
// error with an Error frame and a non-zero exit.
func runIngestWorkerHelper() int {
	cfgFile := os.NewFile(3, "ingest-config")
	if cfgFile == nil {
		return 2
	}
	cfgBytes, err := io.ReadAll(cfgFile)
	cfgFile.Close()
	if err != nil {
		return 2
	}
	var cfg IngestWorkerConfig
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		return 2
	}
	framesFile := os.NewFile(4, "ingest-frames")
	if framesFile == nil {
		return 2
	}
	defer framesFile.Close()
	frames := ingestframe.NewWriter(framesFile)
	if err := RunMKVIngestWorker(context.Background(), cfg, os.Stdin, frames); err != nil {
		_ = frames.Error(err.Error())
		return 1
	}
	_ = frames.End()
	return 0
}

// TestIngestWorkerSubprocess exercises the real process boundary: it spawns the
// worker as an actual subprocess (config over fd 3, MKV over stdin, frames over
// fd 4 — the exact wiring MKVIngestIsolated uses) and verifies the worker
// produces valid signed segments, a clean End frame, and a zero exit. This is
// the part the in-process worker test can't cover: fd passing, the framed wire
// protocol over a real pipe, and process lifecycle.
func TestIngestWorkerSubprocess(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)

	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)
	cfgJSON, err := json.Marshal(IngestWorkerConfig{
		StreamerDID: ms.Streamer(),
		KeyPEM:      keyPEM,
		CertPEM:     ms.Cert,
		Manifest:    manifest,
	})
	require.NoError(t, err)

	mkv := makeH264AACMKV(t, ctx, getFixture("5sec.mp4"))

	exe, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.CommandContext(ctx, exe, "ingest-worker")
	// Quiet gst in the child (it inherits the parent test's verbose leak-tracer env).
	cmd.Env = append(os.Environ(), "GST_DEBUG=0", "GST_TRACERS=")
	cmd.Stdin = bytes.NewReader(mkv)
	cmd.Stderr = os.Stderr

	cfgR, cfgW, err := os.Pipe()
	require.NoError(t, err)
	framesR, framesW, err := os.Pipe()
	require.NoError(t, err)
	cmd.ExtraFiles = []*os.File{cfgR, framesW} // → child fd 3, fd 4

	require.NoError(t, cmd.Start())
	cfgR.Close()
	framesW.Close()
	go func() {
		_, _ = cfgW.Write(cfgJSON)
		cfgW.Close()
	}()

	fr := ingestframe.NewReader(framesR)
	var segs int
	var sawEnd bool
	for {
		typ, payload, rerr := fr.ReadFrame()
		if errors.Is(rerr, io.EOF) {
			break
		}
		require.NoError(t, rerr)
		switch typ {
		case ingestframe.Segment:
			require.False(t, sawEnd, "no segments after End")
			out, verr := muxl.RunMuxlVerify(ctx, bytes.NewReader(payload))
			require.NoError(t, verr, "segment %d verify", segs)
			require.NotContains(t, out, `"validation_state":"Invalid"`, "segment %d must validate", segs)
			segs++
		case ingestframe.End:
			sawEnd = true
		case ingestframe.Error:
			t.Fatalf("worker emitted error frame: %s", payload)
		}
	}
	framesR.Close()

	require.NoError(t, cmd.Wait(), "worker subprocess exits cleanly")
	require.GreaterOrEqual(t, segs, 1, "worker subprocess emitted at least one signed segment")
	require.True(t, sawEnd, "worker subprocess emitted a clean End frame")
	t.Logf("worker subprocess emitted %d valid signed segments + clean End", segs)
}
