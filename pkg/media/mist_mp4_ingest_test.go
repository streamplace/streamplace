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
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/test/remote"
)

// runMP4ThroughIngestWorker feeds a fragmented-MP4 byte stream through the
// isolated ingest worker and returns how many signed canonical segments it
// emitted. The worker watchdog is shortened so a wedged pipeline returns
// promptly instead of hanging the test (override via SP_TEST_WATCHDOG to e.g.
// park a wedge for a stack dump). With transcode=true the node keys are
// supplied so the worker completes to dual-codec, as production does.
func runMP4ThroughIngestWorker(t *testing.T, mp4 []byte, transcode bool) (int, error) {
	segs, err := runMP4ThroughIngestWorkerSegments(t, mp4, transcode)
	return len(segs), err
}

// runMP4ThroughIngestWorkerSegments is runMP4ThroughIngestWorker returning the
// raw segment payloads, for tests that validate the emitted segments rather
// than just count them.
func runMP4ThroughIngestWorkerSegments(t *testing.T, mp4 []byte, transcode bool) ([][]byte, error) {
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
	runErr := RunMP4IngestWorker(ctx, cfg, bytes.NewReader(mp4), ingestframe.NewWriter(&buf), func() []byte { return cfg.Manifest })

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

// runSynthPipeline runs a gst-launch description whose sink is an appsink
// named "sink" and returns everything the sink produced.
func runSynthPipeline(t *testing.T, ctx context.Context, desc string) []byte {
	t.Helper()
	gstinit.InitGST()
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
	require.NoError(t, <-busErr, "synthesize test stream")
	require.NotEmpty(t, buf.Bytes())
	return buf.Bytes()
}

// makeSparseVideoAACFMP4 synthesizes the stream shape that wedged production
// ingest back when the bridge format was MKV: video that degrades to
// keyframe-only at a low rate (here 0.5fps — MistServer drops all delta frames
// when a push falls behind, leaving ~1s-apart keyframes) alongside continuous
// 48kHz AAC audio, in a fragmented MP4. The wedge mechanism is
// demux-agnostic — the audio branch must buffer a full video-frame gap while
// the demux walks the byte stream to the next video frame, and the
// aggregator-based fMP4 muxer downstream consumes nothing until every pad has
// data — so the regression carries over to the qtdemux pipeline (see
// buildMP4IngestPipeline's Queue2Big comment).
func makeSparseVideoAACFMP4(t *testing.T, ctx context.Context, seconds int) []byte {
	t.Helper()
	desc := strings.Join([]string{
		fmt.Sprintf("videotestsrc num-buffers=%d ! video/x-raw,width=320,height=240,framerate=1/2 ! x264enc key-int-max=1 tune=zerolatency speed-preset=ultrafast ! h264parse ! mp4mux name=mux fragment-duration=500 ! appsink name=sink", (seconds+1)/2),
		fmt.Sprintf("audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.", seconds*47),
	}, "\n")
	return runSynthPipeline(t, ctx, desc)
}

// TestMP4IngestSparseVideoNoWedge is the regression test for a production
// ingest wedge: a stream whose video goes sparse (keyframe-only, ≥1s between
// video frames) deadlocked the ingest pipeline — audio backpressure through
// the default 1s-capped queue blocked the demux, the fMP4 aggregator starved
// on its video pad, and the stream hung with no EOS until the watchdog killed
// it. With Queue2Big on both ingest branches the same stream must segment to
// completion.
func TestMP4IngestSparseVideoNoWedge(t *testing.T) {
	ctx := context.Background()
	mp4 := makeSparseVideoAACFMP4(t, ctx, 12)

	segs, err := runMP4ThroughIngestWorker(t, mp4, false)
	require.NoError(t, err, "sparse-video stream ingests cleanly (wedge → watchdog → context canceled)")
	// 12s of 2s-apart keyframes ≈ 6 GoPs; wedging yields 0-1 segments.
	require.GreaterOrEqual(t, segs, 4, "sparse-video stream emits its segments")
	t.Logf("sparse-video stream: %d segments", segs)
}

// videoPTSDTSOffsets flat-wraps a canonical segment and returns every video
// sample's PTS−DTS composition offset. This is the direct probe for the class
// of bug that motivated the fMP4 ingest rewrite: MKV ingest reconstructed DTS
// with h264timestamper, whose SPS fallback minted a constant spurious offset
// for streams that declare no reorder window (VideoToolbox) — pushing every
// GoP's presentation past its segment's declared window and breaking WebRTC
// playback at each keyframe. fMP4 ingest reads the container's real DTS, so a
// no-reorder stream must come out with PTS == DTS on every sample.
func videoPTSDTSOffsets(t *testing.T, ctx context.Context, segment []byte) []time.Duration {
	t.Helper()
	gstinit.InitGST()
	var flat bytes.Buffer
	require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(segment), "flat", &flat))

	desc := strings.Join([]string{
		"appsrc name=src ! qtdemux name=demux",
		"demux.video_0 ! queue ! h264parse ! appsink name=sink sync=false",
		"demux.audio_0 ! queue ! fakesink sync=false",
	}, "\n")
	pipeline, err := gst.NewPipelineFromString(desc)
	require.NoError(t, err)

	srcEle, err := pipeline.GetElementByName("src")
	require.NoError(t, err)
	app.SrcFromElement(srcEle).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, bytes.NewReader(flat.Bytes())),
	})

	sinkEle, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	var offsets []time.Duration
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			buf := sample.GetBuffer()
			pts, dts := buf.PresentationTimestamp(), buf.DecodingTimestamp()
			if pts != gst.ClockTimeNone && dts != gst.ClockTimeNone {
				offsets = append(offsets, time.Duration(int64(pts)-int64(dts)))
			}
			return gst.FlowOK
		},
	})

	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	require.NoError(t, <-busErr, "demux flat-wrapped segment")
	require.NotEmpty(t, offsets, "segment has video samples with timestamps")
	return offsets
}

