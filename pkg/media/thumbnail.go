package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// ThumbnailFromSegment generates a thumbnail from a single VOD segment.
// initSeg is the combined init (ftyp+moov for every track); segment is
// the signed m4s chunk(s) for one muxl segment. The two are reassembled
// into a fragmented MP4, flattened to a faststart MP4 via muxl, then
// decoded for a frame.
//
// It deliberately does NOT go through Thumbnail: that path's
// ConcatDemuxBin front-end hardcodes opusparse for livestream audio and
// deadlocks on VOD's AAC. thumbnailFromFlatMP4 uses decodebin instead,
// which is codec-agnostic.
func ThumbnailFromSegment(ctx context.Context, initSeg, segment []byte, w io.Writer, format string) error {
	fmp4 := make([]byte, 0, len(initSeg)+len(segment))
	fmp4 = append(fmp4, initSeg...)
	fmp4 = append(fmp4, segment...)

	flattenStart := time.Now()
	flat, err := flattenForThumbnail(ctx, fmp4)
	if err != nil {
		return fmt.Errorf("flatten segment for thumbnail: %w", err)
	}
	log.Log(ctx, "thumbnail: flattened segment",
		"ms", time.Since(flattenStart).Milliseconds(),
		"in_bytes", len(fmp4), "flat_bytes", len(flat))

	decodeStart := time.Now()
	err = thumbnailFromFlatMP4(ctx, flat, w, format)
	log.Log(ctx, "thumbnail: decoded frame",
		"ms", time.Since(decodeStart).Milliseconds(), "ok", err == nil)
	return err
}

// thumbnailFromFlatMP4 renders a single frame from a flat, video-only
// MP4 using decodebin for demux+decode. Unlike Thumbnail's
// ConcatDemuxBin path (hardcoded to opus audio for livestreaming),
// decodebin is codec-agnostic, so it handles VOD content's h264/AAC.
//
// The input must be video-only: decodebin's dynamic pad is wired by the
// gst-launch parser (`decodebin ! videoconvert`), so a lone video pad
// links cleanly with no manual pad-added handler — manual pad-added
// closures are a go-gst GC hazard (see ConcatDemuxBin's workarounds).
func thumbnailFromFlatMP4(ctx context.Context, flat []byte, w io.Writer, format string) error {
	ctx = log.WithLogValues(ctx, "function", "thumbnailFromFlatMP4")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var encoder string
	switch format {
	case "jpeg":
		encoder = "jpegenc"
	case "png":
		encoder = "pngenc snapshot=true"
	default:
		log.Error(ctx, "thumbnailFromFlatMP4: expected jpeg or png", "format", format)
		encoder = "jpegenc"
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join([]string{
		"appsrc name=src ! decodebin ! videoconvert ! videoscale ! videorate ! capsfilter caps=video/x-raw,width=[1,1280],height=[1,720],pixel-aspect-ratio=1/1,framerate=1/999999 ! ",
		encoder,
		" ! appsink name=appsink",
	}, "\n"))
	if err != nil {
		return fmt.Errorf("create thumbnail pipeline: %w", err)
	}
	defer func() {
		if e := pipeline.SetState(gst.StateNull); e != nil {
			log.Error(ctx, "thumbnail: failed to set pipeline to null", "error", e)
		}
	}()

	srcEle, err := pipeline.GetElementByName("src")
	if err != nil {
		return fmt.Errorf("thumbnail: get appsrc: %w", err)
	}
	app.SrcFromElement(srcEle).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, bytes.NewReader(flat)),
	})

	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return fmt.Errorf("thumbnail: get appsink: %w", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- HandleBusMessages(ctx, pipeline) }()

	thumbCh := make(chan struct{})
	var once sync.Once
	app.SinkFromElement(appsink).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			buffer := sample.GetBuffer()
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			if _, err := w.Write(bs); err != nil {
				log.Error(ctx, "thumbnail: error writing output", "error", err)
				return gst.FlowError
			}
			once.Do(func() { close(thumbCh) })
			return gst.FlowOK
		},
	})

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("thumbnail: set playing: %w", err)
	}

	select {
	case <-thumbCh:
		// Got the frame; wind the pipeline down so HandleBusMessages returns.
		pipeline.Error(ErrConcatDone.Error(), ErrConcatDone)
		<-errCh
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("thumbnail pipeline: %w", err)
		}
		return fmt.Errorf("thumbnail pipeline ended without producing a frame")
	}
}

