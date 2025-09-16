package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

// take in a segment and return a bunch of packets suitable for webrtc
func Packetize(ctx context.Context, seg *bus.Seg) (*bus.PacketizedSegment, error) {

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pipelineSlice := []string{
		"h264parse name=videoparse ! video/x-h264,stream-format=byte-stream ! appsink sync=false name=videoappsink",
		"opusparse name=audioparse ! appsink sync=false name=audioappsink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err) //nolint:all
	}

	demuxBin, err := ConcatDemuxBin(ctx, seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create concat bin: %w", err)
	}

	err = pipeline.Add(demuxBin.Element)
	if err != nil {
		return nil, fmt.Errorf("failed to add demux bin to bin: %w", err)
	}

	demuxBinPadVideoSrc := demuxBin.GetStaticPad("video_0")
	if demuxBinPadVideoSrc == nil {
		return nil, fmt.Errorf("failed to get demux bin video src pad")
	}

	demuxBinPadAudioSrc := demuxBin.GetStaticPad("audio_0")
	if demuxBinPadAudioSrc == nil {
		return nil, fmt.Errorf("failed to get demux bin audio src pad")
	}

	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return nil, fmt.Errorf("failed to get video parse element: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return nil, fmt.Errorf("failed to get audio parse element: %w", err)
	}

	linked := demuxBinPadVideoSrc.Link(videoParse.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link demux bin video src pad to video parse element: %v", linked)
	}

	linked = demuxBinPadAudioSrc.Link(audioParse.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link demux bin audio src pad to audio parse element: %v", linked)
	}

	videoSink, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get video appsink element: %w", err)
	}
	if videoSink == nil {
		return nil, fmt.Errorf("failed to get video appsink element")
	}

	audioSink, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get audio appsink element: %w", err)
	}
	if audioSink == nil {
		return nil, fmt.Errorf("failed to get audio appsink element")
	}

	videoOutput := [][]byte{}
	audioOutput := [][]byte{}
	// eosCh := make(chan struct{})

	videoappsink := app.SinkFromElement(videoSink)
	videoappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Bytes()

			videoOutput = append(videoOutput, samples)

			// clockTime := buffer.Duration()
			// dur := clockTime.AsDuration()
			// if dur != nil {
			// 	log.Log(ctx, "video duration", "duration", *dur)
			// } else {
			// 	log.Error(ctx, "no video duration", "samples", len(samples))
			// }

			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			log.Debug(ctx, "videoappsink EOSFunc")
		},
	})

	segDur := time.Duration(0)

	audioappsink := app.SinkFromElement(audioSink)
	audioappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				log.Warn(ctx, "audioappsink NewSampleFunc EOS")
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Bytes()

			audioOutput = append(audioOutput, samples)

			clockTime := buffer.Duration()
			dur := clockTime.AsDuration()
			if dur != nil {
				segDur += *dur
			} else {
				log.Error(ctx, "no audio duration", "samples", len(samples))
				return gst.FlowError
			}

			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			log.Debug(ctx, "audioappsink EOSFunc")
		},
	})

	busErr := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		if err != nil {
			log.Log(ctx, "pipeline error", "error", err)
		}
		busErr <- err
	}()

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return nil, fmt.Errorf("failed to set pipeline to playing state: %w", err)
	}

	defer func() {
		err = pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline to null state", "error", err)
		}
		err = pipeline.Remove(demuxBin.Element)
		if err != nil {
			log.Error(ctx, "failed to remove demux bin from bin", "error", err)
		}
	}()

	err = <-busErr
	if err != nil {
		return nil, fmt.Errorf("packetize pipeline error filename=%s, error=%w", seg.Filepath, err)
	}

	return &bus.PacketizedSegment{
		Video:    videoOutput,
		Audio:    audioOutput,
		Duration: segDur,
	}, nil
}
