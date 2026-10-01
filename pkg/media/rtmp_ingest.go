package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

type RTMPH264Data struct {
	AU  [][]byte
	PTS time.Duration
	DTS time.Duration
}

type RTMPAACData struct {
	AU  []byte
	PTS time.Duration
}

type RTMPSession struct {
	EventChan   chan any
	VideoTrack  *format.H264
	AudioTrack  *format.MPEG4Audio
	MediaSigner MediaSigner
}

func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}
	pipeline, err := newRTMPIngestPipeline(rtmpURL, signer)
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
func RTMPIngestUnpublished(ctx context.Context, cli *config.CLI, rtmpURL string, ms MediaSigner, onSegment func(ctx context.Context, segment []byte) error) error {
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
		err := ms.SignSegmentStream(c, input, events)
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
	pipeline, err := newRTMPIngestPipeline(rtmpURL, signer)
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

// newRTMPIngestPipeline pulls rtmpURL with rtmp2src and feeds its H.264 and
// AAC tracks into the signer element.
func newRTMPIngestPipeline(rtmpURL string, signer *gst.Element) (*gst.Pipeline, error) {
	// Mint the source audio: RTMP/FLV audio is already AAC, so pass it through
	// (aacparse) rather than transcoding to Opus. The validate path completes
	// each segment to also carry Opus when a consumer (WebRTC) needs it — so
	// the old RTMP-AAC→Opus→HLS-AAC double-transcode is gone.
	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
		"demux.audio ! queue ! aacparse name=audioenc",
		"demux.video ! queue ! h264parse name=parse",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}

	parseEle, err := pipeline.GetElementByName("parse")
	if err != nil {
		return nil, err
	}

	err = pipeline.Add(signer)
	if err != nil {
		return nil, err
	}
	err = parseEle.Link(signer)
	if err != nil {
		return nil, err
	}
	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return nil, err
	}
	err = audioenc.Link(signer)
	if err != nil {
		return nil, err
	}
	return pipeline, nil
}

// runRTMPIngestPipeline plays pipeline until it ends or ctx does.
func runRTMPIngestPipeline(ctx context.Context, pipeline *gst.Pipeline) error {
	busErr := make(chan error, 1)
	go func() {
		busErr <- HandleBusMessages(ctx, pipeline)
	}()

	err := pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "error setting pipeline to null state", "error", err)
		}
	}()

	return <-busErr
}