// flattenForThumbnail turns a video-only fragmented MP4 — a per-track
// init segment followed by one or more moof+mdat fragments — into a flat
// faststart MP4 (moov up front, full sample tables) via muxl. The
// Thumbnail demux path stalls on fragmented input fed through a
// push-mode appsrc (qtdemux waits for fragments that never arrive), so
// it needs a real flat MP4. muxl skips the unknown c2pa-uuid/muxl-uuid
// boxes that prefix signed segments.
func flattenForThumbnail(ctx context.Context, fmp4 []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := muxl.RunMuxlMp4(ctx, bytes.NewReader(fmp4), &out); err != nil {
		return nil, fmt.Errorf("muxl flatten: %w", err)
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("muxl flatten produced no output")
	}
	return out.Bytes(), nil
}

func Thumbnail(ctx context.Context, r io.Reader, w io.Writer, format string) error {
	ctx = log.WithLogValues(ctx, "function", "Thumbnail")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var encoder string
	switch format {
	case "jpeg":
		encoder = "jpegenc"
	case "png":
		encoder = "pngenc snapshot=true"
	default:
		log.Error(ctx, "media.Thumbnail: expected jpeg or png as format and received %s", format)
		encoder = "pngenc snapshot=true"
	}

	// Read all data from the reader to create a Seg for ConcatDemuxBin
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("error reading input data: %w", err)
	}

	seg := &bus.Seg{
		Data: data,
	}

	pipelineSlice := []string{
		"decodebin name=decode ! videoconvert ! videoscale ! videorate ! capsfilter name=capsfilter caps=video/x-raw,width=[1,1280],height=[1,720],pixel-aspect-ratio=1/1,framerate=1/999999 ! ",
		encoder,
		" ! appsink name=appsink",
		"fakesink name=audiofakesink sync=false",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating Thumbnail pipeline: %w", err)
	}

	demuxBin, err := ConcatDemuxBin(ctx, seg, false)
	if err != nil {
		return fmt.Errorf("failed to create demux bin: %w", err)
	}

	err = pipeline.Add(demuxBin.Element)
	if err != nil {
		return fmt.Errorf("failed to add demux bin to pipeline: %w", err)
	}

	demuxBinPadVideoSrc := demuxBin.GetStaticPad("video_0")
	if demuxBinPadVideoSrc == nil {
		return fmt.Errorf("failed to get demux bin video src pad")
	}

	demuxBinPadAudioSrc := demuxBin.GetStaticPad("audio_0")
	if demuxBinPadAudioSrc == nil {
		return fmt.Errorf("failed to get demux bin audio src pad")
	}

	decode, err := pipeline.GetElementByName("decode")
	if err != nil {
		return fmt.Errorf("failed to get decodebin element: %w", err)
	}

	audioFakeSink, err := pipeline.GetElementByName("audiofakesink")
	if err != nil {
		return fmt.Errorf("failed to get audio fakesink element: %w", err)
	}

	linked := demuxBinPadVideoSrc.Link(decode.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link demux bin video src to decodebin: %v", linked)
	}

	linked = demuxBinPadAudioSrc.Link(audioFakeSink.GetStaticPad("sink"))
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link demux bin audio src to fakesink: %v", linked)
	}

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline state to null", "error", err)
		}
		err = pipeline.Remove(demuxBin.Element)
		if err != nil {
			log.Error(ctx, "failed to remove demux bin from pipeline", "error", err)
		}
	}()

	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		errCh <- err
		close(errCh)
	}()

	thumbCh := make(chan struct{})

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			_, err := w.Write(bs)
			log.Debug(ctx, "wrote buffer", "length", len(bs))

			if err != nil {
				log.Error(ctx, "error writing to output", "error", err)
				return gst.FlowError
			}

			close(thumbCh)

			return gst.FlowOK
		},
	})

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("error setting pipeline state: %w", err)
	}

	<-thumbCh
	// signals the pipeline to clean up cleanly
	pipeline.Error(ErrConcatDone.Error(), ErrConcatDone)

	busErr := <-errCh

	log.Debug(ctx, "thumbnail done")

	return busErr
}
