package media

import (
	"context"
	"fmt"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

const (
	llhlsParentDuration = 2 * time.Second
	llhlsPartDuration   = time.Second
	llhlsPartTarget     = 1100 * time.Millisecond
)

// llhlsMuxerElements contains the shared LL-HLS CMAF muxers used by RTMP and WHIP.
// The input branches remain source-specific because WHIP must encode Opus to
// AAC while RTMP already supplies AAC.
func llhlsMuxerElements(videoChunkDuration time.Duration) []string {
	if videoChunkDuration <= 0 {
		videoChunkDuration = llhlsPartDuration
	}
	return []string{
		fmt.Sprintf("isofmp4mux name=ll_video_mux fragment-duration=%d chunk-duration=%d ! appsink name=ll_video_sink sync=false async=false", llhlsParentDuration, videoChunkDuration),
		fmt.Sprintf("isofmp4mux name=ll_audio_mux manual-split=true fragment-duration=%d chunk-duration=%d ! appsink name=ll_audio_sink sync=false async=false", llhlsParentDuration, llhlsPartDuration),
	}
}

func installCMAFBranch(ctx context.Context, pipeline *gst.Pipeline, output *llhlsIngestOutput) error {
	if output == nil {
		return fmt.Errorf("LL-HLS CMAF sink: missing output")
	}
	// Both rendition playlists use the same program-date-time anchor.
	programDateTimeBase := time.Now().UTC()
	installTrack := func(name, track string, audioOnly bool) error {
		element, err := pipeline.GetElementByName(name)
		if err != nil {
			return fmt.Errorf("LL-HLS CMAF sink: %w", err)
		}
		installCMAFSink(ctx, app.SinkFromElement(element), &cmafTrackSink{
			presentation:        output.presentation,
			session:             output.session,
			track:               track,
			window:              output.window,
			publish:             output.publish,
			generation:          1,
			partDuration:        llhlsPartDuration,
			programDateTimeBase: programDateTimeBase,
			partTarget:          llhlsPartTarget,
			audioOnly:           audioOnly,
		})
		return nil
	}
	if err := installTrack("ll_video_sink", "video", false); err != nil {
		return err
	}
	return installTrack("ll_audio_sink", "audio", true)
}

// startLLAudioSplitter triggers manual muxer splits from AAC buffer PTS. The
// muxer cuts between input buffers, preserving AAC frames at parent boundaries.
func startLLAudioSplitter(ctx context.Context, pipeline *gst.Pipeline) error {
	mux := safeElement(pipeline, "ll_audio_mux")
	if mux == nil {
		return fmt.Errorf("LL-HLS audio splitter: ll_audio_mux is missing")
	}
	queue := safeElement(pipeline, "ll_audio_queue")
	if queue == nil {
		return fmt.Errorf("LL-HLS audio splitter: ll_audio_queue is missing")
	}
	queueSrc := queue.GetStaticPad("src")
	if queueSrc == nil {
		return fmt.Errorf("LL-HLS audio splitter: ll_audio_queue has no source pad")
	}
	splitter := &llAudioSplitter{}
	probeID := queueSrc.AddProbe(gst.PadProbeTypeBuffer, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		buffer := info.GetBuffer()
		if buffer != nil {
			splitter.handleBuffer(ctx, pad, buffer)
		}
		return gst.PadProbeOK
	})
	go func() {
		<-ctx.Done()
		queueSrc.RemoveProbe(probeID)
	}()
	return nil
}

type llAudioSplitter struct {
	initialized  bool
	nextBoundary time.Duration
	splitIndex   uint64
}

func (s *llAudioSplitter) handleBuffer(ctx context.Context, pad *gst.Pad, buffer *gst.Buffer) {
	if buffer.GetFlags()&gst.BufferFlagDiscont != 0 {
		s.initialized = false
	}
	pts := buffer.PresentationTimestamp().AsDuration()
	if pts == nil {
		return
	}
	if !s.initialized {
		s.nextBoundary = *pts + llhlsPartDuration
		s.initialized = true
		return
	}
	if *pts < s.nextBoundary {
		return
	}

	chunk := (s.splitIndex+1)%2 == 1
	if err := emitLLAudioManualSplit(pad, chunk); err != nil {
		log.Warn(ctx, "LL-HLS audio split event failed", "chunk", chunk, "error", err)
		return
	}
	s.splitIndex++
	s.nextBoundary += llhlsPartDuration
}

func emitLLAudioSplit(mux *gst.Element, boundary gst.ClockTime) error {
	_, err := mux.Emit("split-at-running-time", uint64(boundary))
	return err
}

func emitLLAudioManualSplit(queueSrc *gst.Pad, chunk bool) error {
	if queueSrc == nil {
		return fmt.Errorf("LL-HLS audio splitter: nil queue source pad")
	}
	structure := gst.NewStructure("FMP4MuxSplitNow")
	if err := structure.SetValue("chunk", chunk); err != nil {
		return fmt.Errorf("set audio split event: %w", err)
	}
	if !queueSrc.PushEvent(gst.NewCustomEvent(gst.EventTypeCustomDownstream, structure)) {
		return fmt.Errorf("push audio split event")
	}
	return nil
}
