package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/go-gst/go-gst/gst/pbutils"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/log"
)

func TestAudioSmear(t *testing.T) {

	uri := getFixture("duration-mismatch.mp4")

	// info, err := discoverer.DiscoverURI(fmt.Sprintf("file://%s", uri))
	// if err != nil {
	// 	panic(err)
	// }

	f, err := os.Open(uri)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// audioBs := bytes.Buffer{}
	// videoBs := bytes.Buffer{}

	seg, err := ToBuffers(context.Background(), f)
	require.NoError(t, err)

	err = seg.Normalize(context.Background())
	require.NoError(t, err)

	buf := bytes.Buffer{}
	_ = JoinAudioVideo(context.Background(), seg, &buf)

	// require.NoError(t, err)

	// require.Equal(t, 1191255, buf.Len())
	// Write the buffer to a file
	outputPath := "/home/iameli/code/streamplace/output.mp4"
	outputFile, err := os.Create(outputPath)
	require.NoError(t, err)

	_, err = io.Copy(outputFile, &buf)
	require.NoError(t, err)

	err = outputFile.Close()
	require.NoError(t, err)

	t.Logf("Successfully wrote output to %s", outputPath)

	// require.Equal(t, 1180120, videoBs.Len())

	// // Write audio and video buffers to temporary files for further analysis
	// tempDir := t.TempDir()

	// audioFilePath := fmt.Sprintf("%s/audio.mp4", tempDir)
	// videoFilePath := fmt.Sprintf("%s/video.mp4", tempDir)

	// // Write audio buffer to file
	// audioFile, err := os.Create(audioFilePath)
	// require.NoError(t, err)
	// _, err = io.Copy(audioFile, bytes.NewReader(audioBs.Bytes()))
	// require.NoError(t, err)
	// err = audioFile.Close()
	// require.NoError(t, err)

	// // Write video buffer to file
	// videoFile, err := os.Create(videoFilePath)
	// require.NoError(t, err)
	// _, err = io.Copy(videoFile, bytes.NewReader(videoBs.Bytes()))
	// require.NoError(t, err)
	// err = videoFile.Close()
	// require.NoError(t, err)

	// SmearAudioTimestamps(context.Background(), bytes.NewReader(audioBs.Bytes()), &bytes.Buffer{})

	// checkSame(t, videoFile.Name(), getFixture("duration-mismatch-video.mp4"))
	// checkSame(t, audioFile.Name(), getFixture("duration-mismatch-audio.mp4"))
	// printDiscovererInfo(info)

}

func checkSame(t *testing.T, v1, v2 string) {
	discoverer, err := pbutils.NewDiscoverer(gst.ClockTime(time.Second * 15))
	if err != nil {
		panic(err)
	}

	info, err := discoverer.DiscoverURI(fmt.Sprintf("file://%s", v1))
	require.NoError(t, err)
	dur1 := info.GetDuration().AsDuration()

	info, err = discoverer.DiscoverURI(fmt.Sprintf("file://%s", v2))
	require.NoError(t, err)
	dur2 := info.GetDuration().AsDuration()

	require.Equal(t, *dur2, *dur1)
}

func SplitAudioVideo(ctx context.Context, input io.Reader, audioOut, videoOut io.Writer) error {
	ctx = log.WithLogValues(ctx, "func", "SplitAudioVideo")

	pipelineSlice := []string{
		"appsrc name=mp4src ! qtdemux name=demux",
		"demux.video_0 ! queue ! h264parse name=videoparse ! appsink sync=false name=videoappsink",
		"demux.audio_0 ! queue ! opusparse name=audioparse ! appsink sync=false name=audioappsink",
	}

	ctx, cancel := context.WithCancel(ctx)

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		cancel()
		errCh <- err
		close(errCh)
	}()

	defer func() {
		cancel()
		err := <-errCh
		if err != nil {
			log.Error(ctx, "bus handler error", "error", err)
		}
		err = pipeline.BlockSetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "failed to set pipeline to null state", "error", err)
		}
	}()

	mp4src, err := pipeline.GetElementByName("mp4src")
	if err != nil {
		return fmt.Errorf("failed to get mp4src element: %w", err)
	}
	src := app.SrcFromElement(mp4src)
	if src == nil {
		return fmt.Errorf("failed to get mp4src element: %w", err)
	}
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, input),
	})

	audioSinkElem, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return fmt.Errorf("failed to get audioappsink element: %w", err)
	}
	audioSink := app.SinkFromElement(audioSinkElem)
	if audioSink == nil {
		return fmt.Errorf("failed to get audioappsink element: %w", err)
	}
	audioSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()
			log.Log(ctx, "audio buffer", "presentation_timestamp", buffer.PresentationTimestamp(), "duration", buffer.Duration())
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			_, err := audioOut.Write(bs)

			if err != nil {
				panic(err)
			}

			return gst.FlowOK
		},
	})

	videoSinkElem, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return fmt.Errorf("failed to get videoappsink element: %w", err)
	}
	videoSink := app.SinkFromElement(videoSinkElem)
	if videoSink == nil {
		return fmt.Errorf("failed to get videoappsink element: %w", err)
	}
	videoSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()
			log.Log(ctx, "video buffer", "presentation_timestamp", buffer.PresentationTimestamp(), "duration", buffer.Duration())
			bs := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			_, err := videoOut.Write(bs)

			if err != nil {
				panic(err)
			}

			return gst.FlowOK
		},
	})

	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	return <-errCh
}

