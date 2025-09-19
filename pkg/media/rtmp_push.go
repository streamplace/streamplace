package media

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

// This function remains in scope for the duration of a single users' playback
func (mm *MediaManager) RTMPPush(ctx context.Context, user string, rendition string, url string) error {
	uu, err := uuid.NewV7()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String())
	ctx = log.WithLogValues(ctx, "mediafunc", "RTMPPush")

	pipelineSlice := []string{
		"flvmux name=muxer ! rtmp2sink name=rtmp2sink",
		"h264parse name=videoparse ! muxer.video",
		"opusparse name=audioparse ! opusdec ! fdkaacenc ! muxer.audio",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err) //nolint:all
	}

	rtmp2sink, err := pipeline.GetElementByName("rtmp2sink")
	if err != nil {
		return fmt.Errorf("failed to get rtmp2sink element from pipeline: %w", err)
	}
	err = rtmp2sink.SetProperty("location", url)
	if err != nil {
		return fmt.Errorf("failed to set rtmp2sink location: %w", err)
	}

	segBuffer := make(chan *bus.Seg, 1024)
	go func() {
		segChan := mm.bus.SubscribeSegment(ctx, user, rendition)
		defer mm.bus.UnsubscribeSegment(ctx, user, rendition, segChan)
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case file := <-segChan.C:
				log.Debug(ctx, "got segment", "file", file.Filepath)
				segBuffer <- file
			}
		}
	}()

	segCh := make(chan *bus.Seg)
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case seg := <-segBuffer:
				select {
				case <-ctx.Done():
					return
				case segCh <- seg:
				}
			}
		}
	}()

	concatBin, err := ConcatBin(ctx, segCh)
	if err != nil {
		return fmt.Errorf("failed to create concat bin: %w", err)
	}

	err = pipeline.Add(concatBin.Element)
	if err != nil {
		return fmt.Errorf("failed to add concat bin to pipeline: %w", err)
	}

	videoPad := concatBin.GetStaticPad("video_0")
	if videoPad == nil {
		return fmt.Errorf("video pad not found")
	}

	audioPad := concatBin.GetStaticPad("audio_0")
	if audioPad == nil {
		return fmt.Errorf("audio pad not found")
	}

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
	videoParsePad := videoParse.GetStaticPad("sink")
	if videoParsePad == nil {
		return fmt.Errorf("video parse pad not found")
	}
	linked := videoPad.Link(videoParsePad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link video pad to video parse pad: %v", linked)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	audioParsePad := audioParse.GetStaticPad("sink")
	if audioParsePad == nil {
		return fmt.Errorf("audio parse pad not found")
	}
	linked = audioPad.Link(audioParsePad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio pad to audio parse pad: %v", linked)
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		errCh <- err
	}()

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}

	defer func() {
		err = pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline state to null", "error", err)
		}
	}()

	return <-errCh
}
