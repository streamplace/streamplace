package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
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
	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
		"demux.audio ! queue ! fdkaacdec ! audioresample ! opusenc name=audioenc",
		"demux.video ! queue ! h264parse name=parse",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}

	parseEle, err := pipeline.GetElementByName("parse")
	if err != nil {
		return err
	}

	err = pipeline.Add(signer)
	if err != nil {
		return err
	}
	err = parseEle.Link(signer)
	if err != nil {
		return err
	}
	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return err
	}
	err = audioenc.Link(signer)
	if err != nil {
		return err
	}

	busErr := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		busErr <- err
	}()

	go mm.HandleKeyRevocation(ctx, ms, pipeline)

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "error setting pipeline to null state", "error", err)
		}
	}()

	err = <-busErr

	return err
}
