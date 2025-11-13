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

type VideoBuffer struct {
	Buffer *gst.Buffer
}

type AudioBuffer struct {
	Buffer *gst.Buffer
}

type VideoEvent struct {
	Event *gst.Event
}

type AudioEvent struct {
	Event *gst.Event
}

func CombineSegmentsUnsigned(ctx context.Context, sources []io.ReadSeeker, w io.Writer) error {
	initMP4Sync()
	ctx = log.WithLogValues(ctx, "mediafunc", "CombineSegmentsUnsigned")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		"capsfilter caps=video/x-h264,parsed=true name=videoqueue ! appsink sync=false name=videosink",
		"opusparse name=audioparse ! appsink sync=false name=audiosink",
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

	// var videoCaps string
	// videoReady := make(chan struct{})
	// var audioCaps string
	// audioReady := make(chan struct{})
	outputCh := make(chan any)

	videoSinkElem, err := pipeline.GetElementByName("videosink")
	if err != nil {
		return fmt.Errorf("failed to get mp4sink element: %w", err)
	}

	videoSink := app.SinkFromElement(videoSinkElem)
	videoSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			// pads, err := sink.GetSinkPads()
			// if err != nil {
			// 	return gst.FlowError
			// }
			// if videoCaps == "" {
			// 	caps := pads[0].GetCurrentCaps()
			// 	videoCaps = caps.String()
			// 	close(videoReady)
			// }
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}
			// bs := buffer.Map(gst.MapRead).Bytes()
			// defer buffer.Unmap()
			log.Debug(ctx, "video buffer input", "presentation_timestamp", buffer.PresentationTimestamp(), "duration", buffer.Duration())
			outputCh <- &VideoBuffer{
				Buffer: buffer,
			}
			// videoCh <- &SegmentBuffer{
			// 	bytes: bs,
			// 	pts:   buffer.PresentationTimestamp().AsDuration(),
			// 	dur:   buffer.Duration().AsDuration(),
			// }
			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			go func() {
				outputCh <- &VideoBuffer{
					Buffer: nil,
				}
				eos <- struct{}{}
			}()
		},
	})

	videoSinkSinkPad := videoSink.GetStaticPad("sink")
	videoSinkSinkPad.AddProbe(gst.PadProbeTypeEventDownstream, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		// log.Warn(ctx, "video probe event", "event", fmt.Sprintf("%+v", info.GetEvent().Type()), "direction", pad.GetDirection())
		if info.GetEvent().Type() == gst.EventTypeSegment {
			segment := info.GetEvent().ParseSegment()
			log.Debug(ctx, "video segment", "segment", fmt.Sprintf("%+v", segment.GetFormat().String()))
		}
		if info.GetEvent().Type() != gst.EventTypeSegment && info.GetEvent().Type() != gst.EventTypeCaps && info.GetEvent().Type() != gst.EventTypeStreamStart {
			return gst.PadProbeOK
		}
		log.Debug(ctx, "video event input", "event", fmt.Sprintf("%+v", info.GetEvent().Type()), "pointer", info.GetEvent())
		outputCh <- &VideoEvent{
			Event: info.GetEvent().Copy(),
		}
		return gst.PadProbeOK
	})

	audioSinkElem, err := pipeline.GetElementByName("audiosink")
	if err != nil {
		return fmt.Errorf("failed to get mp4sink element: %w", err)
	}

	audioSink := app.SinkFromElement(audioSinkElem)
	audioSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			// pads, err := sink.GetSinkPads()
			// if err != nil {
			// 	return gst.FlowError
			// }
			// if audioCaps == "" {
			// 	caps := pads[0].GetCurrentCaps()
			// 	audioCaps = caps.String()
			// 	close(audioReady)
			// }
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}
			log.Debug(ctx, "audio buffer input", "presentation_timestamp", buffer.PresentationTimestamp(), "duration", buffer.Duration())
			outputCh <- &AudioBuffer{
				Buffer: buffer,
			}

			// bs := buffer.Map(gst.MapRead).Bytes()
			// defer buffer.Unmap()
			// audioCh <- &SegmentBuffer{
			// 	bytes: bs,
			// 	pts:   buffer.PresentationTimestamp().AsDuration(),
			// 	dur:   buffer.Duration().AsDuration(),
			// }
			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			go func() {
				outputCh <- &AudioBuffer{
					Buffer: nil,
				}
				eos <- struct{}{}
			}()
		},
	})

	audioSinkSinkPad := audioSink.GetStaticPad("sink")
	audioSinkSinkPad.AddProbe(gst.PadProbeTypeEventDownstream, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		// log.Warn(ctx, "audio probe event", "event", fmt.Sprintf("%+v", info.GetEvent().Type()))
		if info.GetEvent().Type() == gst.EventTypeSegment {
			info.GetEvent().GetStructure()
			segment := info.GetEvent().ParseSegment()
			log.Debug(ctx, "audio segment", "segment", fmt.Sprintf("%+v", segment.GetFormat().String()))
		}
		if info.GetEvent().Type() == gst.EventTypeStreamStart {
			info.GetEvent().ParseStreamStart()
		}
		if info.GetEvent().Type() != gst.EventTypeSegment && info.GetEvent().Type() != gst.EventTypeCaps {
			return gst.PadProbeOK
		}
		log.Debug(ctx, "audio event input", "event", fmt.Sprintf("%+v", info.GetEvent().Type()), "pointer", info.GetEvent())
		outputCh <- &AudioEvent{
			Event: info.GetEvent().Copy(),
		}
		return gst.PadProbeOK
	})

	outputErrCh := make(chan error)
	go func() {
		// select {
		// case <-ctx.Done():
		// 	return
		// case <-videoReady:
		// }
		// select {
		// case <-ctx.Done():
		// 	return
		// case <-audioReady:
		// }
		outputErrCh <- SegmentsToMP4(ctx, outputCh, w)
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

func SegmentsToMP4(ctx context.Context, ch chan any, w io.Writer) error {
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

	videoCh := make(chan *VideoBuffer)
	audioCh := make(chan *AudioBuffer)

	videoSrcElem, err := pipeline.GetElementByName("videosrc")
	if err != nil {
		return fmt.Errorf("failed to get video src element: %w", err)
	}
	videoSrc := app.SrcFromElement(videoSrcElem)
	if videoSrc == nil {
		return fmt.Errorf("failed to get video src element: %w", err)
	}
	// videoSrc.SetCallbacks(&app.SourceCallbacks{
	// 	NeedDataFunc: func(self *app.Source, length uint) {
	// 		select {
	// 		case <-ctx.Done():
	// 			log.Warn(ctx, "video source context done (cancellation implied?)")
	// 			return
	// 		case buf := <-videoCh:
	// 			if buf == nil {
	// 				log.Warn(ctx, "video source ended")
	// 				self.EndStream()
	// 				return
	// 			}
	// 			// buf := gst.NewBufferFromBytes(seg.bytes)
	// 			// if seg.pts != nil {
	// 			// 	buf.SetPresentationTimestamp(gst.ClockTime(uint64(seg.pts.Nanoseconds())))
	// 			// }
	// 			// if seg.dur != nil {
	// 			// 	buf.SetDuration(gst.ClockTime(uint64(seg.dur.Nanoseconds())))
	// 			// }
	// 			result := videoSrc.PushBuffer(buf.Buffer)
	// 			if result != gst.FlowOK {
	// 				log.Error(ctx, "failed to push video buffer", "error", "push buffer returned false")
	// 				self.EndStream()
	// 				return
	// 			}
	// 			return
	// 		}
	// 	},
	// })
	// videoSrc.SetCaps(gst.NewCapsFromString(videoCaps))
	videoSrcPad := videoSrc.GetStaticPad("src")

	audioSrcElem, err := pipeline.GetElementByName("audiosrc")
	if err != nil {
		return fmt.Errorf("failed to get audio src element: %w", err)
	}
	audioSrc := app.SrcFromElement(audioSrcElem)
	if audioSrc == nil {
		return fmt.Errorf("failed to get audio src element: %w", err)
	}
	// audioSrc.SetCallbacks(&app.SourceCallbacks{
	// 	NeedDataFunc: func(self *app.Source, length uint) {
	// 		select {
	// 		case <-ctx.Done():
	// 			return
	// 		case buf := <-audioCh:
	// 			if buf == nil {
	// 				log.Warn(ctx, "audio source ended")
	// 				self.EndStream()
	// 				return
	// 			}
	// 			// buf := gst.NewBufferFromBytes(seg.bytes)
	// 			// if seg.pts != nil {
	// 			// 	buf.SetPresentationTimestamp(gst.ClockTime(uint64(seg.pts.Nanoseconds())))
	// 			// }
	// 			// if seg.dur != nil {
	// 			// 	buf.SetDuration(gst.ClockTime(uint64(seg.dur.Nanoseconds())))
	// 			// }
	// 			result := audioSrc.PushBuffer(buf.Buffer)
	// 			if result != gst.FlowOK {
	// 				log.Error(ctx, "failed to push audio buffer", "error", "push buffer returned false")
	// 				self.EndStream()
	// 				return
	// 			}
	// 			return
	// 		}
	// 	},
	// })
	// audioSrc.SetCaps(gst.NewCapsFromString(audioCaps))
	audioSrcPad := audioSrc.GetStaticPad("src")

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

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case buf := <-ch:
				if buf == nil {
					log.Warn(ctx, "source ended")
					close(videoCh)
					close(audioCh)
					return
				}
				switch buf := buf.(type) {
				case *VideoBuffer:
					if buf.Buffer == nil {
						log.Warn(ctx, "video buffer is nil")
						videoSrc.EndStream()
						continue
					}
					log.Debug(ctx, "video buffer output", "presentation_timestamp", buf.Buffer.PresentationTimestamp(), "duration", buf.Buffer.Duration())
					ret := videoSrc.PushBuffer(buf.Buffer)
					if ret != gst.FlowOK {
						log.Error(ctx, "failed to push video buffer", "error", ret.String())
					}
					// videoCh <- buf
				case *AudioBuffer:
					// audioCh <- buf
					if buf.Buffer == nil {
						log.Warn(ctx, "audio buffer is nil")
						audioSrc.EndStream()
						continue
					}
					log.Debug(ctx, "audio buffer output", "presentation_timestamp", buf.Buffer.PresentationTimestamp(), "duration", buf.Buffer.Duration())
					ret := audioSrc.PushBuffer(buf.Buffer)
					if ret != gst.FlowOK {
						log.Error(ctx, "failed to push audio buffer", "error", ret.String())
					}
				case *VideoEvent:
					log.Warn(ctx, "video event output", "event", fmt.Sprintf("%+v", buf.Event.Type()))
					videoSrcPad.PushEvent(buf.Event)
				case *AudioEvent:
					log.Warn(ctx, "audio event output", "event", fmt.Sprintf("%+v", buf.Event.Type()))
					audioSrcPad.PushEvent(buf.Event)
				}
			}
		}
	}()

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
