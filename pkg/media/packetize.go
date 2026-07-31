package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// hasVideoSlice reports whether byte-stream H264 data contains a VCL NAL
// (coded slice, nal_unit_type 1–5) — i.e. an actual picture, as opposed to
// bare parameter sets or SEI metadata.
func hasVideoSlice(data []byte) bool {
	for i := 0; i+3 < len(data); i++ {
		if data[i] != 0 || data[i+1] != 0 {
			continue
		}
		var nalIdx int
		if data[i+2] == 1 {
			nalIdx = i + 3
		} else if data[i+2] == 0 && i+4 < len(data) && data[i+3] == 1 {
			nalIdx = i + 4
		} else {
			continue
		}
		if nalIdx < len(data) {
			if t := data[nalIdx] & 0x1f; t >= 1 && t <= 5 {
				return true
			}
		}
		i = nalIdx // skip the matched start code (else a 4-byte code re-matches as 3-byte)
	}
	return false
}

// take in a segment and return a bunch of packets suitable for webrtc
func Packetize(ctx context.Context, cli *config.CLI, seg *bus.Seg) (*bus.PacketizedSegment, error) {

	uu, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx = log.WithLogValues(ctx, "func", "Packetize", "uuid", uu.String())

	// WebRTC playback needs Opus. From a dual-codec segment select video+Opus
	// and present it as a flat MP4, so the demux below yields exactly one
	// (Opus) audio pad for opusparse — no extra AAC pad to strand.
	if len(seg.Muxl) > 0 {
		opusM4s, err := filterSegmentToCodec(ctx, seg.Muxl, true)
		if err != nil {
			return nil, fmt.Errorf("select opus audio: %w", err)
		}
		var flat bytes.Buffer
		if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(opusM4s), "flat", &flat); err != nil {
			return nil, fmt.Errorf("wrap opus segment: %w", err)
		}
		seg = &bus.Seg{Filepath: seg.Filepath, Data: flat.Bytes(), Muxl: opusM4s}
	}

	cli.DumpDebugSegment(ctx, fmt.Sprintf("packetize-input-%s.mp4", uu.String()), bytes.NewReader(seg.Data))

	pipelineSlice := []string{
		fmt.Sprintf("%s name=videoparse ! h264parse ! video/x-h264,stream-format=byte-stream ! appsink sync=false name=videoappsink", constants.Queue2Big),
		fmt.Sprintf("%s name=audioparse ! opusparse ! appsink sync=false name=audioappsink", constants.Queue2Big),
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err) //nolint:all
	}

	demuxBin, err := ConcatDemuxBin(ctx, seg, true)
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

	// Slice-less video data (parameter sets / SEI metadata with no picture)
	// held back for the next real frame. Streams with embedded closed captions
	// carry a trailing caption SEI after some frames' slices; when h264parse
	// re-parses the byte stream it splits that SEI into its own timestamp-less
	// AU. Sent to WebRTC as a standalone "frame" it breaks strict decoders
	// (iOS VideoToolbox errors on a picture-less access unit, and with PLI
	// unanswered the picture stays broken until the next keyframe). Prepending
	// it to the following frame is where a caption SEI normally lives, so
	// caption-aware receivers still get it. A remainder at EOS has no frame to
	// ride with and is dropped.
	var pendingVideo []byte

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

			if !hasVideoSlice(samples) {
				pendingVideo = append(pendingVideo, samples...)
				return gst.FlowOK
			}
			if pendingVideo != nil {
				samples = append(pendingVideo, samples...)
				pendingVideo = nil
			}

			videoOutput = append(videoOutput, samples)

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
			// log.Warn(ctx, "audioappsink NewSampleFunc", "sample", len(samples))

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
