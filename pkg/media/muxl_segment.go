package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

func MuxlSegmentElem(ctx context.Context, cli *config.CLI, streamer string, doH264Parse bool, cb func(ctx context.Context, buf []byte, now int64) error) (*gst.Element, error) {
	ctx = log.WithLogValues(ctx, "func", "MuxlSegmentElem")
	bin := gst.NewBin("muxl-segment-bin")
	elem, err := gst.NewElementWithProperties("mp4mux", map[string]any{
		"name":              "fmp4mux",
		"fragment-mode":     0,
		"fragment-duration": 1,
	})
	if err != nil {
		return nil, err
	}

	err = bin.Add(elem)
	if err != nil {
		return nil, fmt.Errorf("failed to add mp4mux to bin: %w", err)
	}

	videoPad := elem.GetRequestPad("video_%u")
	if videoPad == nil {
		return nil, fmt.Errorf("failed to get video pad")
	}
	videoGhost := gst.NewGhostPad("video_0", videoPad)
	if videoGhost == nil {
		return nil, fmt.Errorf("failed to create video ghost pad")
	}
	audioPad := elem.GetRequestPad("audio_%u")
	if audioPad == nil {
		return nil, fmt.Errorf("failed to get audio pad")
	}
	audioGhost := gst.NewGhostPad("audio_0", audioPad)
	if audioGhost == nil {
		return nil, fmt.Errorf("failed to create audio ghost pad")
	}

	ok := bin.AddPad(videoGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add video ghost pad to bin")
	}

	ok = bin.AddPad(audioGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add audio ghost pad to bin")
	}

	appsink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name": "muxl-appsink",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create appsink element: %w", err)
	}
	err = bin.Add(appsink)
	if err != nil {
		return nil, fmt.Errorf("failed to add appsink to bin: %w", err)
	}

	err = elem.Link(appsink)
	if err != nil {
		return nil, fmt.Errorf("failed to link mp4mux to appsink: %w", err)
	}

	r, w := io.Pipe()
	go func() {
		<-ctx.Done()
		r.Close()
	}()

	initCh := make(chan []byte)
	segCh := make(chan []byte)
	go func() {
		err := muxl.RunMuxlSegmenter(ctx, r, initCh, segCh)
		if err != nil {
			log.Error(ctx, "error running muxl segmenter", "error", err)
		}
	}()

	go func() {
		var initSeg []byte
		select {
		case <-ctx.Done():
			return
		case initSeg = <-initCh:
			log.Debug(ctx, "got init segment", "size", len(initSeg))
		}
		for {
			select {
			case <-ctx.Done():
				return
			case seg := <-segCh:
				log.Debug(ctx, "got segment", "size", len(seg))
				fullSeg := []byte{}
				fullSeg = append(fullSeg, initSeg...)
				fullSeg = append(fullSeg, seg...)
				cli.DumpDebugSegment(ctx, "muxl_segment_input.fmp4", bytes.NewReader(fullSeg))
				err := cb(ctx, fullSeg, time.Now().UnixMilli())
				if err != nil {
					log.Error(ctx, "error calling callback", "error", err)
				}
			}
		}
	}()

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	return bin.Element, nil
}
