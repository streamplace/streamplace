package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/muxl"
)

// TestWorkerInputDeframes checks the body-deframing the worker applies to the
// fd-passed push connection: prebuf (bytes main read past the headers) is
// prepended, and a chunked transfer-encoding is decoded back to the raw media.
func TestWorkerInputDeframes(t *testing.T) {
	payload := []byte("the-actual-media-bytes-pretend-this-is-mkv-data")
	// A textbook chunked body: one chunk then the zero terminator.
	body := []byte(fmt.Sprintf("%x\r\n%s\r\n0\r\n\r\n", len(payload), payload))

	// prebuf = the slice main already read; the rest is still on the fd.
	cfg := IngestWorkerConfig{Chunked: true, Prebuf: append([]byte(nil), body[:5]...)}
	got, err := io.ReadAll(WorkerInput(cfg, bytes.NewReader(body[5:])))
	require.NoError(t, err)
	require.Equal(t, payload, got, "prebuf + chunked fd de-frames to the original media")

	// Raw (no prebuf, not chunked) passes through unchanged.
	got, err = io.ReadAll(WorkerInput(IngestWorkerConfig{}, bytes.NewReader(payload)))
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

// makeH264AACMKV builds a clean, single-track, streamable H264+AAC MKV from an
// H264+Opus MP4 fixture (video passed through, audio transcoded Opus→AAC). The
// repo's only AAC fixture (sample-stream.mkv) carries four audio tracks, which
// the single-audio ingest pipeline leaves three of unlinked — wedging
// matroskademux with no EOS. This produces exactly the 1-video-1-audio AAC MKV
// the RTMP push path actually delivers.
func makeH264AACMKV(t *testing.T, ctx context.Context, srcMP4 string) []byte {
	t.Helper()
	gstinit.InitGST()
	desc := strings.Join([]string{
		"filesrc location=" + srcMP4 + " ! qtdemux name=d",
		"d. ! queue ! h264parse ! matroskamux name=mux streamable=true ! appsink name=sink",
		"d. ! queue ! opusdec ! audioconvert ! audioresample ! fdkaacenc ! aacparse ! mux.",
	}, "\n")
	pipeline, err := gst.NewPipelineFromString(desc)
	require.NoError(t, err)

	sinkEle, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	var buf bytes.Buffer
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &buf),
	})

	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	require.NoError(t, <-busErr, "remux to H264+AAC MKV")
	require.NotEmpty(t, buf.Bytes(), "remux produced an MKV")
	return buf.Bytes()
}

// TestRunMKVIngestWorkerProducesValidSignedFrames drives the isolated ingest
// worker's core directly (no subprocess): feed it an H264+AAC MKV, collect the
// framed output, and verify every emitted segment is a valid signed canonical
// .m4s. This is the contract the supervisor relies on — frames it can hand
// straight to ValidateMP4. (The real subprocess spawn + fault injection is
// Stage 3.)
func TestRunMKVIngestWorkerProducesValidSignedFrames(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)

	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)

	// Provide the node transcode signer (the same test key serves both roles
	// here, as in the transcoder tests) so the worker completes to dual-codec.
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		NodeCertPEM:     ms.Cert,
		NodeKeyPEM:      keyPEM,
		BroadcasterHost: "test.example.com",
	}

	mkv := makeH264AACMKV(t, ctx, getFixture("5sec.mp4"))

	// All frame writes complete before RunMKVIngestWorker returns (it waits on
	// the signer drain), so reading the buffer single-threaded afterwards is safe.
	var buf bytes.Buffer
	frames := ingestframe.NewWriter(&buf)
	require.NoError(t, RunMKVIngestWorker(ctx, cfg, bytes.NewReader(mkv), frames))

	r := ingestframe.NewReader(&buf)
	var segs int
	for {
		typ, payload, err := r.ReadFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		require.Equal(t, ingestframe.Segment, typ, "worker emits only Segment frames; End is the subcommand's job")
		require.NotEmpty(t, payload)

		out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(payload))
		require.NoError(t, err, "segment %d verify", segs)
		require.NotContains(t, out, `"validation_state":"Invalid"`, "segment %d must validate", segs)

		// With a node key the worker completes to dual-codec: every segment must
		// carry both the source Opus and a worker-transcoded AAC track.
		codecs := audioCodecsOf(t, ctx, payload)
		hasOpus, hasAAC := false, false
		for _, c := range codecs {
			if isOpusCodec(c) {
				hasOpus = true
			}
			if isAACCodec(c) {
				hasAAC = true
			}
		}
		require.True(t, hasOpus, "segment %d keeps source Opus (got %v)", segs, codecs)
		require.True(t, hasAAC, "segment %d gains worker-transcoded AAC (got %v)", segs, codecs)
		segs++
	}
	require.GreaterOrEqual(t, segs, 1, "worker emitted at least one signed dual-codec segment")
	t.Logf("worker emitted %d valid dual-codec segments", segs)
}

// TestRunMKVIngestWorkerRecords proves debug recording works INSIDE the worker:
// with cfg.Record set and a DataDir handed over, the worker tees its ingest
// media to debug-recordings/<did>/<ts>.rtmp.mkv. This is what keeps debug
// recording working on the isolated paths where main is out of the data path
// (it can't tee the bytes itself), so main decides and the worker records.
func TestRunMKVIngestWorkerRecords(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)

	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)

	dataDir := t.TempDir()
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		BroadcasterHost: "test.example.com",
		Record:          true,
		DataDir:         dataDir,
	}

	mkv := makeH264AACMKV(t, ctx, getFixture("5sec.mp4"))

	// Frames are irrelevant here (recording is on the input side); discard them.
	require.NoError(t, RunMKVIngestWorker(ctx, cfg, bytes.NewReader(mkv), ingestframe.NewWriter(io.Discard)))

	// The recording lands at debug-recordings/<sanitized-did>/<ts>.rtmp.mkv and
	// must contain exactly the media the worker ingested. The dump goroutine
	// flushes asynchronously, so allow it a moment to finish the last write.
	glob := filepath.Join(dataDir, "debug-recordings", "*", "*.rtmp.mkv")
	require.Eventually(t, func() bool {
		matches, _ := filepath.Glob(glob)
		if len(matches) != 1 {
			return false
		}
		got, rerr := os.ReadFile(matches[0])
		return rerr == nil && bytes.Equal(got, mkv)
	}, 10*time.Second, 25*time.Millisecond, "worker records the ingest media verbatim")
}
