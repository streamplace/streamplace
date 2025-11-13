package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

var registerMP4SyncOnce sync.Once

func initMP4Sync() {
	registerMP4SyncOnce.Do(func() {
		success := RegisterMP4Sync()
		if !success {
			panic("failed to register mp4sync element")
		}
	})
}

func CombineSegmentsUnsigned(ctx context.Context, sources []io.ReadSeeker, w io.Writer) error {
	initMP4Sync()
	ctx = log.WithLogValues(ctx, "mediafunc", "CombineSegmentsUnsigned")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		"capsfilter caps=video/x-h264,parsed=true name=videoqueue ! queue ! appsink name=videosink",
		"opusparse name=audioparse ! queue ! appsink name=audiosink",
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

	eos := make(chan struct{})

	var videoCaps string
	videoReady := make(chan struct{})
	var audioCaps string
	audioReady := make(chan struct{})
	videoCh := make(chan *SegmentBuffer)
	audioCh := make(chan *SegmentBuffer)

	videoSinkElem, err := pipeline.GetElementByName("videosink")
	if err != nil {
		return fmt.Errorf("failed to get mp4sink element: %w", err)
	}

	videoSink := app.SinkFromElement(videoSinkElem)
	videoSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			pads, err := sink.GetSinkPads()
			if err != nil {
				return gst.FlowError
			}
			if videoCaps == "" {
				caps := pads[0].GetCurrentCaps()
				videoCaps = caps.String()
				close(videoReady)
			}
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			videoCh <- &SegmentBuffer{
				bytes: bs,
				pts:   buffer.PresentationTimestamp().AsDuration(),
				dur:   buffer.Duration().AsDuration(),
			}
			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			go func() {
				close(videoCh)
				eos <- struct{}{}
			}()
		},
	})

	audioSinkElem, err := pipeline.GetElementByName("audiosink")
	if err != nil {
		return fmt.Errorf("failed to get mp4sink element: %w", err)
	}

	audioSink := app.SinkFromElement(audioSinkElem)
	audioSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			pads, err := sink.GetSinkPads()
			if err != nil {
				return gst.FlowError
			}
			if audioCaps == "" {
				caps := pads[0].GetCurrentCaps()
				audioCaps = caps.String()
				close(audioReady)
			}
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			audioCh <- &SegmentBuffer{
				bytes: bs,
				pts:   buffer.PresentationTimestamp().AsDuration(),
				dur:   buffer.Duration().AsDuration(),
			}
			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			go func() {
				close(audioCh)
				eos <- struct{}{}
			}()
		},
	})

	outputErrCh := make(chan error)
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-videoReady:
		}
		select {
		case <-ctx.Done():
			return
		case <-audioReady:
		}
		outputErrCh <- SegmentsToMP4(ctx, videoCaps, audioCaps, videoCh, audioCh, w)
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

	// Handle bus messages
	err = HandleBusMessages(ctx, pipeline)

	<-eos
	<-eos

	outputErr := <-outputErrCh
	if outputErr != nil {
		return fmt.Errorf("failed to output concatted mp4: %w", outputErr)
	}

	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}

	log.Warn(ctx, "pipeline ended")

	return nil
}

func SegmentsToMP4(ctx context.Context, videoCaps, audioCaps string, videoCh, audioCh chan *SegmentBuffer, w io.Writer) error {
	pipelineSlice := []string{
		"appsrc format=time name=videosrc ! queue ! mux.video_0",
		"appsrc format=time name=audiosrc ! queue ! mux.audio_0",
		"mp4mux name=mux faststart=true ! appsink sync=false name=mp4sink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	eos := make(chan struct{})

	videoSrcElem, err := pipeline.GetElementByName("videosrc")
	if err != nil {
		return fmt.Errorf("failed to get video src element: %w", err)
	}
	videoSrc := app.SrcFromElement(videoSrcElem)
	if videoSrc == nil {
		return fmt.Errorf("failed to get video src element: %w", err)
	}
	videoSrc.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, length uint) {
			select {
			case <-ctx.Done():
				return
			case seg := <-videoCh:
				if seg == nil {
					log.Warn(ctx, "video source ended")
					self.EndStream()
					return
				}
				buf := gst.NewBufferFromBytes(seg.bytes)
				if seg.pts != nil {
					buf.SetPresentationTimestamp(gst.ClockTime(uint64(seg.pts.Nanoseconds())))
				}
				if seg.dur != nil {
					buf.SetDuration(gst.ClockTime(uint64(seg.dur.Nanoseconds())))
				}
				videoSrc.PushBuffer(buf)
				return
			}
		},
	})
	videoSrc.SetCaps(gst.NewCapsFromString(videoCaps))

	audioSrcElem, err := pipeline.GetElementByName("audiosrc")
	if err != nil {
		return fmt.Errorf("failed to get audio src element: %w", err)
	}
	audioSrc := app.SrcFromElement(audioSrcElem)
	if audioSrc == nil {
		return fmt.Errorf("failed to get audio src element: %w", err)
	}
	audioSrc.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, length uint) {
			select {
			case <-ctx.Done():
				return
			case seg := <-audioCh:
				if seg == nil {
					log.Warn(ctx, "audio source ended")
					self.EndStream()
					return
				}
				buf := gst.NewBufferFromBytes(seg.bytes)
				if seg.pts != nil {
					buf.SetPresentationTimestamp(gst.ClockTime(uint64(seg.pts.Nanoseconds())))
				}
				if seg.dur != nil {
					buf.SetDuration(gst.ClockTime(uint64(seg.dur.Nanoseconds())))
				}
				audioSrc.PushBuffer(buf)
				return
			}
		},
	})
	audioSrc.SetCaps(gst.NewCapsFromString(audioCaps))

	mp4SinkElem, err := pipeline.GetElementByName("mp4sink")
	if err != nil {
		return fmt.Errorf("failed to get mp4 sink element: %w", err)
	}
	mp4Sink := app.SinkFromElement(mp4SinkElem)
	if mp4Sink == nil {
		return fmt.Errorf("failed to get mp4 sink element: %w", err)
	}
	mp4Sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
		EOSFunc: func(sink *app.Sink) {
			go func() {
				eos <- struct{}{}
			}()
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

	err = HandleBusMessages(ctx, pipeline)

	<-eos

	if err != nil {
		return fmt.Errorf("pipeline error: %w", err)
	}
	// Handle bus messages
	return nil
}