// func SmearAudioTimestamps(ctx context.Context, input io.Reader, audioOut io.Writer) error {
// 	ctx = log.WithLogValues(ctx, "func", "SplitAudioVideo")

// 	pipelineSlice := []string{
// 		"appsrc name=mp4src ! qtdemux name=demux",
// 		"demux.audio_0 ! queue ! opusparse name=audioparse ! appsink sync=false name=audioappsink",
// 	}

// 	ctx, cancel := context.WithCancel(ctx)

// 	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
// 	if err != nil {
// 		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
// 	}

// 	errCh := make(chan error)
// 	go func() {
// 		err := HandleBusMessages(ctx, pipeline)
// 		cancel()
// 		errCh <- err
// 		close(errCh)
// 	}()

// 	defer func() {
// 		cancel()
// 		err := <-errCh
// 		if err != nil {
// 			log.Error(ctx, "bus handler error", "error", err)
// 		}
// 		err = pipeline.BlockSetState(gst.StateNull)
// 		if err != nil {
// 			log.Error(ctx, "failed to set pipeline to null state", "error", err)
// 		}
// 	}()

// 	mp4src, err := pipeline.GetElementByName("mp4src")
// 	if err != nil {
// 		return fmt.Errorf("failed to get mp4src element: %w", err)
// 	}
// 	src := app.SrcFromElement(mp4src)
// 	if src == nil {
// 		return fmt.Errorf("failed to get mp4src element: %w", err)
// 	}
// 	src.SetCallbacks(&app.SourceCallbacks{
// 		NeedDataFunc: ReaderNeedData(ctx, input),
// 	})

// 	audioSinkElem, err := pipeline.GetElementByName("audioappsink")
// 	if err != nil {
// 		return fmt.Errorf("failed to get audioappsink element: %w", err)
// 	}
// 	audioSink := app.SinkFromElement(audioSinkElem)
// 	if audioSink == nil {
// 		return fmt.Errorf("failed to get audioappsink element: %w", err)
// 	}
// 	audioSink.SetCallbacks(&app.SinkCallbacks{
// 		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
// 			sample := sink.PullSample()
// 			if sample == nil {
// 				return gst.FlowOK
// 			}

// 			// Retrieve the buffer from the sample.
// 			buffer := sample.GetBuffer()
// 			log.Log(ctx, "buffer", "presentation_timestamp", buffer.PresentationTimestamp(), "duration", buffer.Duration())
// 			bs := buffer.Map(gst.MapRead).Bytes()
// 			defer buffer.Unmap()

// 			_, err := audioOut.Write(bs)

// 			if err != nil {
// 				panic(err)
// 			}

// 			return gst.FlowOK
// 		},
// 	})

// 	pipeline.SetState(gst.StatePlaying)

// 	<-ctx.Done()

// 	return <-errCh
// }

func printDiscovererInfo(info *pbutils.DiscovererInfo) {
	fmt.Println("URI:", info.GetURI())
	fmt.Println("Duration:", info.GetDuration())

	printTags(info)
	printStreamInfo(info.GetStreamInfo())

	children := info.GetStreamList()
	fmt.Println("Children streams:")
	for _, child := range children {
		printStreamInfo(child)
	}
}

func printTags(info *pbutils.DiscovererInfo) {
	fmt.Println("Tags:")
	tags := info.GetTags()
	if tags != nil {
		fmt.Println("  ", tags)
		return
	}
	fmt.Println("  no tags")
}

func printStreamInfo(info *pbutils.DiscovererStreamInfo) {
	if info == nil {
		return
	}
	fmt.Println("Stream: ")
	fmt.Println("  Stream id:", info.GetStreamID())
	if caps := info.GetCaps(); caps != nil {
		fmt.Println("  Format:", caps)
	}
}
