package media

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

// Splits out video into MPEG-TS and audio into MP4 (to be recombined after transcoding)
func MP4ToMPEGTSVideoMP4Audio(ctx context.Context, input io.Reader, videoOutput io.Writer, audioOutput io.Writer) error {
	ctx = log.WithLogValues(ctx, "func", "MP4ToMPEGTSVideoMP4Audio")
	pipelineStr := strings.Join([]string{
		"appsrc name=appsrc ! qtdemux name=demux",
		"mpegtsmux name=videomux ! appsink name=videoappsink sync=false",
		"mp4mux name=audiomux ! appsink name=audioappsink sync=false",
		// config-interval=-1: SPS/PPS before every IDR, taken from the MP4's
		// avcC. A transcoder decodes each pushed segment with a fresh
		// decoder, so a segment without them ("Could not find codec
		// parameters") kills its session — encoders that don't repeat the
		// headers in-band (most) produced exactly that.
		// Unlimited queues: a muxer holds its first video buffers until the
		// audio pad has data too, and a 1s 1080p60 segment with B-frame
		// reordering fills the default 1s queue before the demuxer gets to
		// the audio — the pipeline deadlocked and produced nothing.
		"demux.video_0 ! h264parse config-interval=-1 ! video/x-h264,stream-format=byte-stream ! queue name=videoqueue max-size-time=0 max-size-buffers=0 max-size-bytes=0",
		// parsebin plugs the parser the audio actually needs (opusparse for a
		// WHIP/Opus ingest, aacparse for RTMP/AAC); a hard-wired opusparse
		// never linked for AAC and the pipeline hung on it, wedging every
		// transcode behind the in-flight guard.
		"demux.audio_0 ! parsebin ! queue name=audioqueue max-size-time=0 max-size-buffers=0 max-size-bytes=0",
	}, " ")

	pipelineElem, err := gst.ParseLaunch(pipelineStr)
	if err != nil {
		return err
	}
	pipeline, ok := pipelineElem.(gst.Pipeline)
	if !ok {
		return fmt.Errorf("failed to create pipeline: not a pipeline")
	}

	videomux := pipeline.GetByName("videomux")
	if videomux == nil {
		return fmt.Errorf("failed to get video mux element")
	}
	muxVideoSinkPad := videomux.RequestPadSimple("sink_%d")
	if muxVideoSinkPad == nil {
		return fmt.Errorf("failed to get video sink pad")
	}

	audiomux := pipeline.GetByName("audiomux")
	if audiomux == nil {
		return fmt.Errorf("failed to get audio mux element")
	}
	muxAudioSinkPad := audiomux.RequestPadSimple("audio_%u")
	if muxAudioSinkPad == nil {
		return fmt.Errorf("failed to get audio sink pad")
	}

	videoQueue := pipeline.GetByName("videoqueue")
	if videoQueue == nil {
		return fmt.Errorf("failed to get video queue element")
	}
	audioQueue := pipeline.GetByName("audioqueue")
	if audioQueue == nil {
		return fmt.Errorf("failed to get audio queue element")
	}

	videoQueueSrcPad := videoQueue.GetStaticPad("src")
	if videoQueueSrcPad == nil {
		return fmt.Errorf("failed to get video queue source pad")
	}
	audioQueueSrcPad := audioQueue.GetStaticPad("src")
	if audioQueueSrcPad == nil {
		return fmt.Errorf("failed to get audio queue source pad")
	}

	linked := videoQueueSrcPad.Link(muxVideoSinkPad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link video queue source pad to mux video sink pad: %v", linked)
	}
	linked = audioQueueSrcPad.Link(muxAudioSinkPad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio queue source pad to mux audio sink pad: %v", linked)
	}

	// Get elements
	appsrc := pipeline.GetByName("appsrc")
	if appsrc == nil {
		return fmt.Errorf("failed to get appsrc element")
	}
	videoappsink := pipeline.GetByName("videoappsink")
	if videoappsink == nil {
		return fmt.Errorf("failed to get video appsink element")
	}
	audioappsink := pipeline.GetByName("audioappsink")
	if audioappsink == nil {
		return fmt.Errorf("failed to get audio appsink element")
	}

	// Set up source callbacks
	appsrc.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, input),
		EnoughData: func(self app.AppSrc) {
			// Nothing to do here
		},
		SeekData: func(self app.AppSrc, offset uint64) bool {
			return false // We don't support seeking
		},
	})

	// Set up sink callbacks
	videoappsink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: WriterNewSample(ctx, videoOutput),
		NewPreroll: func(self app.AppSink) gst.FlowReturn {
			return gst.FlowOK
		},
	})

	// Set up sink callbacks
	audioappsink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: WriterNewSample(ctx, audioOutput),
		NewPreroll: func(self app.AppSink) gst.FlowReturn {
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
	if ret := pipeline.SetState(gst.StatePlaying); ret == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline state to playing")
	}

	// Wait for the pipeline to finish or context to be canceled
	<-ctx.Done()

	// Clean up
	if ret := pipeline.SetState(gst.StateNull); ret == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline state to null")
	}

	return g.Wait()
}

