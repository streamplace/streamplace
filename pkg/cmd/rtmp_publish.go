package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
)

// rtmpPublishFile streams file, an mp4 with H.264 video and Opus audio like
// the e2e fixture, to rtmpURL (rtmp://host:port/app/streamkey) at realtime, as
// an encoder would, and returns when the file ends. RTMP carries AAC rather
// than Opus, so the audio is transcoded. The client's tcUrl is
// rtmp://host:port/app, from the URL as given.
func rtmpPublishFile(ctx context.Context, file, rtmpURL string) (retErr error) {
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		"filesrc name=filesrc ! qtdemux name=demux",
		// rtmp2sink syncs to the clock (the basesink default), which is what
		// paces the file at realtime
		"flvmux name=mux streamable=true ! rtmp2sink name=rtmpsink",
		"demux.video_0 ! queue ! h264parse ! mux.video",
		"demux.audio_0 ! queue ! opusparse ! opusdec ! audioconvert ! audioresample ! fdkaacenc ! aacparse ! mux.audio",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create RTMP publish pipeline: %w", err)
	}
	defer func() {
		cancel()
		if err := pipeline.BlockSetState(gst.StateNull); err != nil && retErr == nil {
			retErr = err
		}
	}()

	fileSrc, err := pipeline.GetElementByName("filesrc")
	if err != nil {
		return err
	}
	if err := fileSrc.Set("location", file); err != nil {
		return err
	}
	sink, err := pipeline.GetElementByName("rtmpsink")
	if err != nil {
		return err
	}
	if err := sink.Set("location", rtmpURL); err != nil {
		return err
	}

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("failed to start RTMP publish pipeline: %w", err)
	}
	return media.HandleBusMessages(ctx, pipeline)
}
