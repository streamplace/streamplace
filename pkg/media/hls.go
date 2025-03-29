package media

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

func (mm *MediaManager) ToHLS(ctx context.Context, input io.Reader, m3u8 *M3U8) error {
	ctx = log.WithLogValues(ctx, "GStreamerFunc", "ToHLS")

	splitmuxsink, err := gst.NewElementWithProperties("splitmuxsink", map[string]any{
		"name":           "mux",
		"async-finalize": true,
		"sink-factory":   "appsink",
		"muxer-factory":  "mpegtsmux",
		"max-size-bytes": 1,
	})
	if err != nil {
		return err
	}

	p := splitmuxsink.GetRequestPad("video")
	if p == nil {
		return fmt.Errorf("failed to get video pad")
	}
	p = splitmuxsink.GetRequestPad("audio_%u")
	if p == nil {
		return fmt.Errorf("failed to get audio pad")
	}

	pipelineSlice := []string{
		"appsrc name=appsrc ! matroskademux name=demux",
		"demux.video_0 ! queue ! h264parse name=videoparse",
		"demux.audio_0 ! queue ! opusdec use-inband-fec=true ! audioresample ! fdkaacenc name=audioenc",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating ToHLS pipeline: %w", err)
	}

	err = pipeline.Add(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error adding splitmuxsink to ToHLS pipeline: %w", err)
	}

	videoparse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("error getting videoparse from ToHLS pipeline: %w", err)
	}
	err = videoparse.Link(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error linking videoparse to splitmuxsink: %w", err)
	}

	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return fmt.Errorf("error getting audioenc from ToHLS pipeline: %w", err)
	}
	err = audioenc.Link(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error linking audioenc to splitmuxsink: %w", err)
	}

	splitmuxsink.Connect("sink-added", func(split, sinkEle *gst.Element) {
		vf, err := m3u8.GetNextSegment(ctx)
		if err != nil {
			panic(err)
		}
		appsink := app.SinkFromElement(sinkEle)
		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, vf.Buf),
			EOSFunc: func(sink *app.Sink) {
				m3u8.CloseSegment(ctx, vf)
			},
		})
	})

	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, input),
	})

	onPadAdded := func(element *gst.Element, pad *gst.Pad) {
		caps := pad.GetCurrentCaps()
		if caps == nil {
			fmt.Println("Unable to get pad caps")
			return
		}

		fmt.Printf("New pad added: %s\n", pad.GetName())
		fmt.Printf("Caps: %s\n", caps.String())

		structure := caps.GetStructureAt(0)
		if structure == nil {
			fmt.Println("Unable to get structure from caps")
			return
		}

		name := structure.Name()
		fmt.Printf("Structure Name: %s\n", name)

		if name[:5] == "video" {
			// Get some common video properties
			widthVal, _ := structure.GetValue("width")
			heightVal, _ := structure.GetValue("height")

			width, ok := widthVal.(int)
			if ok {
				m3u8.Width = uint64(width)
			}
			height, ok := heightVal.(int)
			if ok {
				m3u8.Height = uint64(height)
			}
			// framerate, ok := framerateVal.(string)
			// if ok {
			// 	fmt.Printf("  Framerate: %s\n", framerate)
			// }
			// pixelAspectRatio, ok := pixelAspectRatioVal.(string)
			// if ok {
			// 	fmt.Printf("  Pixel Aspect Ratio: %s\n", pixelAspectRatio)
			// }
			// if codecVal != nil {
			// 	fmt.Printf("  Has codec data: true\n")
			// }
		}

		// if name[:5] == "audio" {
		// 	// Get some common audio properties
		// 	rateVal, _ := structure.GetValue("rate")
		// 	channelsVal, _ := structure.GetValue("channels")
		// 	formatVal, err := structure.GetValue("format")
		// 	mpegversion, _ := structure.GetValue("mpegversion")
		// 	log.Log(ctx, "format error", "error", err, "mpegversion", mpegversion)

		// 	fmt.Printf("  Structure: %s\n", structure.String())
		// 	rate, ok := rateVal.(int)
		// 	if ok {
		// 		fmt.Printf("  Rate: %d\n", rate)
		// 	}
		// 	channels, ok := channelsVal.(int)
		// 	if ok {
		// 		fmt.Printf("  Channels: %d\n", channels)
		// 	}
		// 	format, ok := formatVal.(int)
		// 	if ok {
		// 		fmt.Printf("  Format: %d\n", format)
		// 	}

		// }
	}

	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return err
	}
	demux.Connect("pad-added", onPadAdded)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		HandleBusMessagesCustom(ctx, pipeline, func(msg *gst.Message) {
			switch msg.Type() {
			case gst.MessageElement:
				structure := msg.GetStructure()
				name := structure.Name()
				if name == "splitmuxsink-fragment-opened" {
					runningTime, err := structure.GetValue("running-time")
					if err != nil {
						log.Warn(ctx, "splitmuxsink-fragment-opened error", "error", err)
						cancel()
					}
					runningTimeInt, ok := runningTime.(uint64)
					if !ok {
						log.Warn(ctx, "splitmuxsink-fragment-opened not a uint64")
						cancel()
					}
					m3u8.FragmentOpened(ctx, runningTimeInt)
				}
				if name == "splitmuxsink-fragment-closed" {
					runningTime, err := structure.GetValue("running-time")
					if err != nil {
						log.Warn(ctx, "splitmuxsink-fragment-closed error", "error", err)
						cancel()
					}
					runningTimeInt, ok := runningTime.(uint64)
					if !ok {
						log.Warn(ctx, "splitmuxsink-fragment-closed not a uint64")
						cancel()
					}
					m3u8.FragmentClosed(ctx, runningTimeInt)
				}
			}
		})
		cancel()
	}()

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	pipeline.BlockSetState(gst.StateNull)

	return nil
}
