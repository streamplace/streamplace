package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

// element that takes the input stream, muxes to mp4, and signs the result
func SegmentElem(ctx context.Context, cb func(ctx context.Context, buf []byte, now int64) error) (*gst.Element, error) {
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
		err := sinkEle.SetProperty("sync", false)
		if err != nil {
			panic("error setting sync to false: " + err.Error())
		}
		appsink := app.SinkFromElement(sinkEle)
		if appsink == nil {
			panic("appsink should not be nil")
		}

		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, buf),
			EOSFunc: func(sink *app.Sink) {
				// ctx, span := otel.Tracer("signer").Start(ctx, "SegmentAndSignElem", trace.WithAttributes(
				// 	attribute.String("streamer", ms.Streamer()),
				// ))
				// defer span.End()
				resetTimer <- struct{}{}
				now := time.Now().UnixMilli()
				bs := buf.Bytes()

				err := cb(ctx, bs, now)
				if err != nil {
					log.Error(ctx, "error signing segment", "error", err)
					return
				}

			},
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect sink-added handler: %w", err)
	}

	return elem, nil
}

func (mm *MediaManager) SegmentAndSignElem(ctx context.Context, ms MediaSigner) (*gst.Element, error) {
	return SegmentElem(ctx, func(ctx context.Context, bs []byte, now int64) error {
		if mm.cli.SmearAudio {
			smearedBuf := &bytes.Buffer{}
			err := SmearAudioTimestamps(ctx, bytes.NewReader(bs), smearedBuf)
			if err != nil {
				return fmt.Errorf("error smearing audio timestamps: %w", err)
			}
			bs = smearedBuf.Bytes()
		}
		signedBs, err := ms.SignMP4(ctx, bytes.NewReader(bs), now)
		if err != nil {
			return err
		}
		return mm.ValidateMP4(ctx, bytes.NewReader(signedBs))
	})
}

func SegmentFile(ctx context.Context, input string, outDir string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)
	pipelineSlice := []string{
		"filesrc name=filesrc ! qtdemux name=demux",
		"demux. ! queue ! h264parse name=videoparse",
		"demux. ! queue ! opusparse name=audioparse",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating MKVIngest pipeline: %w", err)
	}

	srcele, err := pipeline.GetElementByName("filesrc")
	if err != nil {
		return err
	}
	if err := srcele.Set("location", input); err != nil {
		return err
	}

	videoParseEle, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return err
	}

	var segCount atomic.Int64

	segmenter, err := SegmentElem(ctx, func(ctx context.Context, buf []byte, now int64) error {
		seg := segCount.Load()
		segCount.Add(1)
		g.Go(func() error {
			fpath := fmt.Sprintf("%s/%d.mp4", outDir, seg)
			log.Log(ctx, "writing segment", "path", fpath)
			fd, err := os.Create(fpath)
			if err != nil {
				return err
			}
			defer fd.Close()
			_, err = fd.Write(buf)
			return err
		})
		return nil
	})
	if err != nil {
		return err
	}

	err = pipeline.Add(segmenter)
	if err != nil {
		return err
	}
	err = videoParseEle.Link(segmenter)
	if err != nil {
		return err
	}
	audioparse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return err
	}
	err = audioparse.Link(segmenter)
	if err != nil {
		return err
	}

	busErr := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		cancel()
		busErr <- err
	}()

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "error setting pipeline to null state", "error", err)
		}
	}()

	<-busErr
	err = g.Wait()
	if err != nil {
		return err
	}

	return nil
}
