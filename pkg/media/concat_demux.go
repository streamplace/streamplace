package media

import (
	"bytes"
	"context"
	"fmt"
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

// Function for demuxing a single segment. Needs to be handled very carefully.
// In particular: users of this MUST cancel the passed context when they're
// done with the bin.
func ConcatDemuxBin(ctx context.Context, seg *bus.Seg, doH264Parse bool) (gst.Bin, error) {
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

	opusparse := gst.ElementFactoryMakeWithProperties("opusparse", map[string]interface{}{
		"name":                "concat-demux-opusparse",
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

	mqVideoSink := mq.RequestPadSimple("sink_%u")
	if mqVideoSink == nil {
		return nil, fmt.Errorf("video sink pad not found")
	}

	mqAudioSink := mq.RequestPadSimple("sink_%u")
	if mqAudioSink == nil {
		return nil, fmt.Errorf("audio sink pad not found")
	}

	mqVideoSrc := mq.GetStaticPad("src_0")
	if mqVideoSrc == nil {
		return nil, fmt.Errorf("video source pad not found")
	}

	mqAudioSrc := mq.GetStaticPad("src_1")
	if mqAudioSrc == nil {
		return nil, fmt.Errorf("audio source pad not found")
	}

	linked := mqAudioSrc.Link(opusparseSinkPad)
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link opusparse sink pad to mq audio sink pad")
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

	audioGhost := gst.NewGhostPad("audio_0", opusparseSrcPad)
	if audioGhost == nil {
		return nil, fmt.Errorf("failed to create audio ghost pad")
	}

	needed := 2
	videoLinked := false
	audioLinked := false

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
		if strings.HasPrefix(pad.GetName(), "video_") {
			downstreamPad = mqVideoSink
			videoLinked = true
		} else if strings.HasPrefix(pad.GetName(), "audio_") {
			downstreamPad = mqAudioSink
			audioLinked = true
		} else {
			log.Error(ctx, "unknown pad", "name", pad.GetName(), "direction", pad.GetDirection())
			// cancel()
			return
		}
		ret := pad.Link(downstreamPad)
		if ret != gst.PadLinkOK {
			log.Error(ctx, "failed to link demux to downstream pad", "name", pad.GetName(), "direction", pad.GetDirection(), "error", ret)
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
		if !audioLinked {
			log.Warn(ctx, "segment has no audio track; completing audio branch with EOS")
			mqAudioSink.SendEvent(gst.NewEventEOS())
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

	ok = bin.AddPad(audioGhost)
	if !ok {
		return nil, fmt.Errorf("failed to add audio ghost pad to bin")
	}

	src := appSrc.(app.AppSrc)
	src.SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, bytes.NewReader(seg.Data)),
	})

	return bin, nil
}
