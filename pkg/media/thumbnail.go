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

func Thumbnail(ctx context.Context, r io.Reader, w io.Writer, format string) error {
	ctx = log.WithLogValues(ctx, "function", "Thumbnail")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var encoder string
	switch format {
	case "jpeg":
		encoder = "jpegenc"
	case "png":
		encoder = "pngenc snapshot=true"
	default:
		log.Error(ctx, "media.Thumbnail: expected jpeg or png as format and received %s", format)
		encoder = "pngenc snapshot=true"
	}

	// Read all data from the reader to create a Seg for ConcatDemuxBin
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("error reading input data: %w", err)
	}

	seg := &bus.Seg{
		Data: data,
	}

	pipelineSlice := []string{
		"decodebin name=decode ! videoconvert ! videoscale ! videorate ! capsfilter name=capsfilter caps=video/x-raw,width=[1,1280],height=[1,720],pixel-aspect-ratio=1/1,framerate=1/999999 ! ",
		encoder,
		" ! appsink name=appsink",
		"fakesink name=audiofakesink sync=false",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating Thumbnail pipeline: %w", err)
	}

	demuxBin, err := ConcatDemuxBin(ctx, seg, false)
	if err != nil {
		return fmt.Errorf("failed to create demux bin: %w", err)
	}

	err = pipeline.Add(demuxBin.Element)
	if err != nil {
		return fmt.Errorf("failed to add demux bin to pipeline: %w", err)
	}

	demuxBinPadVideoSrc := demuxBin.GetStaticPad("video_0")
	if demuxBinPadVideoSrc == nil {
		return fmt.Errorf("failed to get demux bin video src pad")
	}

	demuxBinPadAudioSrc := demuxBin.GetStaticPad("audio_0")
	if demuxBinPadAudioSrc == nil {
		return fmt.Errorf("failed to get demux bin audio src pad")
	}

	decode, err := pipeline.GetElementByName("decode")
	if err != nil {
		return fmt.Errorf("failed to get decodebin element: %w", err)
	}

	audioFakeSink, err := pipeline.GetElementByName("audiofakesink")
	if err != nil {
		return fmt.Errorf("failed to get audio fakesink element: %w", err)
	}

	linked := demuxBinPadVideoSrc.Link(decode.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link demux bin video src to decodebin: %v", linked)
	}

	linked = demuxBinPadAudioSrc.Link(audioFakeSink.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link demux bin audio src to fakesink: %v", linked)
	}

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline state to null", "error", err)
		}
		err = pipeline.Remove(demuxBin.Element)
		if err != nil {
			log.Error(ctx, "failed to remove demux bin from pipeline", "error", err)
		}
	}()

	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		errCh <- err
		close(errCh)
	}()

	thumbCh := make(chan struct{})

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			_, err := w.Write(bs)
			log.Debug(ctx, "wrote buffer", "length", len(bs))

			if err != nil {
				log.Error(ctx, "error writing to output", "error", err)
				return gst.FlowError
			}

			close(thumbCh)

			return gst.FlowOK
		},
	})

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("error setting pipeline state: %w", err)
	}

	<-thumbCh
	// signals the pipeline to clean up cleanly
	pipeline.Error(ErrConcatDone.Error(), ErrConcatDone)

	busErr := <-errCh

	log.Debug(ctx, "thumbnail done")

	return busErr
}
