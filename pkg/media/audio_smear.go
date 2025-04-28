package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

type SegmentBuffer struct {
	bytes []byte
	pts   *time.Duration
	dur   *time.Duration
}

type SegmentData struct {
	Audio     []SegmentBuffer
	AudioCaps string
	Video     []SegmentBuffer
	VideoCaps string
}

func SmearAudioTimestamps(ctx context.Context, input io.Reader, output io.Writer) error {
	seg, err := ToBuffers(ctx, input)
	if err != nil {
		return err
	}

	seg.Normalize()

	return JoinAudioVideo(ctx, seg, output)
}

func (s *SegmentData) Normalize(ctx context.Context) error {
	lastVideo := s.Video[len(s.Video)-1]
	lastAudio := s.Audio[len(s.Audio)-1]

	videoEnd := lastVideo.pts.Nanoseconds() + lastVideo.dur.Nanoseconds()
	audioEnd := lastAudio.pts.Nanoseconds() + lastAudio.dur.Nanoseconds()
	log.Log(ctx, "video end", "video end", videoEnd, "audio end", audioEnd)

	diff := videoEnd - audioEnd
	diffPerAudio := diff / int64(len(s.Audio)-1)
	log.Log(ctx, "diff per audio", "diff per audio", diffPerAudio)
	for i, audio := range s.Audio {
		newPts := time.Duration(audio.pts.Nanoseconds() + (diffPerAudio * int64(i)))
		log.Log(ctx, "new pts", "new pts", newPts, "old pts", audio.pts)
		audio.pts = &newPts
		s.Audio[i] = audio
	}

	lastVideo = s.Video[len(s.Video)-1]
	lastAudio = s.Audio[len(s.Audio)-1]
	videoEnd = lastVideo.pts.Nanoseconds() + lastVideo.dur.Nanoseconds()
	audioEnd = lastAudio.pts.Nanoseconds() + lastAudio.dur.Nanoseconds()
	log.Log(ctx, "video end", "video end", videoEnd, "audio end", audioEnd)
}

func ToBuffers(ctx context.Context, input io.Reader) (*SegmentData, error) {
	ctx = log.WithLogValues(ctx, "func", "SplitAudioVideo")

	pipelineSlice := []string{
		"appsrc name=mp4src ! qtdemux name=demux",
		"demux.video_0 ! queue ! h264parse name=videoparse ! appsink sync=false name=videoappsink",
		"demux.audio_0 ! queue ! opusparse name=audioparse ! appsink sync=false name=audioappsink",
	}

	ctx, cancel := context.WithCancel(ctx)

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		cancel()
		errCh <- err
		close(errCh)
	}()

	defer func() {
		cancel()
		err := <-errCh
		if err != nil {
			log.Error(ctx, "bus handler error", "error", err)
		}
		err = pipeline.BlockSetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline to null state", "error", err)
		}
	}()

	mp4src, err := pipeline.GetElementByName("mp4src")
	if err != nil {
		return nil, fmt.Errorf("failed to get mp4src element: %w", err)
	}
	src := app.SrcFromElement(mp4src)
	if src == nil {
		return nil, fmt.Errorf("failed to get mp4src element: %w", err)
	}
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, input),
	})

	audioSinkElem, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get audioappsink element: %w", err)
	}
	audioSink := app.SinkFromElement(audioSinkElem)
	if audioSink == nil {
		return nil, fmt.Errorf("failed to get audioappsink element: %w", err)
	}

	seg := SegmentData{
		Audio: []SegmentBuffer{},
		Video: []SegmentBuffer{},
	}

	audioSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()
			log.Log(ctx, "audio buffer", "presentation_timestamp", buffer.PresentationTimestamp(), "duration", buffer.Duration(), "dts", buffer.DecodingTimestamp())
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			sinkPads, err := sink.GetSinkPads()
			if err != nil {
				panic(err)
			}
			caps := sinkPads[0].GetCurrentCaps()
			if caps != nil {
				seg.AudioCaps = caps.String()
			}

			seg.Audio = append(seg.Audio, SegmentBuffer{
				bytes: bs,
				pts:   buffer.PresentationTimestamp().AsDuration(),
				dur:   buffer.Duration().AsDuration(),
			})

			if err != nil {
				panic(err)
			}

			return gst.FlowOK
		},
	})

	videoSinkElem, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get videoappsink element: %w", err)
	}
	videoSink := app.SinkFromElement(videoSinkElem)
	if videoSink == nil {
		return nil, fmt.Errorf("failed to get videoappsink element: %w", err)
	}
	videoSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()
			// log.Log(ctx, "video buffer", "presentation_timestamp", buffer.PresentationTimestamp(), "duration", buffer.Duration())
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			sinkPads, err := sink.GetSinkPads()
			if err != nil {
				panic(err)
			}
			caps := sinkPads[0].GetCurrentCaps()
			if caps != nil {
				seg.VideoCaps = caps.String()
			}

			seg.Video = append(seg.Video, SegmentBuffer{
				bytes: bs,
				pts:   buffer.PresentationTimestamp().AsDuration(),
				dur:   buffer.Duration().AsDuration(),
			})

			if err != nil {
				panic(err)
			}

			return gst.FlowOK
		},
	})

	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	return &seg, <-errCh
}

