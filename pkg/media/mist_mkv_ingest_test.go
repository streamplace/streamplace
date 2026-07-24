package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/test/remote"
)

// runMKVThroughIngestWorker feeds an MKV byte stream through the isolated
// ingest worker and returns how many signed canonical segments it emitted. The
// worker watchdog is shortened so a wedged pipeline returns promptly instead of
// hanging the test (override via SP_TEST_WATCHDOG to e.g. park a wedge for a
// stack dump). With transcode=true the node keys are supplied so the worker
// completes to dual-codec, as production does.
func runMKVThroughIngestWorker(t *testing.T, mkv []byte, transcode bool) (int, error) {
	segs, err := runMKVThroughIngestWorkerSegments(t, mkv, transcode)
	return len(segs), err
}

// runMKVThroughIngestWorkerSegments is runMKVThroughIngestWorker returning the
// raw segment payloads, for tests that validate the emitted segments rather
// than just count them.
func runMKVThroughIngestWorkerSegments(t *testing.T, mkv []byte, transcode bool) ([][]byte, error) {
	t.Helper()
	old := ingestWorkerWatchdog
	ingestWorkerWatchdog = 10 * time.Second
	if wd := os.Getenv("SP_TEST_WATCHDOG"); wd != "" {
		d, perr := time.ParseDuration(wd)
		require.NoError(t, perr)
		ingestWorkerWatchdog = d
	}
	defer func() { ingestWorkerWatchdog = old }()

	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		BroadcasterHost: "test.example.com",
	}
	if transcode {
		cfg.NodeCertPEM = ms.Cert
		cfg.NodeKeyPEM = keyPEM
	}

	var buf bytes.Buffer
	runErr := RunMKVIngestWorker(ctx, cfg, bytes.NewReader(mkv), ingestframe.NewWriter(&buf), func() []byte { return cfg.Manifest })

	r := ingestframe.NewReader(&buf)
	var segs [][]byte
	for {
		typ, payload, rerr := r.ReadFrame()
		if errors.Is(rerr, io.EOF) {
			break
		}
		require.NoError(t, rerr)
		if typ == ingestframe.Segment {
			segs = append(segs, payload)
		}
	}
	return segs, runErr
}