// TestMP4IngestMistRealSample runs a real MistServer live-MP4 capture through
// the worker: a VideoToolbox (macOS hardware encoder) 720p H264 + AAC RTMP
// push, pulled from Mist's HTTP .mp4 output — exactly what production ingest
// consumes since the MKV→fMP4 bridge rewrite. VideoToolbox is the interesting
// encoder here because its SPS declares no reorder window, which is what sent
// the old MKV path's h264timestamper into its spurious-offset fallback; this
// stream must instead come through with its real timestamps: PTS == DTS on
// every video sample of every signed segment.
func TestMP4IngestMistRealSample(t *testing.T) {
	ctx := context.Background()
	mp4, err := os.ReadFile(remote.RemoteFixture("ee4d7f8f9b267ba8229314162ef268186048f91ac4242b13fce3f5ee955b97ae/mist-vt-720p.mp4"))
	require.NoError(t, err)

	segs, err := runMP4ThroughIngestWorkerSegments(t, mp4, false)
	require.NoError(t, err, "real Mist fMP4 capture ingests cleanly")
	require.GreaterOrEqual(t, len(segs), 3, "capture emits its segments")

	for i, seg := range segs {
		res, verr := ValidateMP4Media(ctx, seg)
		require.NoError(t, verr, "segment %d validates (video+audio both present)", i)
		dur := time.Duration(res.MediaData.Duration)
		require.Greater(t, dur, 200*time.Millisecond, "segment %d duration sane", i)
		require.Less(t, dur, 6*time.Second, "segment %d duration not stretched", i)
		require.False(t, res.MediaData.Video[0].BFrames, "VideoToolbox capture has no B-frames")
		for _, off := range videoPTSDTSOffsets(t, ctx, seg) {
			require.Equal(t, time.Duration(0), off, "segment %d: no-reorder stream must keep PTS == DTS — a nonzero offset means ingest invented a reorder delay", i)
		}
	}
	t.Logf("real Mist capture: %d segments, all validated with PTS == DTS", len(segs))
}

