package media

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

// MP4ToMPEGTS converts an MP4 file with H264 video and Opus audio to an MPEG-TS file
// It reads from the provided reader and writes the converted MPEG-TS to the writer.
// The conversion is optimized for speed.
func MP4ToMPEGTS(ctx context.Context, input io.Reader, output io.Writer) (int64, error) {
	pipelineStr := strings.Join([]string{
		"appsrc name=appsrc ! qtdemux name=demux",
		"mpegtsmux name=mux ! appsink name=appsink sync=false",
		"demux.video_0 ! h264parse ! video/x-h264,stream-format=byte-stream ! queue name=videoqueue",
		"demux.audio_0 ! opusdec name=audioparse ! audioresample ! audiorate ! fdkaacenc name=audioenc ! queue name=audioqueue",
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
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
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

// MPEGTSToMP4 converts an MPEG-TS file with H264 video and Opus audio to an MP4 file.
// It reads from the provided reader and writes the converted MP4 to the writer.
func MPEGTSToMP4(ctx context.Context, input io.Reader, output io.Writer) error {
	pipelineStr := strings.Join([]string{
		"appsrc name=appsrc ! tsdemux name=demux",
		"mp4mux name=mux ! appsink sync=false name=appsink",
		"demux.video_0_0100 ! h264parse ! video/x-h264,stream-format=avc ! queue name=videoqueue",
		"demux.audio_0_0101 ! opusdec ! opusenc ! queue name=audioqueue",
	}, " ")

	pipeline, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		return err
	}

	mux, err := pipeline.GetElementByName("mux")
	if err != nil {
		return err
	}
	muxVideoSinkPad := mux.GetRequestPad("video_%u")
	if muxVideoSinkPad == nil {
		return fmt.Errorf("failed to get video sink pad")
	}
	muxAudioSinkPad := mux.GetRequestPad("audio_%u")
	if muxAudioSinkPad == nil {
		return fmt.Errorf("failed to get audio sink pad")
	}
	videoQueue, err := pipeline.GetElementByName("videoqueue")
	if err != nil {
		return err
	}
	audioQueue, err := pipeline.GetElementByName("audioqueue")
	if err != nil {
		return err
	}
	videoQueueSrcPad := videoQueue.GetStaticPad("src")
	if videoQueueSrcPad == nil {
		return fmt.Errorf("failed to get video queue source pad")
	}
	audioQueueSrcPad := audioQueue.GetStaticPad("src")
	if audioQueueSrcPad == nil {
		return fmt.Errorf("failed to get audio queue source pad")
	}

	ok := videoQueueSrcPad.Link(muxVideoSinkPad)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link video queue source pad to mux video sink pad: %v", ok)
	}
	ok = audioQueueSrcPad.Link(muxAudioSinkPad)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio queue source pad to mux audio sink pad: %v", ok)
	}

	// Get elements
	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}
	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	source := app.SrcFromElement(appsrc)
	sink := app.SinkFromElement(appsink)

	// Set up source callbacks
	source.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
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
		return fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}

	// Wait for the pipeline to finish or context to be canceled
	<-ctx.Done()

	// durOk, dur := pipeline.QueryDuration(gst.FormatTime)
	// if !durOk {
	// 	return fmt.Errorf("failed to query duration")
	// }

	// Clean up
	err = pipeline.SetState(gst.StateNull)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to null: %w", err)
	}

	return nil
}

// Splits out video into MPEG-TS and audio into MP4 (to be recombined after transcoding)
func MP4ToMPEGTSVideoMP4Audio(ctx context.Context, input io.Reader, videoOutput io.Writer, audioOutput io.Writer) error {
	pipelineStr := strings.Join([]string{
		"appsrc name=appsrc ! qtdemux name=demux",
		"mpegtsmux name=videomux ! appsink name=videoappsink sync=false",
		"mp4mux name=audiomux ! appsink name=audioappsink sync=false",
		"demux.video_0 ! h264parse ! video/x-h264,stream-format=byte-stream ! queue name=videoqueue",
		"demux.audio_0 ! opusparse ! queue name=audioqueue",
	}, " ")

	pipeline, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		return err
	}

	videomux, err := pipeline.GetElementByName("videomux")
	if err != nil {
		return err
	}
	muxVideoSinkPad := videomux.GetRequestPad("sink_%d")
	if muxVideoSinkPad == nil {
		return fmt.Errorf("failed to get video sink pad")
	}

	audiomux, err := pipeline.GetElementByName("audiomux")
	if err != nil {
		return err
	}
	muxAudioSinkPad := audiomux.GetRequestPad("audio_%u")
	if muxAudioSinkPad == nil {
		return fmt.Errorf("failed to get audio sink pad")
	}

	videoQueue, err := pipeline.GetElementByName("videoqueue")
	if err != nil {
		return err
	}
	audioQueue, err := pipeline.GetElementByName("audioqueue")
	if err != nil {
		return err
	}

	videoQueueSrcPad := videoQueue.GetStaticPad("src")
	if videoQueueSrcPad == nil {
		return fmt.Errorf("failed to get video queue source pad")
	}
	audioQueueSrcPad := audioQueue.GetStaticPad("src")
	if audioQueueSrcPad == nil {
		return fmt.Errorf("failed to get audio queue source pad")
	}

	ok := videoQueueSrcPad.Link(muxVideoSinkPad)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link video queue source pad to mux video sink pad: %v", ok)
	}
	ok = audioQueueSrcPad.Link(muxAudioSinkPad)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio queue source pad to mux audio sink pad: %v", ok)
	}

	// Get elements
	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}
	videoappsink, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return err
	}
	audioappsink, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return err
	}

	source := app.SrcFromElement(appsrc)
	videoSink := app.SinkFromElement(videoappsink)
	audioSink := app.SinkFromElement(audioappsink)

	// Set up source callbacks
	source.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
		EnoughDataFunc: func(self *app.Source) {
			// Nothing to do here
		},
		SeekDataFunc: func(self *app.Source, offset uint64) bool {
			return false // We don't support seeking
		},
	})

	// Set up sink callbacks
	videoSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, videoOutput),
		NewPrerollFunc: func(self *app.Sink) gst.FlowReturn {
			return gst.FlowOK
		},
	})

	// Set up sink callbacks
	audioSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, audioOutput),
		NewPrerollFunc: func(self *app.Sink) gst.FlowReturn {
			return gst.FlowOK
		},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle bus messages in a separate goroutine
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err = HandleBusMessages(ctx, pipeline)
		cancel()
		return err
	})

	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}

	// Wait for the pipeline to finish or context to be canceled
	<-ctx.Done()

	// Clean up
	err = pipeline.SetState(gst.StateNull)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to null: %w", err)
	}

	return g.Wait()
}

