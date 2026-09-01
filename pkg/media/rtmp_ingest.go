package media

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
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
	// Mint the source audio: RTMP/FLV audio is already AAC, so pass it through
	// (aacparse) rather than transcoding to Opus. The validate path completes
	// each segment to also carry Opus when a consumer (WebRTC) needs it — so
	// the old RTMP-AAC→Opus→HLS-AAC double-transcode is gone.
	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
	}
	llEnabled := mm.cli != nil && mm.cli.LLHLS
	if llEnabled && gst.Find("isofmp4mux") == nil {
		log.Warn(ctx, "LL-HLS requested but isofmp4mux is unavailable; using conventional ingest")
		llEnabled = false
	}
	if llEnabled {
		// Keep audio and video in one CMAF output so both tracks are published
		// atomically with one set of parent and part boundaries.
		pipelineSlice = append(pipelineSlice,
			fmt.Sprintf("isofmp4mux name=ll_video_mux fragment-duration=%d chunk-duration=%d ! appsink name=ll_video_sink sync=false", llhlsParentDuration, llhlsPartDuration),
			"demux.audio ! queue ! aacparse name=audioenc ! audio/mpeg,mpegversion=4,stream-format=raw ! tee name=audio_tee",
			"audio_tee. ! queue ! ll_video_mux.",
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
	videoElement, err := pipeline.GetElementByName("ll_video_sink")
	if err != nil {
		return fmt.Errorf("LL-HLS CMAF sink: %w", err)
	}
	for _, track := range []struct {
		id  string
		tee string
	}{
		{id: "video", tee: "video_tee"},
		{id: "audio", tee: "audio_tee"},
	} {
		tee := safeElement(pipeline, track.tee)
		if tee == nil {
			return fmt.Errorf("LL-HLS %s tee is missing", track.id)
		}
		pad := tee.GetStaticPad("sink")
		if pad == nil {
			return fmt.Errorf("LL-HLS %s tee has no sink pad", track.id)
		}
		var buffers atomic.Uint64
		pad.AddProbe(gst.PadProbeTypeBuffer, func(_ *gst.Pad, _ *gst.PadProbeInfo) gst.PadProbeReturn {
			if n := buffers.Add(1); n <= 3 || n%300 == 0 {
				log.Log(ctx, "parsed RTMP buffer reached LL-HLS tee", "track", track.id, "buffer", n)
			}
			return gst.PadProbeOK
		})
	}
	installCMAFSink(ctx, app.SinkFromElement(videoElement), &cmafTrackSink{
		presentation: presentation,
		track:        "video",
		window:       window,
		generation:   1,
		partDuration: llhlsPartDuration,
	})
	return nil
}
