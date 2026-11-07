package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/go-gst/go-gst/pkg/gst"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

type RTMPH264Data struct {
	AU  [][]byte
	PTS time.Duration
	DTS time.Duration
}

// RTMPAV1Data is one AV1 temporal unit, as OBUs.
type RTMPAV1Data struct {
	TU  [][]byte
	PTS time.Duration
}

type RTMPAACData struct {
	AU  []byte
	PTS time.Duration
}

type RTMPSession struct {
	EventChan chan any
	// VideoTrack's codec is a *codecs.H264 or a *codecs.AV1.
	VideoTrack *gortmplib.Track
	AudioTrack *gortmplib.Track
	// AV1SequenceHeader is the sequence header OBU of the publisher's AV1
	// sequence start, if it carried one.
	AV1SequenceHeader []byte
	MediaSigner       MediaSigner
}

// VideoCodec is the codec of the session's video track.
func (s *RTMPSession) VideoCodec() VideoCodec {
	if s.VideoTrack != nil {
		if _, ok := s.VideoTrack.Codec.(*codecs.AV1); ok {
			return VideoAV1
		}
	}
	return VideoH264
}

func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner, codec VideoCodec) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}
	pipeline, err := newRTMPIngestPipeline(ctx, rtmpURL, signer, codec, ms.Streamer())
	if err != nil {
		return err
	}
	go mm.HandleKeyRevocation(ctx, ms, pipeline)
	return runRTMPIngestPipeline(ctx, pipeline)
}

// RTMPIngestUnpublished runs RTMPIngest's pipeline for the
// --duplicate-mist-test shadow: the same demux, parse, mux and sign path,
// but every signed segment goes to onSegment instead of being validated
// and published, and no streamer state is watched. It returns once the
// stream has ended and the last signed segment has reached onSegment.
func RTMPIngestUnpublished(ctx context.Context, cli *config.CLI, rtmpURL string, ms MediaSigner, codec VideoCodec, onSegment func(ctx context.Context, segment []byte) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Pipeline cancellation must stop GStreamer before closing the signer's
	// input. Closing it while mp4mux is still writing creates a false mux error.
	signCtx, flush := context.WithCancel(context.WithoutCancel(ctx))
	defer flush()
	var firstErr error
	var once sync.Once
	fail := func(err error) {
		if err != nil {
			once.Do(func() { firstErr = err; cancel() })
		}
	}
	signStream := func(c context.Context, input io.Reader, events chan *muxl.MuxlEvent) error {
		err := ms.SignSegmentStream(c, input, events, false)
		fail(err)
		return err
	}
	sink := func(c context.Context, segment []byte) error {
		err := onSegment(c, segment)
		fail(err)
		return err
	}
	signer, done, err := muxlSignSegmentElem(signCtx, cli, signStream, sink)
	if err != nil {
		return err
	}
	pipeline, err := newRTMPIngestPipeline(ctx, rtmpURL, signer, codec, ms.Streamer())
	if err != nil {
		return err
	}
	err = runRTMPIngestPipeline(ctx, pipeline)
	// GStreamer is now stopped; closing the signer input flushes its final GoP.
	flush()
	<-done
	if firstErr != nil {
		return firstErr
	}
	return err
}

// newRTMPIngestPipeline pulls rtmpURL with rtmp2src and feeds its video
// track, in codec, and its AAC track into the signer element.
func newRTMPIngestPipeline(ctx context.Context, rtmpURL string, signer gst.Element, codec VideoCodec, streamer string) (gst.Pipeline, error) {
	// Mint the source audio: RTMP/FLV audio is already AAC, so pass it through
	// (aacparse) rather than transcoding to Opus. The validate path completes
	// each segment to also carry Opus when a consumer (WebRTC) needs it — so
	// the old RTMP-AAC→Opus→HLS-AAC double-transcode is gone.
	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
		"demux.audio ! queue ! aacparse name=audioenc",
		"demux.video ! queue ! " + codec.ParserLaunch("parse", ""),
	}
	pipelineElem, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}
	pipeline, ok := pipelineElem.(gst.Pipeline)
	if !ok {
		return nil, fmt.Errorf("error creating RTMPIngest pipeline: not a pipeline")
	}

	parseEle := pipeline.GetByName("parse")
	if parseEle == nil {
		return nil, fmt.Errorf("failed to get parse element")
	}
	// Only a frame a decoder can start on starts a segment (see installKeyframeProbe).
	installKeyframeProbe(ctx, codec, parseEle.GetStaticPad("src"), streamer)

	if !pipeline.Add(signer) {
		return nil, fmt.Errorf("failed to add signer to pipeline")
	}
	if !parseEle.Link(signer) {
		return nil, fmt.Errorf("failed to link parse to signer")
	}
	audioenc := pipeline.GetByName("audioenc")
	if audioenc == nil {
		return nil, fmt.Errorf("failed to get audioenc element")
	}
	if !audioenc.Link(signer) {
		return nil, fmt.Errorf("failed to link audioenc to signer")
	}
	return pipeline, nil
}

// runRTMPIngestPipeline plays pipeline until it ends or ctx does.
func runRTMPIngestPipeline(ctx context.Context, pipeline gst.Pipeline) error {
	busErr := make(chan error, 1)
	go func() {
		busErr <- HandleBusMessages(ctx, pipeline)
	}()

	if ret := pipeline.SetState(gst.StatePlaying); ret == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline to playing state")
	}

	defer func() {
		if ret := pipeline.SetState(gst.StateNull); ret == gst.StateChangeFailure {
			log.Error(ctx, "error setting pipeline to null state")
		}
	}()

	return <-busErr
}