// Joins video and audio back together from MPEG-TS and MP4 (from transcoding)
func MPEGTSVideoMP4AudioToMP4(ctx context.Context, videoInput io.Reader, audioInput io.Reader, output io.Writer) error {
	pipelineStr := strings.Join([]string{
		"appsrc name=videoappsrc ! tsdemux name=videodemux",
		"appsrc name=audioappsrc ! qtdemux name=audiodemux",
		"mp4mux name=mux ! appsink name=appsink sync=false",
		"h264parse name=videoparse ! video/x-h264,stream-format=avc ! queue name=videoqueue",
		"audiodemux.audio_0 ! opusparse ! queue name=audioqueue",
	}, " ")

	pipeline, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		return err
	}

	mux, err := pipeline.GetElementByName("mux")
	if err != nil {
		return err
	}
	muxVideoSinkPad := mux.GetRequestPad("video_%u")
	if muxVideoSinkPad == nil {
		return fmt.Errorf("failed to get video sink pad")
	}
	muxAudioSinkPad := mux.GetRequestPad("audio_%u")
	if muxAudioSinkPad == nil {
		return fmt.Errorf("failed to get audio sink pad")
	}

	videoQueue, err := pipeline.GetElementByName("videoqueue")
	if err != nil {
		return err
	}
	audioQueue, err := pipeline.GetElementByName("audioqueue")
	if err != nil {
		return err
	}

	videoQueueSrcPad := videoQueue.GetStaticPad("src")
	if videoQueueSrcPad == nil {
		return fmt.Errorf("failed to get video queue source pad")
	}
	audioQueueSrcPad := audioQueue.GetStaticPad("src")
	if audioQueueSrcPad == nil {
		return fmt.Errorf("failed to get audio queue source pad")
	}

	ok := videoQueueSrcPad.Link(muxVideoSinkPad)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link video queue source pad to mux video sink pad: %v", ok)
	}
	ok = audioQueueSrcPad.Link(muxAudioSinkPad)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio queue source pad to mux audio sink pad: %v", ok)
	}

	videodemux, err := pipeline.GetElementByName("videodemux")
	if err != nil {
		return err
	}
	videoparse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return err
	}
	videoParseSinkPad := videoparse.GetStaticPad("sink")
	if videoParseSinkPad == nil {
		return fmt.Errorf("failed to get video parse sink pad")
	}

	// Get elements
	videoappsrc, err := pipeline.GetElementByName("videoappsrc")
	if err != nil {
		return err
	}
	audioappsrc, err := pipeline.GetElementByName("audioappsrc")
	if err != nil {
		return err
	}
	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	videoSource := app.SrcFromElement(videoappsrc)
	audioSource := app.SrcFromElement(audioappsrc)
	sink := app.SinkFromElement(appsink)

	// Set up source callbacks
	videoSource.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, videoInput),
		EnoughDataFunc: func(self *app.Source) {
			// Nothing to do here
		},
		SeekDataFunc: func(self *app.Source, offset uint64) bool {
			return false // We don't support seeking
		},
	})

	audioSource.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, audioInput),
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

	onPadAdded := func(element *gst.Element, pad *gst.Pad) {
		if pad.GetDirection() == gst.PadDirectionSource {
			ok := pad.Link(videoParseSinkPad)
			defer func() { videoParseSinkPad = nil }()
			if ok != gst.PadLinkOK {
				log.Error(ctx, "failed to link video parse sink pad to video demux pad", "error", ok)
				cancel()
			}
		}
	}
	videodemux.Connect("pad-added", onPadAdded)

	// Handle bus messages in a separate goroutine
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err = HandleBusMessages(ctx, pipeline)
		cancel()
		return err
	})

	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}

	// Wait for the pipeline to finish or context to be canceled
	<-ctx.Done()

	// Clean up
	err = pipeline.SetState(gst.StateNull)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to null: %w", err)
	}

	return g.Wait()
}
