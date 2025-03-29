package media

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

// element that takes the input stream, muxes to mp4, and signs the result
func (mm *MediaManager) SegmentAndSignElem(ctx context.Context, ms MediaSigner) (*gst.Element, error) {
	// elem, err := gst.NewElement("splitmuxsink name=splitter async-finalize=true sink-factory=appsink muxer-factory=matroskamux max-size-bytes=1")
	elem, err := gst.NewElementWithProperties("splitmuxsink", map[string]any{
		"name":           "signer",
		"async-finalize": true,
		"sink-factory":   "appsink",
		"muxer-factory":  "mp4mux",
		"max-size-bytes": 1,
	})
	if err != nil {
		return nil, err
	}

	p := elem.GetRequestPad("video")
	if p == nil {
		return nil, fmt.Errorf("failed to get video pad")
	}
	p = elem.GetRequestPad("audio_%u")
	if p == nil {
		return nil, fmt.Errorf("failed to get audio pad")
	}

	resetTimer := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-resetTimer:
				continue
			case <-time.After(time.Second * 10):
				log.Warn(ctx, "no new segment for 10 seconds")
				elem.ErrorMessage(gst.DomainCore, gst.CoreErrorFailed, "No new segment for 10 seconds", "No new segment for 10 seconds (debug)")
				return
			}
		}
	}()

	elem.Connect("sink-added", func(split, sinkEle *gst.Element) {
		buf := &bytes.Buffer{}
		appsink := app.SinkFromElement(sinkEle)
		if appsink == nil {
			panic("appsink should not be nil")
		}
		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, buf),
			EOSFunc: func(sink *app.Sink) {
				resetTimer <- struct{}{}
				now := time.Now().UnixMilli()
				bs, err := ms.SignMP4(ctx, bytes.NewReader(buf.Bytes()), now)
				if err != nil {
					log.Error(ctx, "error signing segment", "error", err)
					return
				}

				err = mm.ValidateMP4(ctx, bytes.NewReader(bs))
				if err != nil {
					log.Error(ctx, "error validating segment", "error", err)
					return
				}
			},
		})
	})

	return elem, nil
}
