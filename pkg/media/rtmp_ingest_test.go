package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
)

func makeLiveH264OpusFMP4(t *testing.T, ctx context.Context) []byte {
	return makeLiveH264OpusFMP4WithGOP(t, ctx, 15)
}

func makeLiveH264OpusFMP4WithGOP(t *testing.T, ctx context.Context, gopFrames int) []byte {
	t.Helper()
	require.Positive(t, gopFrames)
	// Keep the fixture aligned with the configured live segment target: one video
	// GOP is the source-age budget that the first-RTP test measures.
	gstinit.InitGST()
	desc := strings.Join([]string{
		"mp4mux name=mux fragment-duration=500 ! appsink name=sink",
		fmt.Sprintf("videotestsrc num-buffers=%d ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency speed-preset=ultrafast key-int-max=%d ! h264parse ! mux.video_0", gopFrames, gopFrames),
		"audiotestsrc num-buffers=33 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! opusenc ! mux.audio_0",
	}, "\n")
	pipeline, err := gst.NewPipelineFromString(desc)
	require.NoError(t, err)

	sinkEle, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	var output bytes.Buffer
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &output),
	})

	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	require.NoError(t, <-busErr, "generate a live-shaped fragmented H264+Opus MP4")
	require.NotEmpty(t, output.Bytes(), "generated an fMP4")
	return output.Bytes()
}

func TestRTMPIngestAudioChainPreservesAAC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	input := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))
	path := filepath.Join(t.TempDir(), "source.mp4")
	require.NoError(t, os.WriteFile(path, input, 0600))

	gstinit.InitGST()
	desc := strings.Join([]string{
		"filesrc location=" + path + " ! qtdemux name=demux",
		rtmpIngestAudioChain("demux.") + " ! appsink name=sink sync=false",
	}, "\n")
	pipeline, err := gst.NewPipelineFromString(desc)
	require.NoError(t, err)

	sinkEle, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	var samples int
	var capsName string
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			if caps := sample.GetCaps(); caps != nil && caps.GetStructureAt(0) != nil {
				capsName = caps.GetStructureAt(0).Name()
			}
			samples++
			return gst.FlowOK
		},
	})

	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	require.NoError(t, <-busErr)
	require.Positive(t, samples, "RTMP's AAC input chain should produce audio")
	require.Equal(t, "audio/mpeg", capsName,
		"RTMP ingest should preserve its AAC source")
}

func TestRTMPSourceAudioCodecIdentifiesOpus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ms := newBareSegmentSigner(t)
	segments := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segments)

	codec, err := rtmpSourceAudioCodec(ctx, segments[0])
	require.NoError(t, err)
	require.Equal(t, "opus", codec)
}

func TestRTMPSourceAudioCodecIdentifiesAAC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ms := newBareSegmentSigner(t)
	input := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))
	path := filepath.Join(t.TempDir(), "source.mp4")
	require.NoError(t, os.WriteFile(path, input, 0600))
	segments := allSignedBareSegments(t, ctx, ms, path)
	require.NotEmpty(t, segments)

	codec, err := rtmpSourceAudioCodec(ctx, segments[0])
	require.NoError(t, err)
	require.Equal(t, "aac", codec)
}

func TestRTMPAudioChainSupportsSourceCodecs(t *testing.T) {
	require.Contains(t, rtmpAudioChain("aac"), "aacparse")
	require.NotContains(t, rtmpAudioChain("aac"), "opusdec")
	require.Contains(t, rtmpAudioChain("opus"), "opusparse")
	require.Contains(t, rtmpAudioChain("opus"), "fdkaacenc")
}

func TestRTMPIngestAudioChainExposesNamedParser(t *testing.T) {
	gstinit.InitGST()
	pipeline, err := gst.NewPipelineFromString(rtmpIngestAudioChain("fakesrc"))
	require.NoError(t, err)

	_, err = pipeline.GetElementByName(rtmpIngestAudioElementName)
	require.NoError(t, err)
}

func TestFilterSegmentToCodecRejectsMissingTrack(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ms := newBareSegmentSigner(t)
	aacInput := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))
	aacPath := filepath.Join(t.TempDir(), "aac-source.mp4")
	require.NoError(t, os.WriteFile(aacPath, aacInput, 0600))
	aacSegments := allSignedBareSegments(t, ctx, ms, aacPath)
	require.NotEmpty(t, aacSegments)

	_, err := filterSegmentToCodec(ctx, aacSegments[0], true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no Opus audio track")
}

func TestFilterSegmentToCodecStrictRejectsCodecMismatch(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	input := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))
	path := filepath.Join(t.TempDir(), "source.mp4")
	require.NoError(t, os.WriteFile(path, input, 0600))
	segments := allSignedBareSegments(t, ctx, ms, path)
	require.NotEmpty(t, segments)

	_, err := filterSegmentToCodecStrict(ctx, segments[0], true)
	require.ErrorContains(t, err, "requested opus audio track is missing")
}
