package media

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/go-gst/go-gst/gst"
	"github.com/google/uuid"
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
		config.FrameRate = finiteH264FrameRate(sps.FPS())
	}
	return config
}

func finiteH264FrameRate(fps float64) float64 {
	if fps <= 0 || math.IsNaN(fps) || math.IsInf(fps, 0) {
		return 0
	}
	return fps
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
		// AAC and H.264 use separate muxers so each track can preserve its own
		// sample boundaries. The playlists share a program-date-time grid.
		pipelineSlice = append(pipelineSlice, llhlsMuxerElements(llhlsPartDuration)...)
		pipelineSlice = append(pipelineSlice,
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
	presentation := ""
	if llEnabled {
		if err := linkElementToPad(safeElement(pipeline, "audio_signer_queue"), signer, "audio_0"); err != nil {
			return err
		}
		session := mm.nextIngestSession()
		presentation = fmt.Sprintf("rtmp-%d-%s", session, uuid.NewString())
		window := mm.replaceLLWindow(streamerDID)
		defer mm.removeLLWindow(streamerDID, presentation, window)
		window.SetVideoConfig(h264VideoConfig(videoTrack))
		if err := installCMAFBranch(ctx, pipeline, &llhlsIngestOutput{
			presentation: presentation,
			session:      session,
			window:       window,
		}); err != nil {
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
			return presentation
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
