package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
)

func padProbeEmpty(_ *gst.Pad, _ *gst.PadProbeInfo) gst.PadProbeReturn {
	return gst.PadProbeOK
}

func ParseSegmentMediaData(ctx context.Context, mp4bs []byte) (*localdb.SegmentMediaData, error) {
	ctx, span := otel.Tracer("signer").Start(ctx, "ParseSegmentMediaData")
	defer span.End()
	ctx = log.WithLogValues(ctx, "GStreamerFunc", "ParseSegmentMediaData")
	// Watchdog: parsing a ~1s segment is sub-second. A stalled qtdemux only
	// posts non-fatal warnings, so bound it — a hang surfaces as an error
	// rather than wedging the validate/ingest pipeline (which is what a stray
	// unresolved delayed link did).
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Codec-agnostic: audio rate/channels come from the qtdemux pad caps (read
	// in onPadAdded) and durations from the demuxed buffers, so no per-codec
	// parser is needed — the segment may carry AAC or Opus. We link only
	// video_0 and audio_0. A dual-codec (completed) segment's extra audio track
	// (audio_1) is left unlinked: qtdemux's flow combiner tolerates the
	// not-linked pad, and the linked sinks still reach EOS. We must NOT add an
	// idle sink for it (or delayed-link a pad that may not exist) — an unlinked
	// sink never receives EOS, so the pipeline never posts EOS to the bus and
	// the parse stalls until its watchdog fires.
	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux name=demux",
		fmt.Sprintf("demux.video_0 ! %s ! tee name=videotee", constants.Queue2Big),
		fmt.Sprintf("videotee. ! %s ! h2642json ! appsink sync=false name=jsonappsink", constants.Queue2Big),
		fmt.Sprintf("videotee. ! %s ! appsink sync=false name=videoappsink", constants.Queue2Big),
		fmt.Sprintf("demux.audio_0 ! %s ! appsink sync=false name=audioappsink", constants.Queue2Big),
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}

	var videoMetadata *localdb.SegmentMediadataVideo
	var audioMetadata *localdb.SegmentMediadataAudio
	var videoDuration time.Duration
	var audioDuration time.Duration

	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, bytes.NewReader(mp4bs)),
	})

	foundSomeAudio := false
	audioSinkElem, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}
	audioSink := app.SinkFromElement(audioSinkElem)
	if audioSink == nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}
	audioSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: ParseSegmentMediaDataSinkNewSampleFunc(ctx, &foundSomeAudio, &audioDuration),
	})

	foundSomeVideo := false
	videoSinkElem, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}
	videoSink := app.SinkFromElement(videoSinkElem)
	if videoSink == nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}
	videoSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: ParseSegmentMediaDataSinkNewSampleFunc(ctx, &foundSomeVideo, &videoDuration),
	})
	padsAdded := 0

	var padProbe func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn
	padProbe = func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		if info.GetEvent().Type() != gst.EventTypeEOS {
			return gst.PadProbeOK
		}
		if padsAdded < 2 {
			err := fmt.Errorf("expected at least 2 tracks (video + audio), got %d", padsAdded)
			pipeline.Error(err.Error(), err)
		}
		padProbe = padProbeEmpty
		return gst.PadProbeRemove
	}

	outerPadProbe := func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		return padProbe(pad, info)
	}

	onPadAdded := func(element *gst.Element, pad *gst.Pad) {
		padsAdded += 1
		caps := pad.GetCurrentCaps()
		if caps == nil {
			log.Warn(ctx, "Unable to get pad caps")
			cancel()
			return
		}

		pad.AddProbe(gst.PadProbeTypeEventBoth, outerPadProbe)

		structure := caps.GetStructureAt(0)
		if structure == nil {
			log.Warn(ctx, "Unable to get structure from caps")
			cancel()
			return
		}

		name := structure.Name()

		if name[:5] == "video" {
			videoMetadata = &localdb.SegmentMediadataVideo{}
			// Get some common video properties
			widthVal, _ := structure.GetValue("width")
			heightVal, _ := structure.GetValue("height")

			width, ok := widthVal.(int)
			if ok {
				videoMetadata.Width = width
			}
			height, ok := heightVal.(int)
			if ok {
				videoMetadata.Height = height
			}
			framerateVal, _ := structure.GetValue("framerate")
			framerateStr := fmt.Sprintf("%v", framerateVal)
			parts := strings.Split(framerateStr, "/")
			num := 0
			den := 0
			if len(parts) == 2 {
				num, _ = strconv.Atoi(parts[0])
				den, _ = strconv.Atoi(parts[1])
			}
			if num != 0 && den != 0 {
				videoMetadata.FPSNum = num
				videoMetadata.FPSDen = den
			}
		}

		// Primary audio track (audio_0) is statically linked to audioappsink;
		// read its rate/channels for the segment metadata. Extra audio tracks
		// (audio_1+, on a dual-codec completed segment) are left unlinked.
		if name[:5] == "audio" && pad.GetName() == "audio_0" {
			audioMetadata = &localdb.SegmentMediadataAudio{}
			rateVal, _ := structure.GetValue("rate")
			channelsVal, _ := structure.GetValue("channels")

			rate, ok := rateVal.(int)
			if ok {
				audioMetadata.Rate = rate
			}
			channels, ok := channelsVal.(int)
			if ok {
				audioMetadata.Channels = channels
			}
		}

		// if videoMetadata != nil && audioMetadata != nil {
		// 	cancel()
		// }
	}

	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}
	_, err = demux.Connect("pad-added", onPadAdded)
	if err != nil {
		return nil, fmt.Errorf("error connecting pad-add: %w", err)
	}

	jsonSinkElem, err := pipeline.GetElementByName("jsonappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get videoappsink element: %w", err)
	}
	jsonSink := app.SinkFromElement(jsonSinkElem)
	if jsonSink == nil {
		return nil, fmt.Errorf("failed to get videoappsink element: %w", err)
	}

	hasBFrames := false

	r, w := io.Pipe()
	bufW := bufio.NewWriter(w)
	decoder := json.NewDecoder(r)

	decodeErr := make(chan error)
	go func() {
		for {
			var obj map[string]any
			err := decoder.Decode(&obj)
			if err == io.EOF {
				decodeErr <- nil
				break // End of stream
			}
			if err != nil {
				decodeErr <- err
				break
			}
			// https://github.com/GStreamer/gstreamer/blob/68fa54c7616b93d5b7cc5febaa388546fcd617e0/subprojects/gst-plugins-bad/ext/codec2json/gsth2642json.c#L836
			header, ok := obj["slice header"].(map[string]any)
			if !ok {
				continue
			}
			// https://github.com/GStreamer/gstreamer/blob/68fa54c7616b93d5b7cc5febaa388546fcd617e0/subprojects/gst-plugins-bad/ext/codec2json/gsth2642json.c#L622
			flag, ok := header["direct spatial mv pred flag"].(bool)
			if ok && flag {
				hasBFrames = true
			}
		}
		close(decodeErr)
	}()

	jsonSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			buf := sample.GetBuffer().Bytes()
			_, err := bufW.Write(buf)
			if err != nil {
				log.Error(ctx, "failed to write to buffer", "error", err)
				return gst.FlowError
			}

			return gst.FlowOK
		},
	})

	go func() {
		if err := HandleBusMessages(ctx, pipeline); err != nil {
			log.Log(ctx, "pipeline error", "error", err)
		}
		cancel()
	}()

	// Start the pipeline
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return nil, err
	}

	defer func() {
		if err := pipeline.BlockSetState(gst.StateNull); err != nil {
			log.Error(ctx, "error setting pipeline state to null", "error", err)
		}
	}()

	<-ctx.Done()

	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("error closing writer: %w", err)
	}

	err = <-decodeErr
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON object: %w", err)
	}

	if videoMetadata == nil || !foundSomeVideo {
		return nil, fmt.Errorf("no video in segment")
	}
	if audioMetadata == nil || !foundSomeAudio {
		return nil, fmt.Errorf("no audio in segment")
	}

	videoMetadata.BFrames = hasBFrames

	meta := &localdb.SegmentMediaData{
		Video: []*localdb.SegmentMediadataVideo{videoMetadata},
		Audio: []*localdb.SegmentMediadataAudio{audioMetadata},
	}

	meta.Duration = videoDuration.Nanoseconds()

	return meta, nil
}

func ParseSegmentMediaDataSinkNewSampleFunc(ctx context.Context, foundThisTrack *bool, duration *time.Duration) func(sink *app.Sink) gst.FlowReturn {
	var firstPTS *time.Time
	return func(sink *app.Sink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowOK
		}
		buf := sample.GetBuffer()
		if buf == nil {
			return gst.FlowError
		}
		pts := buf.PresentationTimestamp().AsTimestamp()
		if firstPTS == nil {
			firstPTS = pts
		}
		diff := pts.Sub(*firstPTS)
		*duration = diff
		dur := buf.Duration().AsDuration()
		if dur != nil && *dur > 0 {
			*foundThisTrack = true
		} else {
			log.Warn(ctx, "no duration found for track", "track", sink.GetName())
		}
		return gst.FlowOK
	}
}
