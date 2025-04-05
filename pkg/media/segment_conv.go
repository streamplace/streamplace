package media

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"golang.org/x/sync/errgroup"
)

// MP4ToMPEGTS converts an MP4 file with H264 video and Opus audio to an MPEG-TS file with H264 video and AAC audio.
// It reads from the provided reader and writes the converted MPEG-TS to the writer.
func MP4ToMPEGTS(ctx context.Context, input io.Reader, output io.Writer) (int64, error) {
	pipelineStr := strings.Join([]string{
		"appsrc name=appsrc ! qtdemux name=demux",
		"mpegtsmux name=mux ! appsink name=appsink",
		"demux.video_0 ! h264parse ! video/x-h264,stream-format=byte-stream ! queue name=videoqueue",
		"demux.audio_0 ! opusdec use-inband-fec=true ! audioresample ! fdkaacenc ! aacparse ! queue name=audioqueue",
	}, " ")

	pipeline, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		return 0, err
	}

	mux, err := pipeline.GetElementByName("mux")
	if err != nil {
		return 0, err
	}
	muxVideoSinkPad := mux.GetRequestPad("sink_%d")
	if muxVideoSinkPad == nil {
		return 0, fmt.Errorf("failed to get video sink pad")
	}
	muxAudioSinkPad := mux.GetRequestPad("sink_%d")
	if muxAudioSinkPad == nil {
		return 0, fmt.Errorf("failed to get audio sink pad")
	}
	videoQueue, err := pipeline.GetElementByName("videoqueue")
	if err != nil {
		return 0, err
	}
	audioQueue, err := pipeline.GetElementByName("audioqueue")
	if err != nil {
		return 0, err
	}
	videoQueueSrcPad := videoQueue.GetStaticPad("src")
	if videoQueueSrcPad == nil {
		return 0, fmt.Errorf("failed to get video queue source pad")
	}
	audioQueueSrcPad := audioQueue.GetStaticPad("src")
	if audioQueueSrcPad == nil {
		return 0, fmt.Errorf("failed to get audio queue source pad")
	}

	ok := videoQueueSrcPad.Link(muxVideoSinkPad)
	if ok != gst.PadLinkOK {
		return 0, fmt.Errorf("failed to link video queue source pad to mux video sink pad: %v", ok)
	}
	ok = audioQueueSrcPad.Link(muxAudioSinkPad)
	if ok != gst.PadLinkOK {
		return 0, fmt.Errorf("failed to link audio queue source pad to mux audio sink pad: %v", ok)
	}

	// Get elements
	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return 0, err
	}
	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return 0, err
	}

	source := app.SrcFromElement(appsrc)
	sink := app.SinkFromElement(appsink)

	// Set up source callbacks
	source.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, input),
		EnoughDataFunc: func(self *app.Source) {
			// Nothing to do here
		},
		SeekDataFunc: func(self *app.Source, offset uint64) bool {
			return false // We don't support seeking
		},
	})

	// Set up sink callbacks
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, output),
		NewPrerollFunc: func(self *app.Sink) gst.FlowReturn {
			return gst.FlowOK
		},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle bus messages in a separate goroutine
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		HandleBusMessages(ctx, pipeline)
		cancel()
		return nil
	})

	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return 0, fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}

	// Wait for the pipeline to finish or context to be canceled
	<-ctx.Done()

	durOk, dur := pipeline.QueryDuration(gst.FormatTime)
	if !durOk {
		return 0, fmt.Errorf("failed to query duration")
	}

	// Clean up
	err = pipeline.SetState(gst.StateNull)
	if err != nil {
		return 0, fmt.Errorf("failed to set pipeline state to null: %w", err)
	}

	return dur, nil
}
