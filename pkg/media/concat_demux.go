package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

// silly technique to avoid leaking pads
func doNothing(self *gst.Element, pad *gst.Pad) {}

// Function for demuxing a single segment. Needs to be handled very carefully.
// In particular: users of this MUST cancel the passed context when they're
// done with the bin.
func ConcatDemuxBin(ctx context.Context, seg *bus.Seg, doH264Parse bool) (*gst.Bin, error) {
	ctx = log.WithLogValues(ctx, "func", "ConcatDemuxBin")
	bin := gst.NewBin("seg-demux-bin")

	appSrc, err := gst.NewElementWithProperties("appsrc", map[string]interface{}{
		"name": "concat-appsrc",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create appsrc element: %w", err)
	}
	err = bin.Add(appSrc)
	if err != nil {
		return nil, fmt.Errorf("failed to add appsrc to bin: %w", err)
	}

	demux, err := gst.NewElementWithProperties("qtdemux", map[string]interface{}{
		"name": "concat-demux",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qtdemux element: %w", err)
	}
	err = bin.Add(demux)
	if err != nil {
		return nil, fmt.Errorf("failed to add qtdemux to bin: %w", err)
	}

	err = appSrc.Link(demux)
	if err != nil {
		return nil, fmt.Errorf("failed to link appsrc to qtdemux: %w", err)
	}

	tmpl := demux.GetPadTemplates()
	if tmpl == nil {
		return nil, fmt.Errorf("pad templates not found")
	}

	mq, err := gst.NewElementWithProperties("multiqueue", map[string]interface{}{
		"name": "concat-demux-multiqueue",
		// "max-size-time":    uint(0), // default: 2000000000, 2 seconds
		// "max-size-bytes":   uint(0), // default: 10485760, 10MiB
		// "max-size-buffers": uint(0), // default: 5, 5 buffers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create multiqueue element: %w", err)
	}
	err = bin.Add(mq)
	if err != nil {
		return nil, fmt.Errorf("failed to add multiqueue to bin: %w", err)
	}
	// err = mq.SetProperty("max-size-time", uint64(200000000000))
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to set max-size-time: %w", err)
	// }
	// err = mq.SetProperty("max-size-bytes", uint(1048576000))
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to set max-size-bytes: %w", err)
	// }
	// err = mq.SetProperty("max-size-buffers", uint(500))
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to set max-size-buffers: %w", err)
	// }

	opusparse, err := gst.NewElementWithProperties("opusparse", map[string]interface{}{
		"name":                "concat-demux-opusparse",
		"disable-passthrough": true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create opusparse element: %w", err)
	}
	err = bin.Add(opusparse)
	if err != nil {
		return nil, fmt.Errorf("failed to add opusparse to bin: %w", err)
	}
	opusparseSinkPad := opusparse.GetStaticPad("sink")
	if opusparseSinkPad == nil {
		return nil, fmt.Errorf("failed to get opusparse sink pad")
	}
	opusparseSrcPad := opusparse.GetStaticPad("src")
	if opusparseSrcPad == nil {
		return nil, fmt.Errorf("failed to get opusparse source pad")
	}

	mqVideoSink := mq.GetRequestPad("sink_%u")
	if mqVideoSink == nil {
		return nil, fmt.Errorf("video sink pad not found")
	}

	mqAudioSink := mq.GetRequestPad("sink_%u")
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

	var videoGhost *gst.GhostPad
	if doH264Parse {
		h264parse, err := gst.NewElementWithProperties("h264parse", map[string]interface{}{
			"name":                "concat-demux-h264parse",
			"config-interval":     -1,
			"disable-passthrough": true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create h264parse element: %w", err)
		}
		err = bin.Add(h264parse)
		if err != nil {
			return nil, fmt.Errorf("failed to add h264parse to bin: %w", err)
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

	var padAdded func(self *gst.Element, pad *gst.Pad)
	// the defer funcs are needed to avoid leaking pads for some reason
	padAdded = func(self *gst.Element, pad *gst.Pad) {
		log.Debug(ctx, "demux pad-added", "name", pad.GetName(), "direction", pad.GetDirection())
		var downstreamPad *gst.Pad
		if strings.HasPrefix(pad.GetName(), "video_") {
			downstreamPad = mqVideoSink
		} else if strings.HasPrefix(pad.GetName(), "audio_") {
			downstreamPad = mqAudioSink
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
			padAdded = doNothing
		}
	}
	outerPadAdded := func(self *gst.Element, pad *gst.Pad) {
		padAdded(self, pad)
	}

	// Necessary to avoid leaking `mqVideoSink` and `mqAudioSink` from the
	// pad-added function in the case where we hit invalid data and
	// pad-added never fires.
	go func() {
		<-ctx.Done()
		padAdded = doNothing
	}()

	_, err = demux.Connect("pad-added", outerPadAdded)
	if err != nil {
		return nil, fmt.Errorf("failed to connect demux pad-added signal: %w", err)
	}

	ok := bin.AddPad(videoGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add video ghost pad to bin")
	}

	ok = bin.AddPad(audioGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add audio ghost pad to bin")
	}

	src := app.SrcFromElement(appSrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, bytes.NewReader(seg.Data)),
	})

	return bin, nil
}
