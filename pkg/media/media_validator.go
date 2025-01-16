package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aquareum.tv/aquareum/pkg/log"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/pion/webrtc/v4/pkg/media"
)

type MediaValidator struct {
	idx int
}

// var files []string = []string{
// 	// "/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-00-411Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-01-212Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-01-830Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-02-492Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-03-163Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-03-430Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-04-209Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-04-604Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-05-308Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-05-970Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-06-406Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-07-271Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-07-868Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-08-572Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-09-286Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-09-404Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-10-289Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-11-431Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-12-390Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-13-585Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-14-588Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-15-409Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-17-372Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-18-407Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-19-025Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-19-591Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-20-369Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-20-967Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-21-393Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-21-970Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-22-812Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-24-391Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-24-988Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-25-606Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-26-310Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-27-333Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-27-452Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-28-305Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-29-052Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-29-607Z.mp4",
// 	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/22/18/2025-01-15T22-18-30-407Z.mp4",
// }

var files []string = []string{
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-00-459Z.mp4", // evil
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-03-424Z.mp4", // good
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-04-661Z.mp4", // good
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-07-121Z.mp4", // good
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-11-285Z.mp4", // good
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-12-938Z.mp4", // evil
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-17-343Z.mp4", // evil
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-19-158Z.mp4", // good
	"/home/iameli/.aquareum/segments/0x3371a9b874d9815c8d18e7d4662cda099a4737b2/2025/01/15/23/29/2025-01-15T23-29-22-261Z.mp4", // good
	// "/home/iameli/Desktop/out/2025-01-15T23-29-00-459Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-03-424Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-04-661Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-07-121Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-11-285Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-12-938Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-17-343Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-19-158Z.mp4.mkv.mp4",
	// "/home/iameli/Desktop/out/2025-01-15T23-29-22-261Z.mp4.mkv.mp4",
}

func (mv *MediaValidator) SubscribeSegment(ctx context.Context, user string) <-chan string {
	ch := make(chan string, 1024)
	go func() {
		if mv.idx >= len(files) {
			ch <- ""
			return
		}
		ch <- files[mv.idx]
		mv.idx += 1
	}()
	return ch
}

func ValidateMedia(ctx context.Context) error {
	mv := &MediaValidator{}

	ctx, cancel := context.WithCancel(ctx)

	ctx = log.WithLogValues(ctx, "mediafunc", "ValidateMedia")

	log.Debug(ctx, "starting pipeline")

	pipelineSlice := []string{
		"h264timestamper name=videoparse ! h264parse ! capsfilter caps=video/x-h264,stream-format=byte-stream ! appsink name=videoappsink",
		"opusparse name=audioparse ! appsink name=audioappsink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	ok := pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
			log.Log(ctx, "got gst.MessageEOS, exiting")
			cancel()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Log(ctx, "gstreamer debug", "message", debug)
			}
			cancel()
		default:
			log.Debug(ctx, msg.String())
		}
		return true
	})

	if !ok {
		return fmt.Errorf("failed to add watch to pipeline bus")
	}

	outputQueue, done, err := ConcatStream(ctx, pipeline, "user", mv)
	if err != nil {
		return fmt.Errorf("failed to get output queue: %w", err)
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-done:
			cancel()
		}
	}()
	// queuePadVideo := outputQueue.GetRequestPad("src_%u")
	// if queuePadVideo == nil {
	// 	return fmt.Errorf("failed to get queue video pad")
	// }
	// queuePadAudio := outputQueue.GetRequestPad("src_%u")
	// if queuePadAudio == nil {
	// 	return fmt.Errorf("failed to get queue audio pad")
	// }

	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	err = outputQueue.Link(videoParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to video parse: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	err = outputQueue.Link(audioParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to audio parse: %w", err)
	}

	go func() {
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
	}()

	go func() {
		ticker := time.NewTicker(time.Second * 1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				state := pipeline.GetCurrentState()
				log.Debug(ctx, "pipeline state", "state", state)
			}
		}
	}()

	videoappsinkele, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}

	audioappsinkele, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return fmt.Errorf("failed to get audio sink element from pipeline: %w", err)
	}

	videoappsink := app.SinkFromElement(videoappsinkele)
	videoappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			clockTime := buffer.Duration()
			dur := clockTime.AsDuration()

			mediaSample := media.Sample{Data: samples}
			if dur != nil {
				mediaSample.Duration = *dur
			} else {
				log.Log(ctx, "no video duration", "samples", len(samples), "segment duration", sample.GetSegment().GetDuration())
				// cancel()
				return gst.FlowOK
			}

			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			log.Warn(ctx, "videoappsink EOSFunc")
			cancel()
		},
	})

	audioappsink := app.SinkFromElement(audioappsinkele)
	audioappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			clockTime := buffer.Duration()
			dur := clockTime.AsDuration()
			mediaSample := media.Sample{Data: samples}
			if dur != nil {
				mediaSample.Duration = *dur
			} else {
				log.Log(ctx, "no audio duration", "samples", len(samples))
				// cancel()
				return gst.FlowOK
			}

			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			log.Warn(ctx, "audioappsink EOSFunc")
			cancel()
		},
	})

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)
	log.Warn(ctx, "playing pipeline")

	<-ctx.Done()
	log.Warn(ctx, "!!!!!!!!!!!!!!!!!!!!!!! ctx done")
	return nil
}
