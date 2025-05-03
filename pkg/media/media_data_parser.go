package media

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

func ParseSegmentMediaData(ctx context.Context, mp4bs []byte) (*model.SegmentMediaData, error) {
	ctx, span := otel.Tracer("signer").Start(ctx, "ParseSegmentMediaData")
	defer span.End()
	ctx = log.WithLogValues(ctx, "GStreamerFunc", "ParseSegmentMediaData")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux name=demux ! fakesink sync=false",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}

	var videoMetadata *model.SegmentMediadataVideo
	var audioMetadata *model.SegmentMediadataAudio

	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, bytes.NewReader(mp4bs)),
	})

	onPadAdded := func(element *gst.Element, pad *gst.Pad) {
		caps := pad.GetCurrentCaps()
		if caps == nil {
			log.Warn(ctx, "Unable to get pad caps")
			cancel()
			return
		}

		structure := caps.GetStructureAt(0)
		if structure == nil {
			log.Warn(ctx, "Unable to get structure from caps")
			cancel()
			return
		}

		name := structure.Name()

		if name[:5] == "video" {
			videoMetadata = &model.SegmentMediadataVideo{}
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

		if name[:5] == "audio" {
			audioMetadata = &model.SegmentMediadataAudio{}
			// Get some common audio properties
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
	demux.Connect("pad-added", onPadAdded)

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	meta := &model.SegmentMediaData{
		Video: []*model.SegmentMediadataVideo{videoMetadata},
		Audio: []*model.SegmentMediadataAudio{audioMetadata},
	}

	ok, dur := pipeline.QueryDuration(gst.FormatTime)
	if !ok {
		return nil, fmt.Errorf("error getting duration")
	} else {
		meta.Duration = dur
	}

	pipeline.BlockSetState(gst.StateNull)

	return meta, nil
}
