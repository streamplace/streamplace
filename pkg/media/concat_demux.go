package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
)

// silly technique to avoid leaking pads
func doNothing(self *gst.Element, pad *gst.Pad) {}

func ConcatDemuxBin(ctx context.Context, seg *segchanman.Seg) (*gst.Bin, error) {
	ctx = log.WithLogValues(ctx, "func", "SegDemuxBin")
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
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create multiqueue element: %w", err)
	}
	err = bin.Add(mq)
	if err != nil {
		return nil, fmt.Errorf("failed to add multiqueue to bin: %w", err)
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

	videoGhost := gst.NewGhostPad("video_0", mqVideoSrc)
	if videoGhost == nil {
		return nil, fmt.Errorf("failed to create video ghost pad")
	}

	audioGhost := gst.NewGhostPad("audio_0", mqAudioSrc)
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
			// defer func() { mqVideoSink = nil }()
		} else if strings.HasPrefix(pad.GetName(), "audio_") {
			downstreamPad = mqAudioSink
			// defer func() { mqAudioSink = nil }()
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
