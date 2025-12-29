package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/esimov/stackblur-go"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

func Thumbnail(ctx context.Context, r io.Reader, w io.Writer, format string, blur bool) error {
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

	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux name=demux ! decodebin ! videoconvert ! videoscale ! videorate ! capsfilter name=capsfilter caps=video/x-raw,width=[1,1280],height=[1,720],pixel-aspect-ratio=1/1,framerate=1/999999 ! ",
		encoder,
		" ! appsink name=appsink",
	}

	// If blur is needed, we'll apply it in go after gstreamer generates the thumbnail
	var thumbnailBuf *bytes.Buffer
	originalWriter := w
	if blur {
		thumbnailBuf = &bytes.Buffer{}
		w = thumbnailBuf
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating Thumbnail pipeline: %w", err)
	}

	defer func() {
		cancel()
		err = pipeline.BlockSetState(gst.StateNull)
	}()
	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, r),
	})

	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		cancel()
		errCh <- err
		close(errCh)
	}()

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	if err := pipeline.BlockSetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("error setting pipeline state: %w", err)
	}

	<-ctx.Done()

	if err := pipeline.BlockSetState(gst.StateNull); err != nil {
		return fmt.Errorf("error setting pipeline state: %w", err)
	}

	err = <-errCh
	if err != nil {
		return err
	}

	// Apply blur if needed
	if blur && thumbnailBuf != nil {
		var img image.Image
		switch format {
		case "jpeg":
			img, err = jpeg.Decode(thumbnailBuf)
		case "png":
			img, err = png.Decode(thumbnailBuf)
		default:
			img, err = png.Decode(thumbnailBuf)
		}
		if err != nil {
			return fmt.Errorf("error decoding thumbnail for blur: %w", err)
		}

		// Apply stackblur - needs NRGBA output
		blurred := image.NewNRGBA(img.Bounds())
		err = stackblur.Process(blurred, img, 60)
		if err != nil {
			return fmt.Errorf("error applying blur: %w", err)
		}

		// Encode back to the original writer
		switch format {
		case "jpeg":
			err = jpeg.Encode(originalWriter, blurred, &jpeg.Options{Quality: 85})
		case "png":
			err = png.Encode(originalWriter, blurred)
		default:
			err = png.Encode(originalWriter, blurred)
		}
		if err != nil {
			return fmt.Errorf("error encoding blurred thumbnail: %w", err)
		}
	}

	return nil
}
