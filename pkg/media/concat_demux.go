package media

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
)

// silly technique to avoid leaking pads
func doNothing(self gst.Element, pad gst.Pad) {}

// doNothing for signals without a pad argument
func doNothingElement(self gst.Element) {}

// ConcatDemuxBin demuxes a single segment (seg.Data, a presentation MP4) into
// a "video_0" pad and one "audio_<k>" Opus pad per audio stream, k in order of
// the file's audio tracks, for audioTracks of them (at least one; a stream the
// segment lacks ends with EOS, one it has beyond audioTracks is dropped).
// Needs to be handled very carefully. In particular: users of this MUST
// cancel the passed context when they're done with the bin.
func ConcatDemuxBin(ctx context.Context, seg *bus.Seg, doH264Parse bool, audioTracks int) (gst.Bin, error) {
	audioTracks = max(audioTracks, 1)
	ctx = log.WithLogValues(ctx, "func", "ConcatDemuxBin")
	bin := gst.NewBin("seg-demux-bin").(gst.Bin)

	appSrc := gst.ElementFactoryMakeWithProperties("appsrc", map[string]interface{}{
		"name": "concat-appsrc",
	})
	if appSrc == nil {
		return nil, fmt.Errorf("failed to create appsrc element")
	}
	if !bin.Add(appSrc) {
		return nil, fmt.Errorf("failed to add appsrc to bin")
	}

	demux := gst.ElementFactoryMakeWithProperties("qtdemux", map[string]interface{}{
		"name": "concat-demux",
	})
	if demux == nil {
		return nil, fmt.Errorf("failed to create qtdemux element")
	}
	if !bin.Add(demux) {
		return nil, fmt.Errorf("failed to add qtdemux to bin")
	}

	if !appSrc.Link(demux) {
		return nil, fmt.Errorf("failed to link appsrc to qtdemux")
	}

	tmpl := demux.GetPadTemplateList()
	if tmpl == nil {
		return nil, fmt.Errorf("pad templates not found")
	}

	mq := gst.ElementFactoryMakeWithProperties("multiqueue", map[string]interface{}{
		"name":             "concat-demux-multiqueue",
		"max-size-time":    uint64(constants.QueueMaxSizeTime),
		"max-size-bytes":   uint32(constants.QueueMaxSizeBytes),
		"max-size-buffers": uint32(constants.QueueMaxSizeBuffers),
	})
	if mq == nil {
		return nil, fmt.Errorf("failed to create multiqueue element")
	}
	if !bin.Add(mq) {
		return nil, fmt.Errorf("failed to add multiqueue to bin")
	}

	mqVideoSink := mq.RequestPadSimple("sink_%u")
	if mqVideoSink == nil {
		return nil, fmt.Errorf("video sink pad not found")
	}
	mqVideoSrc := mq.GetStaticPad(strings.Replace(mqVideoSink.GetName(), "sink", "src", 1))
	if mqVideoSrc == nil {
		return nil, fmt.Errorf("video source pad not found")
	}

	// One multiqueue lane + opusparse per audio stream, ghosted as audio_<k>.
	mqAudioSinks := make([]gst.Pad, audioTracks)
	audioGhosts := make([]gst.Pad, audioTracks)
	for k := range mqAudioSinks {
		opusparse := gst.ElementFactoryMakeWithProperties("opusparse", map[string]interface{}{
			"name":                fmt.Sprintf("concat-demux-opusparse-%d", k),
			"disable-passthrough": true,
		})
		if opusparse == nil {
			return nil, fmt.Errorf("failed to create opusparse element")
		}
		if !bin.Add(opusparse) {
			return nil, fmt.Errorf("failed to add opusparse to bin")
		}
		opusparseSinkPad := opusparse.GetStaticPad("sink")
		if opusparseSinkPad == nil {
			return nil, fmt.Errorf("failed to get opusparse sink pad")
		}
		opusparseSrcPad := opusparse.GetStaticPad("src")
		if opusparseSrcPad == nil {
			return nil, fmt.Errorf("failed to get opusparse source pad")
		}
		mqAudioSinks[k] = mq.RequestPadSimple("sink_%u")
		if mqAudioSinks[k] == nil {
			return nil, fmt.Errorf("audio sink pad not found")
		}
		mqAudioSrc := mq.GetStaticPad(strings.Replace(mqAudioSinks[k].GetName(), "sink", "src", 1))
		if mqAudioSrc == nil {
			return nil, fmt.Errorf("audio source pad not found")
		}
		if linked := mqAudioSrc.Link(opusparseSinkPad); linked != gst.PadLinkOK {
			return nil, fmt.Errorf("failed to link opusparse sink pad to mq audio sink pad")
		}
		audioGhosts[k] = gst.NewGhostPad(fmt.Sprintf("audio_%d", k), opusparseSrcPad)
		if audioGhosts[k] == nil {
			return nil, fmt.Errorf("failed to create audio ghost pad")
		}
	}

	var videoGhost gst.Pad
	if doH264Parse {
		h264parse := gst.ElementFactoryMakeWithProperties("h264parse", map[string]interface{}{
			"name":                "concat-demux-h264parse",
			"config-interval":     int32(-1),
			"disable-passthrough": true,
		})
		if h264parse == nil {
			return nil, fmt.Errorf("failed to create h264parse element")
		}
		if !bin.Add(h264parse) {
			return nil, fmt.Errorf("failed to add h264parse to bin")
		}
		h264parseSinkPad := h264parse.GetStaticPad("sink")
		if h264parseSinkPad == nil {
			return nil, fmt.Errorf("failed to get h264parse sink pad")
		}
		h264parseSrcPad := h264parse.GetStaticPad("src")
		if h264parseSrcPad == nil {
			return nil, fmt.Errorf("failed to get h264parse source pad")
		}
		linked := mqVideoSrc.Link(h264parseSinkPad)
		if linked != gst.PadLinkOK {
			return nil, fmt.Errorf("failed to link h264parse sink pad to mq video sink pad")
		}

		videoGhost = gst.NewGhostPad("video_0", h264parseSrcPad)
		if videoGhost == nil {
			return nil, fmt.Errorf("failed to create video ghost pad")
		}
	} else {
		videoGhost = gst.NewGhostPad("video_0", mqVideoSrc)
		if videoGhost == nil {
			return nil, fmt.Errorf("failed to create video ghost pad")
		}
	}

	needed := 1 + audioTracks
	videoLinked := false
	audioLinked := make([]bool, audioTracks)

	// The signal handlers below are swapped for no-ops once they've done
	// their job (or the ctx ends, whichever comes first) so their closures
	// release the mq pads — and, via the pads' sticky events, tag lists and
	// samples — that they capture; without the swap those leak. The swap
	// happens on the ctx goroutine while the demux's dispatch thread reads
	// the same slot, so both sides go through an atomic (a plain assignment
	// here is a data race; a mutex held across dispatch risks blocking the
	// swap behind a stuck handler, pinning the closures).
	var padAdded atomic.Value
	padAdded.Store(func(self gst.Element, pad gst.Pad) {
		var downstreamPad gst.Pad
		name := pad.GetName()
		if strings.HasPrefix(name, "video_") {
			downstreamPad = mqVideoSink
			videoLinked = true
		} else if k, ok := strings.CutPrefix(name, "audio_"); ok {
			// qtdemux numbers its audio pads in the file's track order.
			idx, err := strconv.Atoi(k)
			if err != nil || idx >= audioTracks {
				log.Warn(ctx, "dropping audio stream beyond the requested tracks", "name", name)
				return
			}
			downstreamPad = mqAudioSinks[idx]
			audioLinked[idx] = true
		} else {
			log.Error(ctx, "unknown pad", "name", name, "direction", pad.GetDirection())
			// cancel()
			return
		}
		ret := pad.Link(downstreamPad)
		if ret != gst.PadLinkOK {
			log.Error(ctx, "failed to link demux to downstream pad", "name", name, "direction", pad.GetDirection(), "error", ret)
			// cancel()
			return
		}
		needed--
		if needed == 0 {
			padAdded.Store(doNothing)
		}
	})
	outerPadAdded := func(self gst.Element, pad gst.Pad) {
		padAdded.Load().(func(self gst.Element, pad gst.Pad))(self, pad)
	}

	// Both branches are wired up before the demux runs, but the segment may
	// not have both tracks: a stream whose encoder is shedding everything but
	// keyframes under bandwidth pressure can produce a segment with video and
	// no audio (or, in principle, the reverse). The demux only EOSes pads it
	// created, so a branch whose pad never appears would never see data OR
	// EOS — its sink never finishes, the pipeline never completes, and the
	// caller hangs until its timeout. When the demux declares it's done making
	// pads, flush any branch left orphaned with an EOS so the pipeline can
	// drain. Fires on the demux's streaming thread, same as pad-added, so the
	// linked flags need no synchronization.
	var noMorePads atomic.Value
	noMorePads.Store(func(self gst.Element) {
		if !videoLinked {
			log.Warn(ctx, "segment has no video track; completing video branch with EOS")
			mqVideoSink.SendEvent(gst.NewEventEOS())
		}
		for k, linked := range audioLinked {
			if !linked {
				log.Warn(ctx, "segment has no audio track; completing audio branch with EOS", "audio", k)
				mqAudioSinks[k].SendEvent(gst.NewEventEOS())
			}
		}
		noMorePads.Store(doNothingElement)
	})
	outerNoMorePads := func(self gst.Element) {
		noMorePads.Load().(func(self gst.Element))(self)
	}

	// Necessary to avoid leaking `mqVideoSink` and `mqAudioSink` from the
	// handlers in the case where we hit invalid data and they never fire.
	go func() {
		<-ctx.Done()
		padAdded.Store(doNothing)
		noMorePads.Store(doNothingElement)
	}()

	demux.ConnectPadAdded(outerPadAdded)
	demux.ConnectNoMorePads(outerNoMorePads)

	ok := bin.AddPad(videoGhost)
	if !ok {
		return nil, fmt.Errorf("failed to add video ghost pad to bin")
	}

	for _, ghost := range audioGhosts {
		if ok := bin.AddPad(ghost); !ok {
			return nil, fmt.Errorf("failed to add audio ghost pad to bin")
		}
	}

	src := appSrc.(app.AppSrc)
	src.SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, bytes.NewReader(seg.Data)),
	})

	return bin, nil
}
