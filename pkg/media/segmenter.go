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
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

// For testing. Normally,  We don't want to stop the pipeline upon a
// segmentation error because we want to keep the stream alive. Lots
// of weird invalid data coming in from WebRTC connections on phones.
// Better we drop one weird segment than force the stream to restart.
// But for tests, we want (sometimes) to know if there's a problem.
var FatalSegmentationErrors = false

// element that takes the input stream, muxes to mp4, and signs the result
func SegmentElem(ctx context.Context, cli *config.CLI, streamer string, doH264Parse bool, cb func(ctx context.Context, buf []byte, now int64) error) (*gst.Element, error) {
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
			panic("error setting presentation-time to false: " + err.Error())
		}
		err = muxEle.SetProperty("interleave-bytes", InterleaveBytes)
		if err != nil {
			panic("error setting interleave-bytes" + err.Error())
		}
		err = muxEle.SetProperty("interleave-time", InterleaveTime)
		if err != nil {
			panic("error setting interleave-time" + err.Error())
		}
		err = muxEle.SetProperty("faststart", true)
		if err != nil {
			panic("error setting faststart" + err.Error())
		}
		err = muxEle.SetProperty("movie-timescale", uint(60000))
		if err != nil {
			panic("error setting movie-timescale" + err.Error())
		}
		err = muxEle.SetProperty("trak-timescale", uint(60000))
		if err != nil {
			panic("error setting trak-timescale" + err.Error())
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
				bs := buf.Bytes()

				if previousSegCh != nil {
					<-previousSegCh
				}
				resetTimer <- struct{}{}
				convergeAndSign := func() error {
					convergedBs, err := ConvergeSegment(ctx, cli, bs, now, streamer, doH264Parse)
					if err != nil {
						log.Error(ctx, "error converging segment", "error", err)
					} else {
						bs = convergedBs
					}
					log.Debug(ctx, "signing segment", "size", len(bs))
					err = cb(ctx, bs, now)
					if err != nil {
						return fmt.Errorf("error signing segment: %w", err)
					}
					return nil
				}
				err := func() error {
					convergeDone := make(chan error)
					go func() {
						convergeDone <- convergeAndSign()
					}()
					select {
					case <-ctx.Done():
						return ctx.Err()
					case err := <-convergeDone:
						return err
					case <-time.After(time.Second * 3):
						go func() {
							err = cli.DataFileWrite([]string{"debug-recordings", streamer, fmt.Sprintf("converge-timeout-%d.mp4", now)}, bytes.NewReader(bs), true)
							if err != nil {
								log.Error(ctx, "error writing debug recording", "error", err)
							}
						}()
						return fmt.Errorf("timeout converging segment")
					}
				}()
				close(mySegCh)
				if err != nil {
					log.Error(ctx, "error in segmenter", "error", err)
					if FatalSegmentationErrors {
						sink.ErrorMessage(gst.DomainCore, gst.CoreErrorFailed, "error in segmenter", err.Error())
						return
					}
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
	return SegmentElem(ctx, mm.cli, ms.Streamer(), false, func(ctx context.Context, bs []byte, now int64) error {
		if mm.cli.SmearAudio {
			smearedBuf := &bytes.Buffer{}
			err := RewriteAudioTimestamps(ctx, mm.cli, bytes.NewReader(bs), smearedBuf, true)
			if err != nil {
				return fmt.Errorf("error smearing audio timestamps: %w", err)
			}
			bs = smearedBuf.Bytes()
		}
		signedBs, err := ms.SignMP4(ctx, bytes.NewReader(bs), now)
		if err != nil {
			return fmt.Errorf("error calling SignMP4: %w", err)
		}
		log.Debug(ctx, "signed segment", "size", len(signedBs))
		err = mm.ValidateMP4(ctx, bytes.NewReader(signedBs), true)
		if err != nil {
			mm.cli.DumpDebugSegment(ctx, "just-signed-segment.mp4", bytes.NewReader(signedBs))
			return fmt.Errorf("error validating just-signed segment: %w", err)
		}
		return nil
	})
}

func SegmentFileUnsigned(ctx context.Context, cli *config.CLI, streamer string, input string, ch chan *SplitSegment) error {
	fd, err := os.OpenFile(input, os.O_RDONLY, 0644)
	log.Log(ctx, "reading file", "file", input)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	defer fd.Close()
	return SegmentUnsigned(ctx, cli, streamer, fd, false, ch)
}

func SegmentUnsigned(ctx context.Context, cli *config.CLI, streamer string, input io.Reader, doH264Parse bool, ch chan *SplitSegment) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux name=demux",
		"demux. ! queue ! h264parse name=videoparse disable-passthrough=true config-interval=0",
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

	segmenter, err := SegmentElem(ctx, cli, streamer, doH264Parse, func(ctx context.Context, buf []byte, now int64) error {
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
