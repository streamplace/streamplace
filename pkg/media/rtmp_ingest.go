package media

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
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

func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner, streamerDID string) error {
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
	if llEnabled && gst.Find("cmafmux") == nil {
		log.Warn(ctx, "LL-HLS requested but cmafmux is unavailable; using conventional ingest")
		llEnabled = false
	}
	if llEnabled {
		pipelineSlice = append(pipelineSlice,
			"demux.audio ! queue ! aacparse name=audioenc ! audio/mpeg,mpegversion=4,stream-format=raw ! tee name=audio_tee",
			"audio_tee. ! queue ! cmafmux name=audio_ll_mux fragment-duration=1000000000 chunk-duration=200000000 ! appsink name=audio_ll_sink sync=false",
			"audio_tee. ! queue name=audio_signer_queue",
			"demux.video ! queue ! h264parse name=parse ! video/x-h264,stream-format=avc,alignment=au ! tee name=video_tee",
			"video_tee. ! queue ! cmafmux name=video_ll_mux fragment-duration=1000000000 chunk-duration=200000000 ! appsink name=video_ll_sink sync=false",
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
	for _, track := range []struct {
		name string
		id   string
		tee  string
	}{
		{name: "video_ll_sink", id: "video", tee: "video_tee"},
		{name: "audio_ll_sink", id: "audio", tee: "audio_tee"},
	} {
		element, err := pipeline.GetElementByName(track.name)
		if err != nil {
			return fmt.Errorf("LL-HLS %s sink: %w", track.id, err)
		}
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
		installCMAFSink(ctx, app.SinkFromElement(element), &cmafTrackSink{
			presentation: presentation,
			track:        track.id,
			window:       window,
			generation:   1,
			partDuration: 200 * time.Millisecond,
		})
	}
	return nil
}
