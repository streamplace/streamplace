package media

import (
	"bytes"
	"errors"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"

	"context"
	_ "embed"
	"fmt"
)

// MuxlBufferSize is the upper bound on how much fragmented-mp4 from
// gst we'll hold in memory ahead of the muxl-wasm segmenter. A
// well-behaved wasm drains continuously and never gets near this cap;
// a stuck wasm fills it up, after which Writes return
// ErrBoundedBufferOverflow and we error out the pipeline rather than
// jamming the whole MKV ingest chain with backpressure.
const MuxlBufferSize = 10 * 1024 * 1024

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

	initCh := make(chan []byte)
	segCh := make(chan []byte)
	buf := newBoundedBuffer(MuxlBufferSize)
	go func() {
		err := muxl.RunMuxlSegmenter(ctx, buf, initCh, segCh)
		if err != nil {
			log.Error(ctx, "error running muxl segmenter", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		buf.Close()
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
		NewSampleFunc: func(s *app.Sink) gst.FlowReturn {
			sample := s.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			gstBuf := sample.GetBuffer()
			bs := gstBuf.Map(gst.MapRead).Bytes()
			defer gstBuf.Unmap()
			if _, err := buf.Write(bs); err != nil {
				if errors.Is(err, ErrBoundedBufferOverflow) {
					size, _, max := buf.Stats()
					log.Error(ctx, "muxl buffer overflow — tearing pipeline down",
						"buffered_bytes", size, "max_bytes", max, "incoming_bytes", len(bs))
					s.ErrorMessage(gst.DomainCore, gst.CoreErrorFailed,
						"muxl segmenter stuck", err.Error())
				} else {
					log.Error(ctx, "error writing to muxl buffer", "error", err)
				}
				return gst.FlowError
			}
			return gst.FlowOK
		},
		// Closing the bounded buffer when gst signals EOS gives the
		// wasm an EOF on stdin so it can flush trailing init/segment
		// events. Without this, the wasm only sees EOF when ctx is
		// canceled in cleanup — which also kills the consumer
		// goroutine that would have delivered those events to cb.
		EOSFunc: func(_ *app.Sink) {
			log.Debug(ctx, "muxl appsink got EOS — closing buffer to flush wasm")
			buf.Close()
		},
	})

	return bin.Element, nil
}
