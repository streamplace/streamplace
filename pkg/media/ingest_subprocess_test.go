package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/muxl"
)

// runIngestWorkerHelper is what the test binary becomes when re-exec'd with the
// `ingest-worker` arg (see TestMain). It mirrors makeIngestWorkerCommand exactly:
// config on fd 3, frames on fd 4, fMP4 on stdin; clean run ends with End, a fatal
// error with an Error frame and a non-zero exit.
func runIngestWorkerHelper() int {
	// Test hook: a worker-shaped process that just sleeps (same argv layout as a
	// real worker, so /proc/<pid>/cmdline matches killWorkerPID's safety check).
	// Lets the kill / resume-ban tests act on a real PID without a gst pipeline.
	if len(os.Args) > 3 && os.Args[3] == "__test_sleep__" {
		time.Sleep(10 * time.Minute)
		return 0
	}
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

	// Socket mode (Stage 4): serve frames over the unix socket; media comes from
	// the fd-passed ingest connection (InputFD) when present, else stdin.
	if cfg.SocketPath != "" {
		raw := io.Reader(os.Stdin)
		if cfg.InputFD > 0 {
			f := os.NewFile(uintptr(cfg.InputFD), "ingest-input")
			if f == nil {
				return 2
			}
			defer f.Close()
			raw = f
		}
		if err := ServeMP4IngestWorkerSocket(context.Background(), cfg, WorkerInput(cfg, raw)); err != nil {
			return 1
		}
		return 0
	}

	framesFile := os.NewFile(4, "ingest-frames")
	if framesFile == nil {
		return 2
	}
	defer framesFile.Close()
	frames := ingestframe.NewWriter(framesFile)
	if err := RunMP4IngestWorker(context.Background(), cfg, os.Stdin, frames, func() []byte { return cfg.Manifest }); err != nil {
		_ = frames.Error(err.Error())
		return 1
	}
	_ = frames.End()
	return 0
}

// TestIngestWorkerSubprocess exercises the real process boundary: it spawns the
// worker as an actual subprocess (config over fd 3, fMP4 over stdin, frames over
// fd 4 — the exact wiring MP4IngestIsolated uses) and verifies the worker
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

	mp4 := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))

	exe, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.CommandContext(ctx, exe, "ingest-worker")
	// Quiet gst in the child (it inherits the parent test's verbose leak-tracer env).
	cmd.Env = append(os.Environ(), "GST_DEBUG=0", "GST_TRACERS=")
	cmd.Stdin = bytes.NewReader(mp4)
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

// TestMP4IngestIsolatedWedgeContained is the isolation guarantee: an audio-only
// fMP4 starves the fMP4 muxer's video pad of both data and EOS, so the native
// pipeline wedges with no frames and no EOS — exactly the kind of native
// wedge that would hang (or, with a runaway buffer, OOM-kill) an in-process
// ingest and take the node with it. Run in a worker, it must be contained: the
// watchdog kills the worker and MP4IngestIsolated returns an error, bounded in
// time, with THIS process — the node — still running to assert it.
func TestMP4IngestIsolatedWedgeContained(t *testing.T) {
	old := ingestWorkerWatchdog
	ingestWorkerWatchdog = 6 * time.Second
	defer func() { ingestWorkerWatchdog = old }()

	mm, _ := getStaticTestMediaManager(t)
	ms := newBareSegmentSigner(t)

	wedge := makeAudioOnlyAACFMP4(t, context.Background(), 5)

	start := time.Now()
	err := mm.MP4IngestIsolated(context.Background(), bytes.NewReader(wedge), ms)
	elapsed := time.Since(start)

	require.Error(t, err, "a wedged worker must surface as an error, not a hang")
	require.Less(t, elapsed, 25*time.Second, "watchdog bounded the wedge")
	t.Logf("wedged worker contained in %s: %v", elapsed.Round(time.Second), err)
}

// TestWorkerIngestsFromPassedFD proves the input-ownership mechanism for
// zero-downtime: main fd-passes the (authed) ingest connection to the worker,
// which reads media straight off that fd — main is NOT in the gst pipeline's
// data path — and serves signed segments over its frame socket. Here the passed
// fd is a pipe whose far end the test feeds; in production it's the hijacked push
// connection, so the worker keeps ingesting across a main restart. (HTTP body
// de-framing on a real connection is a separate layer over this mechanism.)
func TestWorkerIngestsFromPassedFD(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)

	sock := filepath.Join(t.TempDir(), "ingest.sock")
	cfgJSON, err := json.Marshal(IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		NodeCertPEM:     ms.Cert,
		NodeKeyPEM:      keyPEM,
		BroadcasterHost: "test.example.com",
		SocketPath:      sock,
		InputFD:         4, // main fd-passes the ingest connection on fd 4
	})
	require.NoError(t, err)

	mp4 := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))

	exe, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.CommandContext(ctx, exe, "ingest-worker")
	cmd.Env = append(os.Environ(), "GST_DEBUG=0", "GST_TRACERS=")
	cmd.Stderr = os.Stderr

	cfgR, cfgW, err := os.Pipe()
	require.NoError(t, err)
	// The fd-passed "connection": the worker reads mediaR (its fd 4); the test
	// feeds mediaW. The worker's gst pipeline reads the fd itself — main is out of
	// the data path entirely.
	mediaR, mediaW, err := os.Pipe()
	require.NoError(t, err)
	cmd.ExtraFiles = []*os.File{cfgR, mediaR} // → child fd 3, fd 4

	require.NoError(t, cmd.Start())
	cfgR.Close()
	mediaR.Close()
	go func() {
		_, _ = cfgW.Write(cfgJSON)
		cfgW.Close()
	}()
	go func() {
		_, _ = mediaW.Write(mp4)
		mediaW.Close()
	}()

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
			break
		}
		switch typ {
		case ingestframe.Segment:
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

	require.NoError(t, cmd.Wait(), "worker subprocess exits cleanly")
	require.GreaterOrEqual(t, segs, 1, "worker ingested from the passed fd and served segments")
	require.True(t, sawEnd, "clean End over the socket")
	t.Logf("worker ingested from passed fd, served %d signed segments + End", segs)
}
