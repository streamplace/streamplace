package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

func TestConcatDemuxBin(t *testing.T) {
	withNoGSTLeaks(t, func() {
		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				return innerTestConcatDemuxBin(t)
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

// This function remains in scope for the duration of a single users' playback
func innerTestConcatDemuxBin(t *testing.T) error {
	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatStream": 9, "TestConcat2": 9, "SegDemuxBin": 9}})
	ctx = log.WithLogValues(ctx, "func", "TestConcat2")

	pipeline, err := gst.NewPipeline("TestConcat2")
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

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
			t.Errorf("bus handler error: %v", err)
		}
		err = pipeline.BlockSetState(gst.StateNull)
		if err != nil {
			t.Errorf("failed to set pipeline to null state: %v", err)
		}
	}()

	filename := getFixture("sample-segment.mp4")
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open fixture file: %w", err)
	}
	defer inputFile.Close()

	bs, err := io.ReadAll(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read fixture file: %w", err)
	}

	testSeg := &bus.Seg{
		Data:     bs,
		Filepath: filename,
	}

	concatBin, err := ConcatDemuxBin(ctx, testSeg)
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

	videoAppSink, err := gst.NewElementWithProperties("appsink", map[string]interface{}{
		"name": "videoappsink",
		"sync": false,
	})
	if err != nil {
		return fmt.Errorf("failed to create video appsink: %w", err)
	}

	err = pipeline.Add(videoAppSink)
	if err != nil {
		return fmt.Errorf("failed to add video appsink to pipeline: %w", err)
	}

	videoAppSinkPadSink := videoAppSink.GetStaticPad("sink")
	if videoAppSinkPadSink == nil {
		return fmt.Errorf("video appsink pad not found")
	}

	audioAppSink, err := gst.NewElementWithProperties("appsink", map[string]interface{}{
		"name": "audioappsink",
		"sync": false,
	})
	if err != nil {
		return fmt.Errorf("failed to create audio appsink: %w", err)
	}

	err = pipeline.Add(audioAppSink)
	if err != nil {
		return fmt.Errorf("failed to add audio appsink to pipeline: %w", err)
	}

	audioAppSinkPadSink := audioAppSink.GetStaticPad("sink")
	if audioAppSinkPadSink == nil {
		return fmt.Errorf("audio appsink pad not found")
	}

	ok := videoPad.Link(videoAppSinkPadSink)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link video pad: %v", ok)
	}

	ok = audioPad.Link(audioAppSinkPadSink)
	if ok != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio pad: %v", ok)
	}

	videoBuf := bytes.Buffer{}
	audioBuf := bytes.Buffer{}

	videoappsink := app.SinkFromElement(videoAppSink)
	videoappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &videoBuf),
	})

	audioappsink := app.SinkFromElement(audioAppSink)
	audioappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &audioBuf),
	})

	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return fmt.Errorf("failed to set pipeline to playing state: %w", err)
	}

	<-ctx.Done()

	require.Equal(t, 987248, videoBuf.Len())
	require.Equal(t, 6440, audioBuf.Len())

	return <-errCh
}