// Joins video and audio back together from MPEG-TS and MP4 (from transcoding)
func MPEGTSVideoMP4AudioToMP4(ctx context.Context, videoInput io.Reader, audioInput io.Reader, output io.Writer) error {
	pipelineStr := strings.Join([]string{
		"appsrc name=videoappsrc ! tsdemux name=videodemux",
		"appsrc name=audioappsrc ! qtdemux name=audiodemux",
		"mp4mux name=mux ! appsink name=appsink sync=false",
		"h264parse name=videoparse ! video/x-h264,stream-format=avc ! queue name=videoqueue max-size-time=0 max-size-buffers=0 max-size-bytes=0",
		"audiodemux.audio_0 ! parsebin ! queue name=audioqueue max-size-time=0 max-size-buffers=0 max-size-bytes=0",
	}, " ")

	pipelineElem, err := gst.ParseLaunch(pipelineStr)
	if err != nil {
		return err
	}
	pipeline, ok := pipelineElem.(gst.Pipeline)
	if !ok {
		return fmt.Errorf("failed to create pipeline: not a pipeline")
	}

	mux := pipeline.GetByName("mux")
	if mux == nil {
		return fmt.Errorf("failed to get mux element")
	}
	muxVideoSinkPad := mux.RequestPadSimple("video_%u")
	if muxVideoSinkPad == nil {
		return fmt.Errorf("failed to get video sink pad")
	}
	muxAudioSinkPad := mux.RequestPadSimple("audio_%u")
	if muxAudioSinkPad == nil {
		return fmt.Errorf("failed to get audio sink pad")
	}

	videoQueue := pipeline.GetByName("videoqueue")
	if videoQueue == nil {
		return fmt.Errorf("failed to get video queue element")
	}
	audioQueue := pipeline.GetByName("audioqueue")
	if audioQueue == nil {
		return fmt.Errorf("failed to get audio queue element")
	}

	videoQueueSrcPad := videoQueue.GetStaticPad("src")
	if videoQueueSrcPad == nil {
		return fmt.Errorf("failed to get video queue source pad")
	}
	audioQueueSrcPad := audioQueue.GetStaticPad("src")
	if audioQueueSrcPad == nil {
		return fmt.Errorf("failed to get audio queue source pad")
	}

	linked := videoQueueSrcPad.Link(muxVideoSinkPad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link video queue source pad to mux video sink pad: %v", linked)
	}
	linked = audioQueueSrcPad.Link(muxAudioSinkPad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio queue source pad to mux audio sink pad: %v", linked)
	}

	videodemux := pipeline.GetByName("videodemux")
	if videodemux == nil {
		return fmt.Errorf("failed to get video demux element")
	}
	videoparse := pipeline.GetByName("videoparse")
	if videoparse == nil {
		return fmt.Errorf("failed to get video parse element")
	}
	videoParseSinkPad := videoparse.GetStaticPad("sink")
	if videoParseSinkPad == nil {
		return fmt.Errorf("failed to get video parse sink pad")
	}

	// Get elements
	videoappsrc := pipeline.GetByName("videoappsrc")
	if videoappsrc == nil {
		return fmt.Errorf("failed to get video appsrc element")
	}
	audioappsrc := pipeline.GetByName("audioappsrc")
	if audioappsrc == nil {
		return fmt.Errorf("failed to get audio appsrc element")
	}
	appsink := pipeline.GetByName("appsink")
	if appsink == nil {
		return fmt.Errorf("failed to get appsink element")
	}

	// Set up source callbacks
	videoappsrc.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, videoInput),
		EnoughData: func(self app.AppSrc) {
			// Nothing to do here
		},
		SeekData: func(self app.AppSrc, offset uint64) bool {
			return false // We don't support seeking
		},
	})

	audioappsrc.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, audioInput),
		EnoughData: func(self app.AppSrc) {
			// Nothing to do here
		},
		SeekData: func(self app.AppSrc, offset uint64) bool {
			return false // We don't support seeking
		},
	})

	wroteAnything := false

	// Set up sink callbacks
	appsink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: func(sink app.AppSink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()
			info, ok := buffer.Map(gst.MapRead)
			if !ok {
				log.Error(ctx, "failed to map buffer")
				return gst.FlowError
			}
			defer info.Unmap()

			_, err := output.Write(info.Data())

			if err != nil {
				log.Error(ctx, "error writing to output", "error", err)
				return gst.FlowError
			}

			wroteAnything = true

			return gst.FlowOK
		},
		NewPreroll: func(self app.AppSink) gst.FlowReturn {
			return gst.FlowOK
		},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	onPadAdded := func(element gst.Element, pad gst.Pad) {
		if pad.GetDirection() == gst.PadSrc {
			ok := pad.Link(videoParseSinkPad)
			if ok != gst.PadLinkOK {
				log.Error(ctx, "failed to link video parse sink pad to video demux pad", "error", ok)
				cancel()
			}
		}
	}
	videodemux.ConnectPadAdded(onPadAdded)

	errCh := make(chan error)
	go func() {
		err = HandleBusMessages(ctx, pipeline)
		cancel()
		errCh <- err
	}()

	// Start the pipeline
	if ret := pipeline.SetState(gst.StatePlaying); ret == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline state to playing")
	}

	defer func() {
		if ret := pipeline.SetState(gst.StateNull); ret == gst.StateChangeFailure {
			log.Error(ctx, "failed to set pipeline state to null")
		}
		videoParseSinkPad = nil
	}()

	err = <-errCh
	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}

	if !wroteAnything {
		return fmt.Errorf("no data written to output")
	}

	return nil
}
