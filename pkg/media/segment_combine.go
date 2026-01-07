package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/aqio"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

// CombineSegments combines a list of segments into a single segment that maintains all of the manifests
func CombineSegments(ctx context.Context, inputFds []io.ReadSeeker, ms MediaSigner, output io.ReadWriteSeeker) error {
	rws := aqio.NewReadWriteSeeker([]byte{})
	err := CombineSegmentsUnsigned(ctx, inputFds, rws, true)
	if err != nil {
		return err
	}
	// rewind all the inputs for the signer
	for _, fd := range inputFds {
		_, err := fd.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
	}
	bs, err := rws.Bytes()
	if err != nil {
		return err
	}
	err = ms.SignConcatMP4(context.Background(), bytes.NewReader(bs), inputFds, output)
	if err != nil {
		return err
	}
	return nil
}

func CombineSegmentsUnsigned(ctx context.Context, sources []io.ReadSeeker, w io.Writer, doH264Parse bool) error {
	ctx = log.WithLogValues(ctx, "mediafunc", "CombineSegmentsUnsigned")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		fmt.Sprintf("mp4mux name=muxer faststart=true interleave-bytes=%d interleave-time=%d movie-timescale=60000 trak-timescale=60000 ! appsink sync=false name=mp4sink", InterleaveBytes, InterleaveTime),
		"capsfilter caps=video/x-h264,parsed=true name=videoqueue ! queue ! muxer.",
		"capsfilter caps=audio/x-opus,framed=true name=audioparse ! queue ! muxer.",
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

	concatBin, err := ConcatBin(ctx, segCh, doH264Parse)
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
	videoQueue, err := pipeline.GetElementByName("videoqueue")
	if err != nil {
		return fmt.Errorf("failed to get video parse element: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element: %w", err)
	}

	// Link the concat bin pads to the parse element sink pads
	linked := videoPad.Link(videoQueue.GetStaticPad("sink"))
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

	appSink := app.SinkFromElement(mp4Sink)
	appSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		errCh <- err
	}()

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

	err = <-errCh
	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}

	return nil
}
