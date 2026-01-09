package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/muxionlabs/ai-go-sdk/pkg/client"
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
	var aiCleanup func()
	var aiResources *AISessionResources

	if mm.cli.AIGatewayBaseURL != "" {
		var err error
		aiResources, err = mm.SetupAISession(ctx, ms.Streamer(), func(ctx context.Context, event client.TranscriptEvent) {
			mm.PublishTranscriptToBus(ctx, ms.Streamer(), event)
		})
		if err != nil {
			log.Log(ctx, "continuing without AI transcription", "reason", err)
		} else {
			aiCleanup = aiResources.Cleanup
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		if aiCleanup != nil {
			aiCleanup()
		}
	}()

	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
	}

	if aiResources != nil {
		pipelineSlice = append(pipelineSlice,
			"demux.audio ! queue ! fdkaacdec ! audioresample ! opusenc ! tee name=audiotee",
			"audiotee. ! queue ! appsink name=ai_audio_sink sync=false drop=true max-buffers=1",
			"audiotee. ! queue name=audioenc",
			"demux.video ! queue ! tee name=videotee",
			"videotee. ! queue ! h264parse config-interval=-1 ! video/x-h264,stream-format=byte-stream,alignment=au ! appsink name=ai_video_sink sync=false drop=true max-buffers=10",
			"videotee. ! queue ! h264parse name=parse",
		)
	} else {
		pipelineSlice = append(pipelineSlice,
			"demux.audio ! queue ! fdkaacdec ! audioresample ! opusenc name=audioenc",
			"demux.video ! queue ! h264parse name=parse",
		)
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}

	if aiResources != nil {
		videoEle, err := pipeline.GetElementByName("ai_video_sink")
		if err != nil {
			return fmt.Errorf("failed to get AI video appsink: %w", err)
		}
		audioEle, err := pipeline.GetElementByName("ai_audio_sink")
		if err != nil {
			return fmt.Errorf("failed to get AI audio appsink: %w", err)
		}
		videoSink := app.SinkFromElement(videoEle)
		audioSink := app.SinkFromElement(audioEle)

		videoSink.SetCallbacks(mm.NewAISinkCallback(aiResources.VideoCh))
		audioSink.SetCallbacks(mm.NewAISinkCallback(aiResources.AudioCh))
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
