package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
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

func innerTestConcatDemuxBin(t *testing.T) error {
	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatStream": 9, "TestConcat2": 9, "SegDemuxBin": 9}})
	ctx = log.WithLogValues(ctx, "func", "TestConcat2")

	pipeline := gst.NewPipeline("TestConcat2").(gst.Pipeline)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		errCh <- err
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

	concatBin, err := ConcatDemuxBin(ctx, testSeg, true, 1)
	if err != nil {
		return fmt.Errorf("failed to create concat bin: %w", err)
	}

	if !pipeline.Add(concatBin) {
		return fmt.Errorf("failed to add concat bin to pipeline")
	}

	videoPad := concatBin.GetStaticPad("video_0")
	if videoPad == nil {
		return fmt.Errorf("video pad not found")
	}

	audioPad := concatBin.GetStaticPad("audio_0")
	if audioPad == nil {
		return fmt.Errorf("audio pad not found")
	}

	videoAppSink := gst.ElementFactoryMakeWithProperties("appsink", map[string]interface{}{
		"name": "videoappsink",
		"sync": false,
	})
	if videoAppSink == nil {
		return fmt.Errorf("failed to create video appsink")
	}

	if !pipeline.Add(videoAppSink) {
		return fmt.Errorf("failed to add video appsink to pipeline")
	}

	videoAppSinkPadSink := videoAppSink.GetStaticPad("sink")
	if videoAppSinkPadSink == nil {
		return fmt.Errorf("video appsink pad not found")
	}

	audioAppSink := gst.ElementFactoryMakeWithProperties("appsink", map[string]interface{}{
		"name": "audioappsink",
		"sync": false,
	})
	if audioAppSink == nil {
		return fmt.Errorf("failed to create audio appsink")
	}

	if !pipeline.Add(audioAppSink) {
		return fmt.Errorf("failed to add audio appsink to pipeline")
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

	videoappsink := videoAppSink.(app.AppSink)
	videoappsink.SetCallbacks(&app.AppSinkCallbacks{
		NewSample: WriterNewSample(ctx, &videoBuf),
	})

	audioappsink := audioAppSink.(app.AppSink)
	audioappsink.SetCallbacks(&app.AppSinkCallbacks{
		NewSample: WriterNewSample(ctx, &audioBuf),
	})

	// Start the pipeline
	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline to playing state")
	}

	defer func() {
		if err != nil {
			t.Errorf("bus handler error: %v", err)
		}
		if pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone) != gst.StateChangeSuccess {
			t.Error("failed to set pipeline to null state")
		}
		require.Equal(t, 987291, videoBuf.Len())
		require.Equal(t, 6440, audioBuf.Len())
	}()

	return <-errCh
}
