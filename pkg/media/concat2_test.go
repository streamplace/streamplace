package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

func TestConcatBin(t *testing.T) {
	withNoGSTLeaks(t, func() {

		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				return innerTestConcatBin(t)
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

// This function remains in scope for the duration of a single users' playback
func innerTestConcatBin(t *testing.T) error {
	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatStream": 9, "ConcatBin": 9, "SegDemuxBin": 9}})
	tag := os.Getenv("TEST_TAG")
	uuid, _ := uuid.NewV7()
	uuidStr := uuid.String()
	if tag != "" {
		ctx = log.WithLogValues(ctx, "tag", tag)
		uuidStr = fmt.Sprintf("%s-%s", tag, uuidStr)
	}
	ctx = log.WithLogValues(ctx, "func", "ConcatBin", "uuid", uuidStr)

	pipeline, err := gst.NewPipeline("TestConcatBin")
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
		require.NoError(t, err, fmt.Sprintf("uuid: %s", uuidStr))
		err = pipeline.BlockSetState(gst.StateNull)
		require.NoError(t, err, fmt.Sprintf("uuid: %s", uuidStr))
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

	testSegs := []*bus.Seg{}
	for range 5 {
		testSegs = append(testSegs, &bus.Seg{
			Data:     bs,
			Filepath: filename,
		})
	}

	segCh := make(chan *bus.Seg)
	go func() {
		for _, seg := range testSegs {
			segCh <- seg
		}
		close(segCh)
	}()

	concatBin, err := ConcatBin(ctx, segCh, true)
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

	// Start a goroutine to print buffer sizes
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				log.Debug(ctx, "buffer sizes",
					"videoBuf", videoBuf.Len(),
					"audioBuf", audioBuf.Len())
			}
		}
	}()

	<-ctx.Done()

	time.Sleep(5 * time.Second)

	padIdleCh := make(chan struct{})

	padIdle := func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		log.Debug(ctx, "pad-idle", "name", pad.GetName(), "direction", pad.GetDirection())
		go func() {
			padIdleCh <- struct{}{}
		}()
		return gst.PadProbeRemove
	}

	videoAppSinkPadSink.AddProbe(gst.PadProbeTypeIdle, padIdle)
	audioAppSinkPadSink.AddProbe(gst.PadProbeTypeIdle, padIdle)

	<-padIdleCh
	<-padIdleCh

	require.Equal(t, 4936455, videoBuf.Len(), fmt.Sprintf("uuid: %s", uuidStr))
	require.Equal(t, 32200, audioBuf.Len(), fmt.Sprintf("uuid: %s", uuidStr))

	return <-errCh
}
