package media

import (
	"bytes"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/mempool"
	"stream.place/streamplace/pkg/muxl"

	"context"
	_ "embed"
	"fmt"
	"io"
)

// MuxlSegmentElem creates a GStreamer bin that:
//  1. Muxes raw tracks to fMP4 via mp4mux
//  2. MUXL-segments the fMP4 into canonical init + per-track segments
//  3. Calls cb with combined init+segment bytes for signing
//  4. Feeds signed segments through a MUXL concatenator
//  5. Delivers the re-MUXLized events (with C2PA signatures baked in) to the mempool
func MuxlSegmentElem(ctx context.Context, cli *config.CLI, streamer string, doH264Parse bool, mp *mempool.Mempool, cb func(ctx context.Context, buf []byte, now int64) ([]byte, error)) (*gst.Element, error) {
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

	// Start the MUXL concatenator that will re-process signed segments.
	// Its structured events go to the mempool.
	concat := muxl.NewConcatenatorEvents(ctx, func(ev muxl.MuxlEvent) error {
		return mp.HandleEvent(ev)
	})

	r, w := io.Pipe()
	go func() {
		var initSeg []byte

		err := muxl.RunMuxlSegmenterEvents(ctx, r, func(ev muxl.MuxlEvent) error {
			switch ev.Type {
			case "init":
				initSeg = ev.Data
				log.Debug(ctx, "got init segment", "size", len(initSeg))
			case "segment":
				// Combine init + all tracks into a full fMP4 for signing
				combined := make([]byte, 0, len(initSeg))
				combined = append(combined, initSeg...)
				for _, data := range ev.Tracks {
					combined = append(combined, data...)
				}
				log.Debug(ctx, "got segment", "size", len(combined))
				cli.DumpDebugSegment(ctx, "muxl_segment_input.fmp4", bytes.NewReader(combined))

				// cb signs the segment, validates it, and returns the signed bytes
				signedBuf, err := cb(ctx, combined, time.Now().UnixMilli())
				if err != nil {
					return fmt.Errorf("error in signing callback: %w", err)
				}
				if signedBuf == nil {
					return nil
				}

				// Feed the signed fMP4 to the concatenator for re-MUXLization
				if err := concat.Write(signedBuf); err != nil {
					return fmt.Errorf("error writing to concatenator: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			log.Error(ctx, "error running muxl segmenter", "error", err)
		}
		// Signal end of stream to concatenator
		if err := concat.Close(); err != nil {
			log.Error(ctx, "error closing concatenator", "error", err)
		}
	}()

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	return bin.Element, nil
}
