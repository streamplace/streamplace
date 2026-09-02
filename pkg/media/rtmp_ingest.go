package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/llhls"
	"stream.place/streamplace/pkg/log"
)

type RTMPH264Data struct {
	AU  [][]byte
	PTS time.Duration
	DTS time.Duration
}

type RTMPAACData struct {
	AU  []byte
	PTS time.Duration
}

type RTMPSession struct {
	EventChan   chan any
	VideoTrack  *format.H264
	AudioTrack  *format.MPEG4Audio
	MediaSigner MediaSigner
}

const (
	llhlsParentDuration = 2 * time.Second
	llhlsPartDuration   = time.Second
	llhlsPartTarget     = 1100 * time.Millisecond
)

func h264VideoConfig(track *format.H264) llhls.VideoConfig {
	if track == nil {
		return llhls.VideoConfig{}
	}

	config := llhls.VideoConfig{}
	if len(track.SPS) >= 4 {
		config.Codec = fmt.Sprintf("avc1.%02x%02x%02x", track.SPS[1], track.SPS[2], track.SPS[3])
	}
	var sps h264.SPS
	if err := sps.Unmarshal(track.SPS); err == nil {
		config.Width = sps.Width()
		config.Height = sps.Height()
	}
	return config
}

func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner, streamerDID string, videoTrack *format.H264) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log.Log(ctx, "starting RTMP ingest", "url", rtmpURL, "ll_hls_requested", mm.cli != nil && mm.cli.LLHLS)
	// RTMP/FLV audio is already AAC, so keep it compressed through ingest.
	// Segment validation adds Opus when WebRTC needs that rendition.
	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
	}
	llEnabled := mm.cli != nil && mm.cli.LLHLS
	if llEnabled && gst.Find("isofmp4mux") == nil {
		log.Warn(ctx, "LL-HLS requested but isofmp4mux is unavailable; using conventional ingest")
		llEnabled = false
	}
	if llEnabled {
		// AAC and H.264 use separate renditions so each track can preserve its
		// own sample boundaries. The playlists share a program-date-time grid.
		pipelineSlice = append(pipelineSlice,
			fmt.Sprintf("isofmp4mux name=ll_video_mux fragment-duration=%d chunk-duration=%d ! appsink name=ll_video_sink sync=false async=false", llhlsParentDuration, llhlsPartDuration),
			fmt.Sprintf("isofmp4mux name=ll_audio_mux manual-split=true fragment-duration=%d chunk-duration=%d ! appsink name=ll_audio_sink sync=false async=false", llhlsParentDuration, llhlsPartDuration),
			"demux.audio ! queue ! aacparse name=audioenc ! audio/mpeg,mpegversion=4,stream-format=raw ! tee name=audio_tee",
			// The manual-split mux waits for a future AAC buffer to carry each
			// split marker. The default one-second queue time limit can fill
			// before that buffer arrives and deadlock the muxer.
			"audio_tee. ! queue name=ll_audio_queue max-size-time=0 ! ll_audio_mux.",
			"audio_tee. ! queue name=audio_signer_queue",
			"demux.video ! queue ! h264parse name=parse ! video/x-h264,stream-format=avc,alignment=au ! tee name=video_tee",
			"video_tee. ! queue ! ll_video_mux.",
			"video_tee. ! queue name=video_signer_queue",
		)
	} else {
		pipelineSlice = append(pipelineSlice,
			"demux.audio ! queue ! aacparse name=audioenc",
			"demux.video ! queue ! h264parse name=parse",
		)
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}

	parseEle, err := pipeline.GetElementByName("parse")
	if err != nil {
		return err
	}

	err = pipeline.Add(signer)
	if err != nil {
		return err
	}
	if llEnabled {
		if err := linkElementToPad(safeElement(pipeline, "video_signer_queue"), signer, "video_0"); err != nil {
			return err
		}
	} else if err = parseEle.Link(signer); err != nil {
		return err
	}
	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return err
	}
	if llEnabled {
		if err := linkElementToPad(safeElement(pipeline, "audio_signer_queue"), signer, "audio_0"); err != nil {
			return err
		}
		presentation := fmt.Sprintf("rtmp-%d", mm.nextIngestSession())
		window := mm.llWindow(streamerDID)
		window.SetVideoConfig(h264VideoConfig(videoTrack))
		if err := installCMAFBranch(ctx, pipeline, window, presentation); err != nil {
			return err
		}
	} else if err = audioenc.Link(signer); err != nil {
		return err
	}

	busErr := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		busErr <- err
	}()

	go mm.HandleKeyRevocation(ctx, ms, pipeline)

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}
	log.Log(ctx, "RTMP ingest pipeline playing", "ll_hls_enabled", llEnabled, "presentation", func() string {
		if llEnabled {
			return fmt.Sprintf("rtmp-%d", mm.ingestSessionSeq.Load())
		}
		return ""
	}())

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "error setting pipeline to null state", "error", err)
		}
	}()
	if llEnabled {
		if err := startLLAudioSplitter(ctx, pipeline); err != nil {
			return err
		}
	}

	err = <-busErr
	log.Log(ctx, "RTMP ingest pipeline stopped", "error", err)
	if err != nil {
		log.Error(ctx, "RTMP ingest pipeline exited with error", "error", err)
	}

	return err
}

func safeElement(pipeline *gst.Pipeline, name string) *gst.Element {
	element, _ := pipeline.GetElementByName(name)
	return element
}

func linkElementToPad(source, destination *gst.Element, sinkPadName string) error {
	if source == nil || destination == nil {
		return fmt.Errorf("LL-HLS element link: missing element")
	}
	src := source.GetStaticPad("src")
	sink := destination.GetStaticPad(sinkPadName)
	if src == nil || sink == nil {
		return fmt.Errorf("LL-HLS element link: missing source or %s sink pad", sinkPadName)
	}
	if result := src.Link(sink); result != gst.PadLinkOK {
		return fmt.Errorf("LL-HLS element link: %s", result.String())
	}
	return nil
}

func installCMAFBranch(ctx context.Context, pipeline *gst.Pipeline, window *llhls.Window, presentation string) error {
	// Both rendition playlists use the same program-date-time anchor.
	programDateTimeBase := time.Now().UTC()
	videoElement, err := pipeline.GetElementByName("ll_video_sink")
	if err != nil {
		return fmt.Errorf("LL-HLS CMAF sink: %w", err)
	}
	installCMAFSink(ctx, app.SinkFromElement(videoElement), &cmafTrackSink{
		presentation:        presentation,
		track:               "video",
		window:              window,
		generation:          1,
		partDuration:        llhlsPartDuration,
		programDateTimeBase: programDateTimeBase,
		partTarget:          llhlsPartTarget,
	})
	audioElement, err := pipeline.GetElementByName("ll_audio_sink")
	if err != nil {
		return fmt.Errorf("LL-HLS CMAF audio sink: %w", err)
	}
	installCMAFSink(ctx, app.SinkFromElement(audioElement), &cmafTrackSink{
		presentation:        presentation,
		track:               "audio",
		window:              window,
		generation:          1,
		partDuration:        llhlsPartDuration,
		programDateTimeBase: programDateTimeBase,
		partTarget:          llhlsPartTarget,
		audioOnly:           true,
	})
	return nil
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
