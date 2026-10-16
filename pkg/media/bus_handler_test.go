package media

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
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

	pipeline := gst.NewPipeline(fmt.Sprintf("TestBusHandler-%d", i)).(gst.Pipeline)

	busDone := make(chan struct{})
	go func() {
		_ = HandleBusMessages(ctx, pipeline)
		busDone <- struct{}{}
		cancel()
	}()

	defer func() {
		cancel()
		<-busDone
		if pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone) != gst.StateChangeSuccess {
			panic("failed to set state to null")
		}
	}()

	fileSrc := gst.ElementFactoryMakeWithProperties("filesrc", map[string]any{
		"location": getFixture("5sec.mp4"),
	})
	if fileSrc == nil {
		return fmt.Errorf("failed to create filesrc")
	}
	if !pipeline.Add(fileSrc) {
		return fmt.Errorf("failed to add filesrc")
	}

	demux := gst.ElementFactoryMakeWithProperties("qtdemux", map[string]any{
		"name": fmt.Sprintf("TestBusHandler-qtdemux-%d", i),
	})
	if demux == nil {
		return fmt.Errorf("failed to create demux")
	}
	if !pipeline.Add(demux) {
		return fmt.Errorf("failed to add demux")
	}

	appSink := gst.ElementFactoryMakeWithProperties("appsink", map[string]any{
		"name": fmt.Sprintf("TestBusHandler-appsink-%d", i),
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
		NewSample: WriterNewSample(ctx, nil),
	})

	return fmt.Errorf("test error")

}
