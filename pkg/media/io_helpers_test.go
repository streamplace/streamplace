package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

func TestWriterNewSample(t *testing.T) {
	withNoGSTLeaks(t, func() {
		filePath := getFixture("5sec.mp4")
		fileInfo, err := os.Stat(filePath)
		require.NoError(t, err)
		fileSize := fileInfo.Size()
		t.Logf("Test file size: %d bytes", fileSize)
		g, ctx := errgroup.WithContext(context.Background())
		// ctx = log.WithDebugValue(ctx, map[string]map[string]int{"func": {"TestWriterNewSample": 9}})
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
	})
}

// A live tiny allocation can pin a neighboring GstSample wrapper indefinitely.
// Keep the writer's markers alive through the native leak check, not just EOS.
func TestWriterNewSampleTinyAllocations(t *testing.T) {
	writer := &tinyAllocWriter{}
	withNoGSTLeaks(t, func() {
		require.NoError(t, writerNewSampleInner(context.Background(), 0, writer))
	})
	runtime.KeepAlive(writer)
}

type tinyAllocWriter struct {
	bytes.Buffer
	markers []*uint64
}

func (w *tinyAllocWriter) Write(p []byte) (int, error) {
	marker := new(uint64)
	*marker = uint64(len(p))
	w.markers = append(w.markers, marker)
	return w.Buffer.Write(p)
}

func writerNewSampleInner(ctx context.Context, i int, w io.Writer) error {
	ctx = log.WithLogValues(ctx, "func", "TestWriterNewSample")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipeline := gst.NewPipeline(fmt.Sprintf("TestWriterNewSample-%d", i)).(gst.Pipeline)

	fileSrc := gst.ElementFactoryMakeWithProperties("filesrc", map[string]any{
		"location": getFixture("5sec.mp4"),
	})
	if fileSrc == nil {
		return fmt.Errorf("failed to create filesrc")
	}
	if !pipeline.Add(fileSrc) {
		return fmt.Errorf("failed to add filesrc")
	}

	var busErr error
	go func() {
		busErr = HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	appSink := gst.ElementFactoryMakeWithProperties("appsink", map[string]any{
		"name": fmt.Sprintf("TestWriterNewSample-appsink-%d", i),
		"sync": false,
	})
	if appSink == nil {
		return fmt.Errorf("failed to create appsink")
	}
	if !pipeline.Add(appSink) {
		return fmt.Errorf("failed to add appsink")
	}

	sink := appSink.(app.AppSink)
	sink.SetCallbacks(&app.AppSinkCallbacks{
		NewSample: WriterNewSample(ctx, w),
	})

	if !fileSrc.Link(appSink) {
		return fmt.Errorf("failed to link filesrc to appsink")
	}

	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return fmt.Errorf("failed to start writer pipeline")
	}

	<-ctx.Done()

	if pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone) != gst.StateChangeSuccess {
		return fmt.Errorf("failed to stop writer pipeline")
	}

	return busErr
}
