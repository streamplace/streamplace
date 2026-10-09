package media

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/gstinit"
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

func TestBusHandlerTagMessages(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		verbosity    string
		debugContext bool
	}{
		{name: "disabled", verbosity: "0"},
		{name: "global", verbosity: "4"},
		{name: "context", verbosity: "0", debugContext: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			logs := captureLogs(t, testCase.verbosity)

			withNoGSTLeaks(t, func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if testCase.debugContext {
					ctx = log.WithLogValues(ctx, "func", "TestBusHandlerTagMessages")
					ctx = log.WithDebugValue(ctx, map[string]map[string]int{"func": {"TestBusHandlerTagMessages": 4}})
				}
				pipeline, err := gst.NewPipeline("bus-message-test")
				require.NoError(t, err)
				tags := gst.NewTagListFromString("taglist, title=(string)streamplace-bus-tag-payload;")
				require.NotNil(t, tags)
				tagMessage := gst.NewTagMessage(pipeline, tags)
				eosMessage := gst.NewEOSMessage(pipeline)
				require.NotNil(t, tagMessage)
				require.NotNil(t, eosMessage)
				bus := pipeline.GetPipelineBus()
				const tagCount = 32
				for range tagCount {
					require.True(t, bus.Post(tagMessage))
				}
				require.True(t, bus.Post(eosMessage))
				seen := make([]gst.MessageType, 0, tagCount+1)
				err = HandleBusMessagesCustom(ctx, pipeline, func(msg *gst.Message) {
					seen = append(seen, msg.Type())
				})
				// The posted fixture retains its native references through dispatch.
				runtime.KeepAlive(tags)
				runtime.KeepAlive(tagMessage)
				runtime.KeepAlive(eosMessage)
				require.NoError(t, err)
				require.Len(t, seen, tagCount+1)
				for _, messageType := range seen[:tagCount] {
					require.Equal(t, gst.MessageTag, messageType)
				}
				require.Equal(t, gst.MessageEOS, seen[tagCount])
			})
			if testCase.verbosity == "0" && !testCase.debugContext {
				require.Empty(t, logs.String())
			} else {
				require.Contains(t, logs.String(), "type=tag")
				require.NotContains(t, logs.String(), "streamplace-bus-tag-payload")
			}
		})
	}
}

func BenchmarkBusMessageHandling(b *testing.B) {
	gstinit.InitGST()
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.Cleanup(func() { slog.SetDefault(previousLogger) })
	previousVerbosity := flag.Lookup("v").Value.String()
	require.NoError(b, flag.Set("v", "0"))
	b.Cleanup(func() { require.NoError(b, flag.Set("v", previousVerbosity)) })
	pipeline, err := gst.NewPipeline("bus-message-benchmark")
	require.NoError(b, err)
	tags := gst.NewTagListFromString("taglist, title=(string)streamplace-bus-tag-payload;")
	require.NotNil(b, tags)
	tagMessage := gst.NewTagMessage(pipeline, tags)
	eosMessage := gst.NewEOSMessage(pipeline)
	require.NotNil(b, tagMessage)
	require.NotNil(b, eosMessage)
	bus := pipeline.GetPipelineBus()
	const tagCount = 32
	b.ReportAllocs()
	for b.Loop() {
		for range tagCount {
			if !bus.Post(tagMessage) {
				b.Fatal("failed to post tag message")
			}
		}
		if !bus.Post(eosMessage) {
			b.Fatal("failed to post EOS message")
		}
		if err := HandleBusMessages(context.Background(), pipeline); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(tagCount+1, "messages/op")
	runtime.KeepAlive(tags)
	runtime.KeepAlive(tagMessage)
	runtime.KeepAlive(eosMessage)
}
