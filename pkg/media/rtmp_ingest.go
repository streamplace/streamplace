package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/constants"
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

const rtmpIngestAudioElementName = "audioenc"

func rtmpIngestAudioChain(audioPad string) string {
	return fmt.Sprintf("%s ! %s ! aacparse name=%s", audioPad, constants.Queue2Big, rtmpIngestAudioElementName)
}

func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = withIngestProtocol(ctx, "rtmp")
	// RTMP carries AAC already. Preserve it through ingest so the canonical path
	// does not pay an AAC→Opus→AAC round trip; validation derives Opus only when a
	// WebRTC-compatible companion is needed.
	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
		rtmpIngestAudioChain("demux.audio"),
		fmt.Sprintf("demux.video ! %s ! h264parse name=parse", constants.Queue2Big),
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
	audioEle, err := pipeline.GetElementByName(rtmpIngestAudioElementName)
	if err != nil {
		return err
	}
	err = audioEle.Link(signer)
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
