package media

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/globalerror"
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
			case <-time.After(time.Second * 30):
				log.Warn(ctx, "no new segment for 30 seconds")
				elem.ErrorMessage(gst.DomainCore, gst.CoreErrorFailed, "No new segment for 30 seconds", "No new segment for 30 seconds (debug)")
				return
			}
		}
	}()

	_, err = elem.Connect("sink-added", func(split, sinkEle *gst.Element) {
		buf := &bytes.Buffer{}
		appsink := app.SinkFromElement(sinkEle)
		if appsink == nil {
			panic("appsink should not be nil")
		}

		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, buf),
			EOSFunc: func(sink *app.Sink) {
				log.Log(ctx, "EOSFunc called - segment complete", "streamer", ms.Streamer(), "bufferSize", buf.Len())
				ctx, span := otel.Tracer("signer").Start(ctx, "SegmentAndSignElem", trace.WithAttributes(
					attribute.String("streamer", ms.Streamer()),
				))
				defer span.End()
				resetTimer <- struct{}{}
				now := time.Now().UnixMilli()
				bs := buf.Bytes()
				log.Debug(ctx, "processing segment data", "segmentSize", len(bs))
				if mm.cli.SmearAudio {
					smearedBuf := &bytes.Buffer{}
					err := SmearAudioTimestamps(ctx, bytes.NewReader(buf.Bytes()), smearedBuf)
					if err != nil {
						log.Error(ctx, "error smearing audio timestamps", "error", err)
						return
					}
					bs = smearedBuf.Bytes()
				}
				bs, err := ms.SignMP4(ctx, bytes.NewReader(bs), now)
				if err != nil {
					log.Error(ctx, "error signing segment", "error", err)
					return
				}

				log.Debug(ctx, "calling ValidateMP4 for segment", "segmentSize", len(bs))
				err = mm.ValidateMP4(ctx, bytes.NewReader(bs), true)
				if err != nil {
					log.Error(ctx, "error validating segment", "error", err)
					globalerror.GlobalError(err)
					// We don't want to stop the pipeline here because we want to keep the stream
					// alive. Lots of weird invalid data coming in from WebRTC connections on
					// phones. Better we drop one weird segment than force the stream to restart
					return
				}
				log.Log(ctx, "segment successfully validated and processed", "segmentSize", len(bs))
			},
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect sink-added handler: %w", err)
	}

	return elem, nil
}
