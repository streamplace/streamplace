package media

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

func TestBusHandlerCleanup(t *testing.T) {
	withNoGSTLeaks(t, func() {

		g, ctx := errgroup.WithContext(context.Background())
		ctx = log.WithDebugValue(ctx, map[string]map[string]int{"func": {"TestBusHandler": 9}})
		for i := range streamplaceTestCount {
			g.Go(func() error {
				err := testBusHandlerCleanupInner(ctx, i)
				if err == nil {
					return fmt.Errorf("expected error")
				}
				return nil
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

func testBusHandlerCleanupInner(ctx context.Context, i int) error {
	ctx = log.WithLogValues(ctx, "func", "TestBusHandler")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipeline, err := gst.NewPipeline(fmt.Sprintf("TestBusHandler-%d", i))
	if err != nil {
		return err
	}

	busDone := make(chan struct{})
	go func() {
		_ = HandleBusMessages(ctx, pipeline)
		busDone <- struct{}{}
		cancel()
	}()

	defer func() {
		cancel()
		<-busDone
		err = pipeline.SetState(gst.StateNull)
		if err != nil {
			panic(fmt.Sprintf("failed to set state to null: %s", err))
		}
	}()

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

	demux, err := gst.NewElementWithProperties("qtdemux", map[string]any{
		"name": fmt.Sprintf("TestBusHandler-qtdemux-%d", i),
	})
	if err != nil {
		return err
	}
	err = pipeline.Add(demux)
	if err != nil {
		return err
	}

	appSink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name": fmt.Sprintf("TestBusHandler-appsink-%d", i),
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
		NewSampleFunc: WriterNewSample(ctx, nil),
	})

	return fmt.Errorf("test error")

}
