package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

func padProbeEmpty(_ gst.Pad, _ *gst.PadProbeInfo) gst.PadProbeReturn {
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

	launched, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}
	pipeline, ok := launched.(gst.Pipeline)
	if !ok {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: parsed description is not a pipeline")
	}

	var videoMetadata *localdb.SegmentMediadataVideo
	var audioMetadata *localdb.SegmentMediadataAudio
	var videoDuration time.Duration
	var audioDuration time.Duration

	appsrc := pipeline.GetByName("appsrc")
	if appsrc == nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: appsrc not found")
	}
	appsrc.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: ReaderNeedDataIncremental(ctx, bytes.NewReader(mp4bs)),
	})

	foundSomeAudio := false
	audioSink := pipeline.GetByName("audioappsink")
	if audioSink == nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: audioappsink not found")
	}
	audioSink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: ParseSegmentMediaDataSinkNewSampleFunc(ctx, &foundSomeAudio, &audioDuration),
	})

	foundSomeVideo := false
	videoSink := pipeline.GetByName("videoappsink")
	if videoSink == nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: videoappsink not found")
	}
	videoSink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: ParseSegmentMediaDataSinkNewSampleFunc(ctx, &foundSomeVideo, &videoDuration),
	})
	padsAdded := 0

	var padProbe func(pad gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn
	padProbe = func(pad gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		if info.GetEvent().GetType() != gst.EventEOS {
			return gst.PadProbeOK
		}
		if padsAdded < 2 {
			err := fmt.Errorf("expected at least 2 tracks (video + audio), got %d", padsAdded)
			pipeline.PostMessage(gst.NewMessageError(pipeline, err.Error(), err))
		}
		padProbe = padProbeEmpty
		return gst.PadProbeRemove
	}

	outerPadProbe := func(pad gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		return padProbe(pad, info)
	}

	onPadAdded := func(_ gst.Element, pad gst.Pad) {
		padsAdded += 1
		caps := pad.GetCurrentCaps()
		if caps == nil {
			log.Warn(ctx, "Unable to get pad caps")
			cancel()
			return
		}

		pad.AddProbe(gst.PadProbeTypeEventBoth, outerPadProbe)

		structure := caps.GetStructure(0)
		if structure == nil {
			log.Warn(ctx, "Unable to get structure from caps")
			cancel()
			return
		}

		name := structure.GetName()

		if name[:5] == "video" {
			videoMetadata = &localdb.SegmentMediadataVideo{}
			// Get some common video properties
			if width, ok := structure.GetInt("width"); ok {
				videoMetadata.Width = int(width)
			}
			if height, ok := structure.GetInt("height"); ok {
				videoMetadata.Height = int(height)
			}
			if num, den, ok := structure.GetFraction("framerate"); ok && num != 0 && den != 0 {
				videoMetadata.FPSNum = int(num)
				videoMetadata.FPSDen = int(den)
			}
		}

		// Primary audio track (audio_0) is statically linked to audioappsink;
		// read its rate/channels for the segment metadata. Extra audio tracks
		// (audio_1+, on a dual-codec completed segment) are left unlinked.
		if name[:5] == "audio" && pad.GetName() == "audio_0" {
			audioMetadata = &localdb.SegmentMediadataAudio{}
			if rate, ok := structure.GetInt("rate"); ok {
				audioMetadata.Rate = int(rate)
			}
			if channels, ok := structure.GetInt("channels"); ok {
				audioMetadata.Channels = int(channels)
			}
		}

		// if videoMetadata != nil && audioMetadata != nil {
		// 	cancel()
		// }
	}

	demux := pipeline.GetByName("demux")
	if demux == nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: demux not found")
	}
	demux.ConnectPadAdded(onPadAdded)

	jsonSinkElem := pipeline.GetByName("jsonappsink")
	if jsonSinkElem == nil {
		return nil, fmt.Errorf("failed to get jsonappsink element")
	}
	jsonSink := jsonSinkElem.(app.AppSink)

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

	jsonSink.SetCallbacks(&app.AppSinkCallbacks{
		NewSample: func(sink app.AppSink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			info, ok := sample.GetBuffer().Map(gst.MapRead)
			if !ok {
				log.Error(ctx, "failed to map buffer")
				return gst.FlowError
			}
			defer info.Unmap()
			// bufio copies the mapped bytes into its own buffer (or writes them
			// through to the pipe synchronously), so nothing escapes the mapping.
			_, err := bufW.Write(info.Data())
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
	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return nil, fmt.Errorf("error setting SegmentMetadata pipeline to playing")
	}

	defer func() {
		if pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone) == gst.StateChangeFailure {
			log.Error(ctx, "error setting pipeline state to null")
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

// audioMediaData describes a catalog's audio tracks for the segment record:
// every track, in track-id order, with the lexicon's codec name ("aac",
// "opus") and its alternateGroup and label when the stream carries them. The
// gst parse in ParseSegmentMediaData reads only the first audio track, so
// this is what a segment with stems lists them from.
func audioMediaData(cat *muxl.MuxlCatalog) []*localdb.SegmentMediadataAudio {
	var out []*localdb.SegmentMediadataAudio
	for _, a := range audioTracks(cat) {
		codec := a.Codec
		switch {
		case isAACCodec(codec):
			codec = "aac"
		case isOpusCodec(codec):
			codec = "opus"
		}
		out = append(out, &localdb.SegmentMediadataAudio{
			Codec:          codec,
			Rate:           int(a.SampleRate),
			Channels:       int(a.NumberOfChannels),
			AlternateGroup: a.AlternateGroup,
			Label:          a.Label,
		})
	}
	return out
}

func ParseSegmentMediaDataSinkNewSampleFunc(ctx context.Context, foundThisTrack *bool, duration *time.Duration) func(sink app.AppSink) gst.FlowReturn {
	var firstPTS gst.ClockTime
	haveFirstPTS := false
	return func(sink app.AppSink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowOK
		}
		buf := sample.GetBuffer()
		if buf == nil {
			return gst.FlowError
		}
		if pts := buf.PTS(); pts != gst.ClockTimeNone {
			if !haveFirstPTS {
				firstPTS = pts
				haveFirstPTS = true
			}
			*duration = time.Duration(int64(pts) - int64(firstPTS))
		}
		if dur, ok := clockTimeDuration(buf.Duration()); ok && dur > 0 {
			*foundThisTrack = true
		} else {
			log.Warn(ctx, "no duration found for track", "track", sink.GetName())
		}
		return gst.FlowOK
	}
}
