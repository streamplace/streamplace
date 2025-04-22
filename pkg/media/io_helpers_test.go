package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

var streamplaceTestCount = 50

func init() {
	testRunsStr := os.Getenv("STREAMPLACE_TEST_COUNT")
	if testRunsStr != "" {
		var err error
		streamplaceTestCount, err = strconv.Atoi(testRunsStr)
		if err != nil {
			panic(fmt.Sprintf("STREAMPLACE_TEST_COUNT is not a number: %s", testRunsStr))
		}
	}
}

func TestWriterNewSample(t *testing.T) {
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)
	before := getLeakCount(t)
	defer checkGStreamerLeaks(t, before)
	filePath := getFixture("5sec.mp4")
	fileInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	fileSize := fileInfo.Size()
	t.Logf("Test file size: %d bytes", fileSize)
	g, ctx := errgroup.WithContext(context.Background())
	ctx = log.WithDebugValue(ctx, map[string]map[string]int{"func": {"TestWriterNewSample": 9}})
	for i := 0; i < streamplaceTestCount; i++ {
		g.Go(func() error {
			bs := bytes.Buffer{}
			err := writerNewSampleInner(ctx, i, &bs)
			if err != nil {
				return err
			}
			if bs.Len() != int(fileSize) {
				return fmt.Errorf("expected %d bytes, got %d", fileSize, bs.Len())
			}
			return nil
		})
	}
	err = g.Wait()
	require.NoError(t, err)
}

func writerNewSampleInner(ctx context.Context, i int, w io.Writer) error {
	ctx = log.WithLogValues(ctx, "func", "TestWriterNewSample")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipeline, err := gst.NewPipeline(fmt.Sprintf("TestWriterNewSample-%d", i))
	if err != nil {
		return err
	}

	fileSrc, err := gst.NewElementWithProperties("filesrc", map[string]any{
		"location": getFixture("5sec.mp4"),
	})
	if err != nil {
		return err
	}
	err = pipeline.Add(fileSrc)
	if err != nil {
		return err
	}

	var busErr error
	go func() {
		busErr = HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	appSink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name": fmt.Sprintf("TestWriterNewSample-appsink-%d", i),
		"sync": false,
	})
	if err != nil {
		return err
	}
	err = pipeline.Add(appSink)
	if err != nil {
		return err
	}

	sink := app.SinkFromElement(appSink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	err = fileSrc.Link(appSink)
	if err != nil {
		return err
	}

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	<-ctx.Done()

	err = pipeline.SetState(gst.StateNull)
	if err != nil {
		return err
	}

	return busErr
}
