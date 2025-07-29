package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

func readFile(ctx context.Context, source string) (*bus.Seg, error) {
	fd, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file: %w", err)
	}
	defer fd.Close()
	bs, err := io.ReadAll(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}
	seg := &bus.Seg{
		Filepath: source,
		Data:     bs,
	}
	return seg, nil
}

// This function remains in scope for the duration of a single users' playback
func Clip(ctx context.Context, sources []string, w io.Writer) error {
	uu, err := uuid.NewV7()
	if err != nil {
		return err
	}
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String())
	ctx = log.WithLogValues(ctx, "mediafunc", "Clip")
	ctx, cancel := context.WithCancel(ctx) //nolint:all
	defer cancel()

	pipelineSlice := []string{
		"matroskamux name=muxer ! appsink sync=false name=mp4sink",
		"h264parse name=videoparse ! muxer.video_0",
		"opusparse name=audioparse ! muxer.audio_0",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err) //nolint:all
	}

	segCh := make(chan *bus.Seg)
	go func() {
		for _, source := range sources {
			log.Log(ctx, "reading file", "source", source)
			seg, err := readFile(ctx, source)
			if err != nil {
				err = fmt.Errorf("failed to read file: %w", err)
				pipeline.Error(err.Error(), err)
				return
			}

			segCh <- seg
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
			log.Log(ctx, "mp4sink EOSFunc")
			close(eos)
		},
	})

	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}

	// Handle bus messages
	err = HandleBusMessages(ctx, pipeline)

	<-eos

	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}

	return nil
}
