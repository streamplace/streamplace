package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

// ThumbnailFromSegment generates a thumbnail from a single VOD segment.
// initSeg is the per-track fmp4 init (ftyp+moov); segment is the signed
// m4s chunk(s) for one muxl segment. The two are concatenated into a
// fragmented MP4 and decoded directly for a frame.
//
// We feed the fragmented init+segment straight to decodebin rather than
// muxl-flattening it to a flat MP4 first. A flat moov carries a
// movie-level duration — for a single mid-stream segment that's the whole
// VOD's duration, wildly inconsistent with the one segment present, which
// trips qtdemux's track-vs-movie duration heuristics; the fmp4 init has no
// movie duration at all. The c2pa-uuid box that prefixes a signed segment
// is a top-level box qtdemux skips.
//
// It deliberately does NOT go through Thumbnail: that path's
// ConcatDemuxBin front-end hardcodes opusparse for livestream audio and
// deadlocks on VOD's AAC. decodebin is codec-agnostic.
func ThumbnailFromSegment(ctx context.Context, initSeg, segment []byte, w io.Writer, format string) error {
	fmp4 := make([]byte, 0, len(initSeg)+len(segment))
	fmp4 = append(fmp4, initSeg...)
	fmp4 = append(fmp4, segment...)

	decodeStart := time.Now()
	err := thumbnailFromMP4(ctx, fmp4, w, format)
	log.Log(ctx, "thumbnail: decoded frame",
		"ms", time.Since(decodeStart).Milliseconds(), "in_bytes", len(fmp4), "ok", err == nil)
	return err
}

// thumbnailFromMP4 renders a single frame from a video-only MP4 (flat or
// fragmented) using decodebin for demux+decode. Unlike Thumbnail's
// ConcatDemuxBin path (hardcoded to opus audio for livestreaming),
// decodebin is codec-agnostic, so it handles VOD content's h264/AAC.
//
// The input must be video-only: decodebin's dynamic pad is wired by the
// gst-launch parser (`decodebin ! videoconvert`), so a lone video pad
// links cleanly with no manual pad-added handler — manual pad-added
// closures are a go-gst GC hazard (see ConcatDemuxBin's workarounds).
func thumbnailFromMP4(ctx context.Context, flat []byte, w io.Writer, format string) error {
	ctx = log.WithLogValues(ctx, "function", "thumbnailFromMP4")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var encoder string
	switch format {
	case "jpeg":
		encoder = "jpegenc"
	case "png":
		encoder = "pngenc snapshot=true"
	default:
		log.Error(ctx, "thumbnailFromMP4: expected jpeg or png", "format", format)
		encoder = "jpegenc"
	}

	launched, err := gst.ParseLaunch(strings.Join([]string{
		"appsrc name=src ! decodebin ! videoconvert ! videoscale ! videorate ! capsfilter caps=video/x-raw,width=[1,1280],height=[1,720],pixel-aspect-ratio=1/1,framerate=1/999999 ! ",
		encoder,
		" ! appsink sync=false name=appsink",
	}, "\n"))
	if err != nil {
		return fmt.Errorf("create thumbnail pipeline: %w", err)
	}
	pipeline, ok := launched.(gst.Pipeline)
	if !ok {
		return fmt.Errorf("create thumbnail pipeline: parsed description is not a pipeline")
	}
	defer func() {
		if pipeline.SetState(gst.StateNull) == gst.StateChangeFailure {
			log.Error(ctx, "thumbnail: failed to set pipeline to null")
		}
	}()

	srcEle := pipeline.GetByName("src")
	if srcEle == nil {
		return fmt.Errorf("thumbnail: get appsrc: element not found")
	}
	srcEle.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, bytes.NewReader(flat)),
	})

	appsink := pipeline.GetByName("appsink")
	if appsink == nil {
		return fmt.Errorf("thumbnail: get appsink: element not found")
	}

	errCh := make(chan error, 1)
	go func() { errCh <- HandleBusMessages(ctx, pipeline) }()

	thumbCh := make(chan struct{})
	var once sync.Once
	appsink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: func(sink app.AppSink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			info, ok := sample.GetBuffer().Map(gst.MapRead)
			if !ok {
				log.Error(ctx, "thumbnail: failed to map buffer")
				return gst.FlowError
			}
			defer info.Unmap()
			if _, err := w.Write(info.Data()); err != nil {
				log.Error(ctx, "thumbnail: error writing output", "error", err)
				return gst.FlowError
			}
			once.Do(func() { close(thumbCh) })
			return gst.FlowOK
		},
	})

	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return fmt.Errorf("thumbnail: set playing: state change failed")
	}

	select {
	case <-thumbCh:
		// Got the frame; wind the pipeline down so HandleBusMessages returns.
		pipeline.PostMessage(gst.NewMessageError(pipeline, ErrPipelineDone.Error(), ErrPipelineDone))
		<-errCh //nolint:errcheck // the pipeline was told to stop
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("thumbnail pipeline: %w", err)
		}
		return fmt.Errorf("thumbnail pipeline ended without producing a frame")
	}
}

