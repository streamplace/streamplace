package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// For testing. Normally,  We don't want to stop the pipeline upon a
// segmentation error because we want to keep the stream alive. Lots
// of weird invalid data coming in from WebRTC connections on phones.
// Better we drop one weird segment than force the stream to restart.
// But for tests, we want (sometimes) to know if there's a problem.
var FatalSegmentationErrors = false

// element that takes the input stream, muxes to mp4, and signs the result
func SegmentElem(ctx context.Context, cli *config.CLI, streamer string, doH264Parse bool, cb func(ctx context.Context, buf []byte, now int64) error) (gst.Element, error) {
	// elem, err := gst.NewElement("splitmuxsink name=splitter async-finalize=true sink-factory=appsink muxer-factory=matroskamux max-size-bytes=1")
	elem := gst.ElementFactoryMakeWithProperties("splitmuxsink", map[string]any{
		"name":           "signer",
		"async-finalize": true,
		"sink-factory":   "appsink",
		"muxer-factory":  "mp4mux",
		"max-size-bytes": uint64(1),
	})
	if elem == nil {
		return nil, fmt.Errorf("failed to create splitmuxsink element")
	}

	p := elem.RequestPadSimple("video")
	if p == nil {
		return nil, fmt.Errorf("failed to get video pad")
	}
	p = elem.RequestPadSimple("audio_%u")
	if p == nil {
		return nil, fmt.Errorf("failed to get audio pad")
	}

	resetTimer := make(chan struct{})
	segmentTimestamps := make([]time.Time, 0, 10)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-resetTimer:
				now := time.Now()
				segmentTimestamps = append(segmentTimestamps, now)

				// Only keep events from the last 3 seconds
				cutoff := now.Add(-3 * time.Second)
				filtered := segmentTimestamps[:0]
				for _, t := range segmentTimestamps {
					if t.After(cutoff) {
						filtered = append(filtered, t)
					}
				}
				segmentTimestamps = filtered

				if len(segmentTimestamps) > 6 {
					log.Error(ctx, "too many segments in 3 seconds", "count", len(segmentTimestamps))
					elem.ErrorMessage(gst.DomainCore, gst.CoreErrorFailed, "Too many segments in 3 seconds", "More than 6 segments in 3 seconds")
					return
				}
			case <-time.After(time.Second * 30):
				log.Warn(ctx, "no new segment for 30 seconds")
				elem.MessageError(gst.CoreErrorQuark(), int32(gst.CoreErrorFailed), "No new segment for 30 seconds", "No new segment for 30 seconds (debug)")
				return
			}
		}
	}()

	// we didn't need faststart but i'm leaving this commented here in case
	// you want to change any other muxer properties in the future

	elem.Connect("muxer-added", func(_ gst.Element, muxEle gst.Element) {
		muxEle.SetObjectProperty("presentation-time", false)
		muxEle.SetObjectProperty("interleave-bytes", InterleaveBytes)
		muxEle.SetObjectProperty("interleave-time", InterleaveTime)
		muxEle.SetObjectProperty("faststart", true)
		muxEle.SetObjectProperty("movie-timescale", uint32(60000))
		muxEle.SetObjectProperty("trak-timescale", uint32(60000))
	})

	// channel to make sure data is emitted in order
	var ch chan struct{}

	elem.Connect("sink-added", func(_ gst.Element, sinkEle gst.Element) {
		previousSegCh := ch
		mySegCh := make(chan struct{}, 1)
		ch = mySegCh
		buf := &bytes.Buffer{}
		sinkEle.SetObjectProperty("sync", false)
		appsink, ok := sinkEle.(app.AppSink)
		if !ok {
			panic("appsink should not be nil")
		}

		appsink.SetCallbacks(&app.AppSinkCallbacks{
			NewSample: WriterNewSample(ctx, buf),
			EOS: func(sink app.AppSink) {
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
					log.Debug(ctx, "signing segment", "size", len(bs))
					err := cb(ctx, bs, now)
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
							err := cli.DataFileWrite([]string{"debug-recordings", streamer, fmt.Sprintf("converge-timeout-%d.mp4", now)}, bytes.NewReader(bs), true)
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
						sink.MessageError(gst.CoreErrorQuark(), int32(gst.CoreErrorFailed), "error in segmenter", err.Error())
						return
					}
				}
			},
		})
	})

	return elem, nil
}