// makeBFrameAACFMP4 synthesizes an H264 stream WITH B-frames (PTS ≠ DTS)
// alongside AAC audio in a fragmented MP4 — the shape a hardware encoder or
// non-zerolatency x264 push produces, as delivered by MistServer's live .mp4
// output. Unlike Matroska, MP4 track fragments carry real decode timestamps,
// so the ingest pipeline needs no DTS reconstruction for the fMP4 muxer to mux
// the reordered stream correctly. b-adapt=false forces x264 to actually emit
// the configured B-frames rather than deciding per-scene.
func makeBFrameAACFMP4(t *testing.T, ctx context.Context, seconds int) []byte {
	t.Helper()
	desc := strings.Join([]string{
		fmt.Sprintf("videotestsrc num-buffers=%d ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc bframes=2 b-adapt=false key-int-max=30 speed-preset=veryfast ! h264parse ! mp4mux name=mux fragment-duration=500 ! appsink name=sink", seconds*30),
		fmt.Sprintf("audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.", seconds*47),
	}, "\n")
	return runSynthPipeline(t, ctx, desc)
}

// TestMP4IngestBFramesValidate is the regression test for B-frame ingest:
// every emitted segment must survive the full ValidateMP4Media chokepoint with
// a sane duration, and the stream's real reorder offsets must be preserved.
// (On the old MKV path this scenario originally lost DTS entirely — mp4mux
// stretched the video track ~2.2× and every segment failed validation with
// "no audio in segment"; the h264timestamper fix for that in turn minted
// spurious offsets on no-reorder streams. fMP4's container DTS sidesteps the
// whole trade-off, and this test pins the B-frame half of it.)
func TestMP4IngestBFramesValidate(t *testing.T) {
	ctx := context.Background()
	mp4 := makeBFrameAACFMP4(t, ctx, 8)

	segs, err := runMP4ThroughIngestWorkerSegments(t, mp4, false)
	require.NoError(t, err, "B-frame stream ingests cleanly")
	require.GreaterOrEqual(t, len(segs), 6, "B-frame stream emits its segments")

	sawBFrames := false
	sawReorderOffset := false
	for i, seg := range segs {
		res, verr := ValidateMP4Media(ctx, seg)
		require.NoError(t, verr, "segment %d validates (video+audio both present)", i)
		dur := time.Duration(res.MediaData.Duration)
		require.Greater(t, dur, 500*time.Millisecond, "segment %d duration sane", i)
		require.Less(t, dur, 2*time.Second, "segment %d duration not stretched", i)
		if res.MediaData.Video[0].BFrames {
			sawBFrames = true
		}
		for _, off := range videoPTSDTSOffsets(t, ctx, seg) {
			if off > 0 {
				sawReorderOffset = true
			}
		}
	}
	require.True(t, sawBFrames, "synthesized stream actually contains B-frames — if this fails the test no longer exercises the reorder path")
	require.True(t, sawReorderOffset, "B-frame stream keeps its real PTS−DTS reorder offsets through ingest")
	t.Logf("B-frame stream: %d segments, all validated", len(segs))
}

// TestMP4IngestRejectsMatroskaWithDiagnosis: an MKV stream landing on the
// fMP4 ingest (a MistServer still running the legacy MKVExec process config)
// must fail immediately with a message that names the actual problem — not a
// generic qtdemux parse error on an endless Mist-side restart loop.
func TestMP4IngestRejectsMatroskaWithDiagnosis(t *testing.T) {
	mkvish := append(append([]byte{}, matroskaMagic...), make([]byte, 1024)...)
	_, err := runMP4ThroughIngestWorker(t, mkvish, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Matroska")
	require.Contains(t, err.Error(), "MKVExec")
}