// makeSparseVideoAACMKV synthesizes the stream shape that wedged production
// ingest: video that degrades to keyframe-only at a low rate (here 0.5fps —
// MistServer drops all delta frames when a push falls behind, leaving ~1s-apart
// keyframes) alongside continuous 48kHz AAC audio, in a streamable MKV. The
// audio branch must buffer a full video-frame gap while matroskademux walks to
// the next video frame; gst's default 1s-capped queue can't, and the
// aggregator-based fMP4 muxer deadlocks (see buildMKVIngestPipeline).
func makeSparseVideoAACMKV(t *testing.T, ctx context.Context, seconds int) []byte {
	t.Helper()
	gstinit.InitGST()
	desc := strings.Join([]string{
		fmt.Sprintf("videotestsrc num-buffers=%d ! video/x-raw,width=320,height=240,framerate=1/2 ! x264enc key-int-max=1 tune=zerolatency speed-preset=ultrafast ! h264parse ! matroskamux name=mux streamable=true ! appsink name=sink", (seconds+1)/2),
		fmt.Sprintf("audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.", seconds*47),
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
	require.NoError(t, <-busErr, "synthesize sparse-video MKV")
	require.NotEmpty(t, buf.Bytes())
	return buf.Bytes()
}

// TestMKVIngestSparseVideoNoWedge is the regression test for a production
// ingest wedge: a stream whose video goes sparse (keyframe-only, ≥1s between
// video frames) deadlocked the MKV ingest pipeline — audio backpressure
// through the default 1s-capped queue blocked the demux, the fMP4 aggregator
// starved on its video pad, and the stream hung with no EOS until the
// watchdog killed it. With Queue2Big on both ingest branches the same stream
// must segment to completion.
func TestMKVIngestSparseVideoNoWedge(t *testing.T) {
	ctx := context.Background()
	mkv := makeSparseVideoAACMKV(t, ctx, 12)

	segs, err := runMKVThroughIngestWorker(t, mkv, false)
	require.NoError(t, err, "sparse-video stream ingests cleanly (wedge → watchdog → context canceled)")
	// 12s of 2s-apart keyframes ≈ 6 GoPs; wedging yields 0-1 segments.
	require.GreaterOrEqual(t, segs, 4, "sparse-video stream emits its segments")
	t.Logf("sparse-video stream: %d segments", segs)
}

// The nyc-* fixtures are cuts of a real 164s production MistServer MKV push
// whose video degrades to keyframe-only at ~140s (behind-push frame-drop) —
// the capture that wedged production ingest. head = first ~10s; head-nojson =
// the same bytes with the 30-byte M_JSON TrackEntry stripped; tail135 = from
// 135s (~5s before the keyframe-only transition); full = the whole capture.

// makeDualVideoAACMKV synthesizes an MKV with TWO H264 video tracks plus 48kHz
// AAC — the shape a multitrack-video (enhanced broadcasting) stream takes once
// MistServer repackages it for the node's MKV push.
func makeDualVideoAACMKV(t *testing.T, ctx context.Context, seconds int) []byte {
	t.Helper()
	gstinit.InitGST()
	desc := strings.Join([]string{
		"matroskamux name=mux streamable=true ! appsink name=sink",
		fmt.Sprintf("videotestsrc num-buffers=%d pattern=smpte ! video/x-raw,width=640,height=360,framerate=30/1 ! x264enc key-int-max=30 tune=zerolatency speed-preset=ultrafast ! h264parse ! mux.", seconds*30),
		fmt.Sprintf("videotestsrc num-buffers=%d pattern=ball ! video/x-raw,width=320,height=180,framerate=30/1 ! x264enc key-int-max=30 tune=zerolatency speed-preset=ultrafast ! h264parse ! mux.", seconds*30),
		fmt.Sprintf("audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.", seconds*47),
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
	require.NoError(t, <-busErr, "synthesize dual-video MKV")
	require.NotEmpty(t, buf.Bytes())
	return buf.Bytes()
}

// TestMKVIngestDualVideoTrack probes what today's single-video-branch ingest
// pipeline does with a multitrack-video (enhanced broadcasting) MKV. Until
// multitrack ingest is actually built, a dual-track push must at least not
// wedge or kill the session: the second video track should be ignored and the
// first one segmented like any other stream.
func TestMKVIngestDualVideoTrack(t *testing.T) {
	ctx := context.Background()
	mkv := makeDualVideoAACMKV(t, ctx, 10)

	segs, err := runMKVThroughIngestWorker(t, mkv, true)
	require.NoError(t, err, "dual-video-track stream ingests without a pipeline error")
	require.GreaterOrEqual(t, segs, 5, "dual-video-track stream emits its ~1s GoP segments")
	t.Logf("dual-video-track stream: %d segments", segs)
}


// TestMKVIngestMistMetadataTrack: MistServer's MKV push declares a
// live-metadata track (CodecID M_JSON, TrackType 3) as track 1, ahead of the
// AAC audio and H264 video tracks. It was the initial suspect for the
// production wedge but proved benign — matroskademux ignores the unknown
// codec, and the same media segments identically with the 30-byte M_JSON
// TrackEntry stripped (the control). Kept as a canary for the MistServer
// track layout. (The real wedge: TestMKVIngestSparseVideoNoWedge.)
func TestMKVIngestMistMetadataTrack(t *testing.T) {
	control, err := os.ReadFile(remote.RemoteFixture("3284ef5658e7864bce326c296a909e985c4167d0b9a445b2ce944c2f0171c71e/nyc-head-nojson.mkv"))
	require.NoError(t, err)
	mist, err := os.ReadFile(remote.RemoteFixture("c0989e044f3350c55f1e129b76252bfb2859914058bb17d2431a605db9693467/nyc-head.mkv"))
	require.NoError(t, err)

	segs, err := runMKVThroughIngestWorker(t, control, false)
	require.NoError(t, err, "control (M_JSON TrackEntry stripped) ingests cleanly")
	require.GreaterOrEqual(t, segs, 1, "control emits segments")
	t.Logf("control: %d segments", segs)

	segs, err = runMKVThroughIngestWorker(t, mist, false)
	require.NoError(t, err, "MistServer MKV (with M_JSON track) ingests cleanly")
	require.GreaterOrEqual(t, segs, 1, "MistServer MKV emits segments")
	t.Logf("with M_JSON track: %d segments", segs)
}

// TestMKVIngestMistFullSample runs the entire 164s production capture through
// the worker with node transcode keys — the closest in-process approximation
// of the production ingest. The capture degrades to keyframe-only video at
// ~140s (MistServer behind-push frame-drop), which is what wedged production;
// with Queue2Big on the ingest branches the whole capture must segment.
func TestMKVIngestMistFullSample(t *testing.T) {
	// This once "progressively slowed until the watchdog fired, then hung in
	// the post-cancel drain" and was skip-gated as known-hanging — that was
	// the muxl-event-drain-vs-cancel deadlock (see muxlSignSegmentElem's
	// drainCtx); with the drain non-cancellable the full capture transcodes
	// at full speed (~13s).
	mkv, err := os.ReadFile(remote.RemoteFixture("3e4e5d9758e67053908e523379a3e2ef2cf60679d0657a940daf96590e866015/nyc-full.mkv"))
	require.NoError(t, err)
	segs, err := runMKVThroughIngestWorker(t, mkv, true)
	t.Logf("full sample: %d segments, err=%v", segs, err)
	require.NoError(t, err, "full production sample ingests cleanly")
	// 171 GoPs in the capture (~1s each); wedging yielded ~144.
	require.GreaterOrEqual(t, segs, 160, "full sample emits the whole stream's segments")
}

// TestMKVIngestMistTail is the fast sample-based wedge check: the tail sample
// starts at 135s, ~5s before the capture goes keyframe-only, so an unfixed
// pipeline wedges within seconds (4 segments) instead of minutes.
func TestMKVIngestMistTail(t *testing.T) {
	mkv, err := os.ReadFile(remote.RemoteFixture("03df698a342f1ab89dccc20ce0a0283e1270104e6382703575686c9f4a88881e/nyc-tail135.mkv"))
	require.NoError(t, err)
	segs, err := runMKVThroughIngestWorker(t, mkv, false)
	t.Logf("tail sample: %d segments, err=%v", segs, err)
	require.NoError(t, err, "tail of production sample ingests cleanly")
	// 135s..164s at ~1s GoPs ≈ 29 segments; wedging yields ~5.
	require.GreaterOrEqual(t, segs, 20, "tail sample emits segments past the keyframe-only transition")
}

// makeBFrameAACMKV synthesizes an H264 stream WITH B-frames (PTS ≠ DTS)
// alongside AAC audio in a streamable MKV — the shape a hardware encoder or
// non-zerolatency x264 push produces. Matroska blocks carry only presentation
// timestamps, so on demux the reordered video arrives with dts=none; without
// DTS reconstruction the fMP4 muxer stretches the video track and every
// segment fails validation downstream (see buildMKVIngestPipeline's
// h264timestamper). b-adapt=false forces x264 to actually emit the configured
// B-frames rather than deciding per-scene.
func makeBFrameAACMKV(t *testing.T, ctx context.Context, seconds int) []byte {
	t.Helper()
	gstinit.InitGST()
	desc := strings.Join([]string{
		fmt.Sprintf("videotestsrc num-buffers=%d ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc bframes=2 b-adapt=false key-int-max=30 speed-preset=veryfast ! h264parse ! matroskamux name=mux streamable=true ! appsink name=sink", seconds*30),
		fmt.Sprintf("audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.", seconds*47),
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
	require.NoError(t, <-busErr, "synthesize B-frame MKV")
	require.NotEmpty(t, buf.Bytes())
	return buf.Bytes()
}

// TestMKVIngestBFramesValidate is the regression test for B-frame MKV ingest:
// every emitted segment must survive the full ValidateMP4Media chokepoint.
// Before DTS reconstruction, mp4mux treated the reordered (dts=none) B-frame
// PTS as monotonic timing, stretching the video track ~2.2×; the segments
// LOOKED fine (both tracks present, signatures valid) but the video/audio
// duration mismatch made push-mode qtdemux EOS the audio pad before the audio
// bytes arrived — muxl's flat wrap is non-interleaved, video first — and every
// segment was rejected with "no audio in segment".
func TestMKVIngestBFramesValidate(t *testing.T) {
	ctx := context.Background()
	mkv := makeBFrameAACMKV(t, ctx, 8)

	segs, err := runMKVThroughIngestWorkerSegments(t, mkv, false)
	require.NoError(t, err, "B-frame stream ingests cleanly")
	require.GreaterOrEqual(t, len(segs), 6, "B-frame stream emits its segments")

	sawBFrames := false
	for i, seg := range segs {
		res, verr := ValidateMP4Media(ctx, seg)
		require.NoError(t, verr, "segment %d validates (video+audio both present)", i)
		dur := time.Duration(res.MediaData.Duration)
		require.Greater(t, dur, 500*time.Millisecond, "segment %d duration sane", i)
		require.Less(t, dur, 2*time.Second, "segment %d duration not stretched", i)
		if res.MediaData.Video[0].BFrames {
			sawBFrames = true
		}
	}
	require.True(t, sawBFrames, "synthesized stream actually contains B-frames — if this fails the test no longer exercises the reorder path")
	t.Logf("B-frame stream: %d segments, all validated", len(segs))
}

// TestMKVIngestMistFullSampleNoTranscode is TestMKVIngestMistFullSample
// without node keys (segment+sign only). During diagnosis this proved the
// wedge was in the core ingest pipeline, not the dual-codec transcode stage —
// both variants wedged at the same GoP.
func TestMKVIngestMistFullSampleNoTranscode(t *testing.T) {
	mkv, err := os.ReadFile(remote.RemoteFixture("3e4e5d9758e67053908e523379a3e2ef2cf60679d0657a940daf96590e866015/nyc-full.mkv"))
	require.NoError(t, err)
	segs, err := runMKVThroughIngestWorker(t, mkv, false)
	t.Logf("full sample (no transcode): %d segments, err=%v", segs, err)
	require.NoError(t, err, "full production sample ingests cleanly without transcode")
	require.GreaterOrEqual(t, segs, 70, "full sample emits the whole stream's segments")
}
