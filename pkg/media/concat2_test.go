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

// TestAddConcatDemuxerUnblocksOnCancel is a regression test for a
// goroutine leak: addConcatDemuxer waits for EOS on both the video and
// audio demux src pads before returning. A segment that only ever emits
// EOS on one pad (audio-only, malformed, or a session torn down before
// the stream completes) used to wedge the goroutine — and the demux
// bin's GStreamer state — forever, with no way for the parent context
// to interrupt it. Over a long-lived server these accumulated until a
// restart.
//
// We feed the audio-only fixture so exactly one EOS ever fires, then
// cancel the context. The fix makes addConcatDemuxer observe
// cancellation and return; before the fix this blocks forever and the
// test trips its deadline.
func TestAddConcatDemuxerUnblocksOnCancel(t *testing.T) {
	ctx := log.WithLogValues(context.Background(), "test", "TestAddConcatDemuxerUnblocksOnCancel")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipeline, err := gst.NewPipeline("TestAddConcatDemuxerUnblocksOnCancel")
	require.NoError(t, err)

	// Minimal stand-in for the concat bin's wiring: a streamsynchronizer
	// whose two request sink pads are the link points addConcatDemuxer
	// expects, with fakesinks downstream so data actually flows.
	bin := gst.NewBin("concat-bin")
	streamsync, err := gst.NewElementWithProperties("streamsynchronizer", map[string]any{"name": "ss"})
	require.NoError(t, err)
	require.NoError(t, bin.Add(streamsync))

	videoSink := streamsync.GetRequestPad("sink_%u")
	require.NotNil(t, videoSink)
	audioSink := streamsync.GetRequestPad("sink_%u")
	require.NotNil(t, audioSink)

	for i, srcName := range []string{"src_0", "src_1"} {
		fakesink, err := gst.NewElementWithProperties("fakesink", map[string]any{
			"name": fmt.Sprintf("fakesink_%d", i),
			"sync": false,
		})
		require.NoError(t, err)
		require.NoError(t, bin.Add(fakesink))
		require.Equal(t, gst.PadLinkOK,
			streamsync.GetStaticPad(srcName).Link(fakesink.GetStaticPad("sink")))
	}

	require.NoError(t, pipeline.Add(bin.Element))

	go func() { _ = HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.BlockSetState(gst.StateNull) }()

	data, err := os.ReadFile(getFixture("duration-mismatch-audio.mp4"))
	require.NoError(t, err)
	seg := &bus.Seg{Data: data, Filepath: "duration-mismatch-audio.mp4"}

	done := make(chan error, 1)
	go func() {
		done <- addConcatDemuxer(ctx, bin, seg, videoSink, audioSink, true)
	}()

	// Let the single audio EOS arrive and be consumed, leaving
	// addConcatDemuxer blocked on the second (video) EOS that never comes.
	time.Sleep(2 * time.Second)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(15 * time.Second):
		t.Fatal("addConcatDemuxer did not return after context cancellation — eosCh leak still present")
	}
}

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
