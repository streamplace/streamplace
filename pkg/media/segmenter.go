package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
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

	// we didn't need faststart but i'm leaving this commented here in case
	// you want to change any other muxer properties in the future

	_, err = elem.Connect("muxer-added", func(split, muxEle *gst.Element) {
		err := muxEle.SetProperty("presentation-time", false)
		if err != nil {
			panic("error setting interleave-bytes to 4000: " + err.Error())
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect muxer-added handler: %w", err)
	}

	// channel to make sure data is emitted in order
	var ch chan struct{}

	_, err = elem.Connect("sink-added", func(split, sinkEle *gst.Element) {
		previousSegCh := ch
		mySegCh := make(chan struct{}, 1)
		ch = mySegCh
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
				now := time.Now().UnixMilli()
				resetTimer <- struct{}{}
				bs := buf.Bytes()

				if previousSegCh != nil {
					<-previousSegCh
				}
				err := cb(ctx, bs, now)
				if err != nil {
					log.Error(ctx, "error signing segment", "error", err)
					return
				}
				close(mySegCh)

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
		return mm.ValidateMP4(ctx, bytes.NewReader(signedBs), true)
	})
}

func SegmentFileUnsigned(ctx context.Context, input string, ch chan *SplitSegment) error {
	fd, err := os.OpenFile(input, os.O_RDONLY, 0644)
	log.Log(ctx, "reading file", "file", input)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	defer fd.Close()
	return SegmentUnsigned(ctx, fd, ch)
}

func SegmentUnsigned(ctx context.Context, input io.Reader, ch chan *SplitSegment) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux name=demux",
		"demux. ! queue ! h264parse ! rtph264pay ! rtph264depay ! h264parse name=videoparse",
		"demux. ! queue ! opusparse name=audioparse",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating MKVIngest pipeline: %w", err)
	}

	srcele, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}
	src := app.SrcFromElement(srcele)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
	})
	videoParseEle, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return err
	}

	segmenter, err := SegmentElem(ctx, func(ctx context.Context, buf []byte, now int64) error {
		ch <- &SplitSegment{
			Filename: fmt.Sprintf("%d.mp4", now),
			Data:     buf,
		}
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

	err = <-busErr
	if err != nil {
		return err
	}

	return nil
}