// SegmentThumbnail renders a thumbnail of a live source segment (a bare
// canonical .m4s) from its first H.264 track, the cheap one to decode, or,
// with none (an AV1-only stream), from its first video track.
func SegmentThumbnail(ctx context.Context, seg []byte, w io.Writer, format string) error {
	pres, err := presentationWithOpus(ctx, seg, onlyH264)
	if errors.Is(err, errNoVideoTrack) {
		pres, err = presentationWithOpus(ctx, seg, nil)
	}
	if err != nil {
		return fmt.Errorf("select thumbnail video: %w", err)
	}
	return Thumbnail(ctx, bytes.NewReader(pres), w, format)
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
		" ! appsink sync=false name=appsink",
		"fakesink name=audiofakesink sync=false",
	}

	launched, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating Thumbnail pipeline: %w", err)
	}
	pipeline, ok := launched.(gst.Pipeline)
	if !ok {
		return fmt.Errorf("error creating Thumbnail pipeline: parsed description is not a pipeline")
	}

	demuxBin, err := ConcatDemuxBin(ctx, seg, VideoCodec{}, 1)
	if err != nil {
		return fmt.Errorf("failed to create demux bin: %w", err)
	}

	if !pipeline.Add(demuxBin) {
		return fmt.Errorf("failed to add demux bin to pipeline")
	}

	demuxBinPadVideoSrc := demuxBin.GetStaticPad("video_0")
	if demuxBinPadVideoSrc == nil {
		return fmt.Errorf("failed to get demux bin video src pad")
	}

	demuxBinPadAudioSrc := demuxBin.GetStaticPad("audio_0")
	if demuxBinPadAudioSrc == nil {
		return fmt.Errorf("failed to get demux bin audio src pad")
	}

	decode := pipeline.GetByName("decode")
	if decode == nil {
		return fmt.Errorf("failed to get decodebin element")
	}

	audioFakeSink := pipeline.GetByName("audiofakesink")
	if audioFakeSink == nil {
		return fmt.Errorf("failed to get audio fakesink element")
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
		if pipeline.SetState(gst.StateNull) == gst.StateChangeFailure {
			log.Error(ctx, "failed to set pipeline state to null")
		}
		if !pipeline.Remove(demuxBin) {
			log.Error(ctx, "failed to remove demux bin from pipeline")
		}
	}()

	appsink := pipeline.GetByName("appsink")
	if appsink == nil {
		return fmt.Errorf("failed to get appsink element")
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		errCh <- err
		close(errCh)
	}()

	thumbCh := make(chan struct{})
	// The first frame is the thumbnail. Another sample can still arrive
	// before the pipeline is torn down (on a loaded machine); it's dropped,
	// neither appended to w nor closing thumbCh twice, which panicked the
	// node. Samples arrive on the appsink's one streaming thread, so a plain
	// bool will do.
	done := false

	appsink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: func(sink app.AppSink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil || done {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			info, ok := sample.GetBuffer().Map(gst.MapRead)
			if !ok {
				log.Error(ctx, "failed to map buffer")
				return gst.FlowError
			}
			defer info.Unmap()

			bs := info.Data()
			_, err := w.Write(bs)
			log.Debug(ctx, "wrote buffer", "length", len(bs))

			if err != nil {
				log.Error(ctx, "error writing to output", "error", err)
				return gst.FlowError
			}

			done = true
			close(thumbCh)

			return gst.FlowOK
		},
	})

	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return fmt.Errorf("error setting pipeline state to playing")
	}

	<-thumbCh
	// signals the pipeline to clean up cleanly
	pipeline.PostMessage(gst.NewMessageError(pipeline, ErrPipelineDone.Error(), ErrPipelineDone))

	busErr := <-errCh

	log.Debug(ctx, "thumbnail done")

	return busErr
}