func JoinAudioVideo(ctx context.Context, seg *SegmentData, output io.Writer) error {
	ctx = log.WithLogValues(ctx, "func", "SplitAudioVideo")

	pipelineSlice := []string{
		"mp4mux name=mux ! appsink sync=false name=mp4sink",
		"appsrc name=videosrc format=time ! queue ! mux.video_0",
		"appsrc name=audiosrc format=time ! queue ! mux.audio_0",
	}

	ctx, cancel := context.WithCancel(ctx)

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		cancel()
		errCh <- err
		close(errCh)
	}()

	defer func() {
		cancel()
		err := <-errCh
		if err != nil {
			log.Error(ctx, "bus handler error", "error", err)
		}
		err = pipeline.BlockSetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline to null state", "error", err)
		}
	}()

	videoSrcElem, err := pipeline.GetElementByName("videosrc")
	if err != nil {
		return fmt.Errorf("failed to get videosrc element: %w", err)
	}
	videoSrc := app.SrcFromElement(videoSrcElem)
	if videoSrc == nil {
		return fmt.Errorf("failed to get videosrc element: %w", err)
	}
	videoSrc.SetCaps(gst.NewCapsFromString(seg.VideoCaps))
	for _, seg := range seg.Video {
		buf := gst.NewBufferFromBytes(seg.bytes)
		if seg.pts != nil {
			buf.SetPresentationTimestamp(gst.ClockTime(uint64(seg.pts.Nanoseconds())))
		}
		if seg.dur != nil {
			buf.SetDuration(gst.ClockTime(uint64(seg.dur.Nanoseconds())))
		}
		ret := videoSrc.PushBuffer(buf)
		if ret != gst.FlowOK {
			return fmt.Errorf("failed to push video buffer: %s", ret)
		} else {
			// log.Log(ctx, "pushed video buffer", "presentation_timestamp", seg.pts, "duration", seg.dur)
		}
	}

	audioSrcElem, err := pipeline.GetElementByName("audiosrc")
	if err != nil {
		return fmt.Errorf("failed to get audiosrc element: %w", err)
	}
	audioSrc := app.SrcFromElement(audioSrcElem)
	if audioSrc == nil {
		return fmt.Errorf("failed to get audiosrc element: %w", err)
	}
	audioSrc.SetCaps(gst.NewCapsFromString(seg.AudioCaps))
	for _, seg := range seg.Audio {
		buf := gst.NewBufferFromBytes(seg.bytes)
		if seg.pts != nil {
			buf.SetPresentationTimestamp(gst.ClockTime(uint64(seg.pts.Nanoseconds())))
		}
		if seg.dur != nil {
			buf.SetDuration(gst.ClockTime(uint64(seg.dur.Nanoseconds())))
		}
		ret := audioSrc.PushBuffer(buf)
		if ret != gst.FlowOK {
			return fmt.Errorf("failed to push audio buffer: %s", ret)
		} else {
			// log.Log(ctx, "pushed audio buffer", "presentation_timestamp", seg.pts, "duration", seg.dur)
		}
	}

	videoSrc.EndStream()
	audioSrc.EndStream()
	mp4sinkElem, err := pipeline.GetElementByName("mp4sink")
	if err != nil {
		return fmt.Errorf("failed to get mp4sink element: %w", err)
	}
	mp4sink := app.SinkFromElement(mp4sinkElem)
	if mp4sink == nil {
		return fmt.Errorf("failed to get mp4sink element: %w", err)
	}
	mp4sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, output),
	})

	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	return <-errCh
}