func (mm *MediaManager) SegmentAndSignElem(ctx context.Context, ms MediaSigner) (gst.Element, error) {
	tracer := otel.Tracer("signer")
	streamer := ms.Streamer()

	// Stamp a fresh ingest-session epoch on the context that flows down to every
	// segment of this session (onSegment → ValidateMP4 → feedStreamTranscoder). A
	// new live session (RTMP/WHIP (re)connect) restarts the media timeline; the
	// per-DID continuous transcoder keys on this epoch and rebuilds rather than
	// feeding the restarted timeline into the previous session's encoder.
	ctx = withIngestSession(ctx, mm.nextIngestSession())

	// muxl path: stream the fMP4 through the per-segment signer. Each GoP
	// arrives as a bare canonical .m4s, which ValidateMP4 verifies, archives
	// (as .m4s), and distributes. muxl-sign stamps the signing time into the
	// segment, so ValidateMP4 derives StartTime from the segment itself — no
	// per-GoP wall-clock is threaded through here.
	onSegment := func(ctx context.Context, segment []byte) error {
		// Top-level span for segment-delivery latency: signed bytes from the
		// streaming signer go in, a validated/persisted/distributed segment
		// comes out (ValidateMP4 = in-wasm c2pa verify + media parse + DB).
		ctx, span := tracer.Start(ctx, "SegmentAndSign", trace.WithAttributes(
			attribute.String("streamer", streamer),
			attribute.Int("segment_bytes", len(segment)),
		))
		defer span.End()
		startTime := time.Now()
		defer func() {
			spmetrics.SegmentDeliveryDuration.WithLabelValues(streamer).
				Observe(float64(time.Since(startTime).Milliseconds()))
		}()
		if err := mm.ValidateMP4(ctx, bytes.NewReader(segment), true); err != nil {
			span.SetAttributes(attribute.String("error", "validate"))
			mm.cli.DumpDebugSegment(ctx, "just-signed-segment.m4s", bytes.NewReader(segment))
			return fmt.Errorf("error validating signed segment: %w", err)
		}
		return nil
	}
	return MuxlSignSegmentElem(ctx, mm.cli, ms, onSegment)
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
	launched, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating SegmentUnsigned pipeline: %w", err)
	}
	pipeline, ok := launched.(gst.Pipeline)
	if !ok {
		return fmt.Errorf("error creating SegmentUnsigned pipeline: parsed description is not a pipeline")
	}

	srcele := pipeline.GetByName("appsrc")
	if srcele == nil {
		return fmt.Errorf("appsrc element not found")
	}
	srcele.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, input),
	})
	videoParseEle := pipeline.GetByName("videoparse")
	if videoParseEle == nil {
		return fmt.Errorf("videoparse element not found")
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

	if !pipeline.Add(segmenter) {
		return fmt.Errorf("failed to add segmenter to pipeline")
	}
	if !videoParseEle.Link(segmenter) {
		return fmt.Errorf("failed to link videoparse to segmenter")
	}
	audioparse := pipeline.GetByName("audioparse")
	if audioparse == nil {
		return fmt.Errorf("audioparse element not found")
	}
	if !audioparse.Link(segmenter) {
		return fmt.Errorf("failed to link audioparse to segmenter")
	}

	busErr := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		cancel()
		busErr <- err
	}()

	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return fmt.Errorf("error setting SegmentUnsigned pipeline to playing")
	}

	defer func() {
		if pipeline.SetState(gst.StateNull) == gst.StateChangeFailure {
			log.Error(ctx, "error setting pipeline to null state")
		}
	}()

	err = <-busErr
	if err != nil {
		return err
	}

	return nil
}
