package media

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

func CombineSegmentsUnsigned(ctx context.Context, sources []io.ReadSeeker, w io.Writer) error {
	ctx = log.WithLogValues(ctx, "mediafunc", "CombineSegmentsUnsigned")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		"mp4mux name=muxer faststart=true ! appsink sync=false name=mp4sink",
		"h264parse name=videoparse ! h264timestamper ! muxer.video_0",
		"opusparse name=audioparse ! muxer.audio_0",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	segCh := make(chan *bus.Seg)
	go func() {
		for _, source := range sources {
			bs, err := io.ReadAll(source)
			if err != nil {
				err = fmt.Errorf("failed to read file: %w", err)
				pipeline.Error(err.Error(), err)
				return
			}
			segCh <- &bus.Seg{
				Filepath: "ignored",
				Data:     bs,
			}
		}
		close(segCh)
	}()

	concatBin, err := ConcatBin(ctx, segCh)
	if err != nil {
		return fmt.Errorf("failed to create concat bin: %w", err)
	}

	err = pipeline.Add(concatBin.Element)
	if err != nil {
		return fmt.Errorf("failed to add concat bin to pipeline: %w", err)
	}

	videoPad := concatBin.GetStaticPad("video_0")
	if videoPad == nil {
		return fmt.Errorf("video pad not found")
	}

	audioPad := concatBin.GetStaticPad("audio_0")
	if audioPad == nil {
		return fmt.Errorf("audio pad not found")
	}

	// Get the videoparse and audioparse elements from the pipeline
	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("failed to get video parse element: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element: %w", err)
	}

	// Link the concat bin pads to the parse element sink pads
	linked := videoPad.Link(videoParse.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link video pad to video parse element: %v", linked)
	}

	linked = audioPad.Link(audioParse.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio pad to audio parse element: %v", linked)
	}

	// Get the mp4sink element and set up its callback
	mp4Sink, err := pipeline.GetElementByName("mp4sink")
	if err != nil {
		return fmt.Errorf("failed to get mp4sink element: %w", err)
	}

	eos := make(chan struct{})

	appSink := app.SinkFromElement(mp4Sink)
	appSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
		EOSFunc: func(sink *app.Sink) {
			close(eos)
		},
	})

	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}
	defer func() {
		err := pipeline.BlockSetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline state to null", "error", err)
		}
	}()

	// Handle bus messages
	err = HandleBusMessages(ctx, pipeline)

	<-eos

	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}

	return nil
}
