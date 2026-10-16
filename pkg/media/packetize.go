package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
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

	pipelineElem, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err) //nolint:all
	}
	pipeline, ok := pipelineElem.(gst.Pipeline)
	if !ok {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: not a pipeline")
	}

	demuxBin, err := ConcatDemuxBin(ctx, seg, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create concat bin: %w", err)
	}

	if !pipeline.Add(demuxBin) {
		return nil, fmt.Errorf("failed to add demux bin to bin")
	}

	demuxBinPadVideoSrc := demuxBin.GetStaticPad("video_0")
	if demuxBinPadVideoSrc == nil {
		return nil, fmt.Errorf("failed to get demux bin video src pad")
	}

	demuxBinPadAudioSrc := demuxBin.GetStaticPad("audio_0")
	if demuxBinPadAudioSrc == nil {
		return nil, fmt.Errorf("failed to get demux bin audio src pad")
	}

	videoParse := pipeline.GetByName("videoparse")
	if videoParse == nil {
		return nil, fmt.Errorf("failed to get video parse element")
	}

	audioParse := pipeline.GetByName("audioparse")
	if audioParse == nil {
		return nil, fmt.Errorf("failed to get audio parse element")
	}

	linked := demuxBinPadVideoSrc.Link(videoParse.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link demux bin video src pad to video parse element: %v", linked)
	}

	linked = demuxBinPadAudioSrc.Link(audioParse.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link demux bin audio src pad to audio parse element: %v", linked)
	}

	videoSink := pipeline.GetByName("videoappsink")
	if videoSink == nil {
		return nil, fmt.Errorf("failed to get video appsink element")
	}

	audioSink := pipeline.GetByName("audioappsink")
	if audioSink == nil {
		return nil, fmt.Errorf("failed to get audio appsink element")
	}

	videoOutput := []rawSample{}
	audioOutput := []rawSample{}
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

	videoSink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: func(sink app.AppSink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			info, ok := buffer.Map(gst.MapRead)
			if !ok {
				log.Error(ctx, "failed to map video buffer")
				return gst.FlowError
			}
			defer info.Unmap()
			samples := info.Data()

			if !hasVideoSlice(samples) {
				pendingVideo = append(pendingVideo, samples...)
				return gst.FlowOK
			}
			if pendingVideo != nil {
				samples = append(pendingVideo, samples...)
				pendingVideo = nil
			} else {
				// the mapping is gone once this callback returns
				samples = bytes.Clone(samples)
			}

			videoOutput = append(videoOutput, newRawSample(samples, buffer))

			return gst.FlowOK
		},
		EOS: func(sink app.AppSink) {
			log.Debug(ctx, "videoappsink EOSFunc")
		},
	})

	segDur := time.Duration(0)

	audioSink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: func(sink app.AppSink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				log.Warn(ctx, "audioappsink NewSampleFunc EOS")
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples, err := copyBufferData(buffer)
			if err != nil {
				log.Error(ctx, "failed to read audio buffer", "error", err)
				return gst.FlowError
			}
			// log.Warn(ctx, "audioappsink NewSampleFunc", "sample", len(samples))

			audioOutput = append(audioOutput, newRawSample(samples, buffer))

			dur, ok := clockTimeDuration(buffer.Duration())
			if ok {
				segDur += dur
			} else {
				log.Error(ctx, "no audio duration", "samples", len(samples))
				return gst.FlowError
			}

			return gst.FlowOK
		},
		EOS: func(sink app.AppSink) {
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

	if ret := pipeline.SetState(gst.StatePlaying); ret == gst.StateChangeFailure {
		return nil, fmt.Errorf("failed to set pipeline to playing state")
	}

	defer func() {
		if ret := pipeline.SetState(gst.StateNull); ret == gst.StateChangeFailure {
			log.Error(ctx, "failed to set pipeline to null state")
		}
		if !pipeline.Remove(demuxBin) {
			log.Error(ctx, "failed to remove demux bin from bin")
		}
	}()

	err = <-busErr
	if err != nil {
		return nil, fmt.Errorf("packetize pipeline error filename=%s, error=%w", seg.Filepath, err)
	}

	video := finalizeSampleDurations(videoOutput, segDur)
	// segDur is audio-derived; a video-only segment would report Duration 0,
	// throwing off the sender's latency bookkeeping (the segment occupies the
	// sender for its full video span). Fall back to the video timeline.
	duration := segDur
	if duration == 0 {
		for _, s := range video {
			duration += s.Duration
		}
	}
	return &bus.PacketizedSegment{
		Video:    video,
		Audio:    finalizeSampleDurations(audioOutput, segDur),
		Duration: duration,
	}, nil
}

// rawSample is a demuxed sample plus the source timing needed to compute its
// WebRTC duration: its decode timestamp (DTS when the container carries one —
// decode order is monotonic even with B-frames — else PTS) and the buffer's
// own duration as a fallback.
type rawSample struct {
	data   []byte
	ts     time.Duration
	hasTS  bool
	bufDur time.Duration
	hasDur bool
}

func newRawSample(data []byte, buffer *gst.Buffer) rawSample {
	rs := rawSample{data: data}
	if ts, ok := clockTimeDuration(buffer.DTS()); ok {
		rs.ts, rs.hasTS = ts, true
	} else if pts, ok := clockTimeDuration(buffer.PTS()); ok {
		rs.ts, rs.hasTS = pts, true
	}
	if dur, ok := clockTimeDuration(buffer.Duration()); ok {
		rs.bufDur, rs.hasDur = dur, true
	}
	return rs
}

// finalizeSampleDurations converts raw buffer timing into the per-sample
// durations the WebRTC sender stamps and paces by. Each sample lasts until
// the next sample's timestamp — the source timeline, so non-uniform spacing
// (an encoder shedding frames under bandwidth pressure) is preserved instead
// of being respaced evenly. The last sample has no successor: it gets its own
// buffer duration, stretched to fill out the segment (total) when the track
// would otherwise end early — a sparse track's final frame must hold until
// the segment ends or its timeline falls behind the other track's.
func finalizeSampleDurations(raw []rawSample, total time.Duration) []bus.PacketizedSample {
	out := make([]bus.PacketizedSample, len(raw))
	var span time.Duration
	for i, rs := range raw {
		dur := rs.bufDur
		if i+1 < len(raw) && rs.hasTS && raw[i+1].hasTS && raw[i+1].ts > rs.ts {
			dur = raw[i+1].ts - rs.ts
		}
		out[i] = bus.PacketizedSample{Data: rs.data, Duration: dur}
		span += dur
	}
	if len(out) > 0 && total > span {
		out[len(out)-1].Duration += total - span
	}
	return out
}
