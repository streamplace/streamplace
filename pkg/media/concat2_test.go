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
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
)

func TestConcat2(t *testing.T) {
	gstinit.InitGST()
	// before := getLeakCount(t)
	// defer checkGStreamerLeaks(t, before)
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)

	innnerTestConcat2(t)
}

// This function remains in scope for the duration of a single users' playback
func innnerTestConcat2(t *testing.T) error {
	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatStream": 9, "TestConcat2": 9}})
	ctx = log.WithLogValues(ctx, "func", "TestConcat2")
	ctx, cancel := context.WithCancel(ctx)
	// defer cancel()

	pipeline, err := gst.NewPipeline("TestConcat2")
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
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
		err, ok := <-errCh
		if err != nil {
			t.Errorf("bus handler error: %v", err)
		}
		if !ok {
			t.Error("error channel closed unexpectedly")
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

	testSegs := []*segchanman.Seg{
		{
			Data:     bs,
			Filepath: filename,
		},
	}

	segCh := make(chan *segchanman.Seg)
	go func() {
		for _, seg := range testSegs {
			segCh <- seg
		}
	}()

	concatBin, err := NewConcatBin(ctx, segCh)
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

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Println(fmt.Sprintf("videoBuf.Len(): %d", videoBuf.Len()))
			case <-ctx.Done():
				return
			}
		}
	}()
	<-ctx.Done()

	if videoBuf.Len() != 347001 {
		t.Errorf("expected video buffer length 347001, got %d", videoBuf.Len())
	}
	if audioBuf.Len() != 40000 {
		t.Errorf("expected audio buffer length 40000, got %d", audioBuf.Len())
	}

	return <-errCh
}
