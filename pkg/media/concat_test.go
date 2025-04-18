package media

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
)

type TestConcatStreamer struct {
	fileName string
	data     []byte
	count    int
}

func (t *TestConcatStreamer) SubscribeSegment(ctx context.Context, user string, rendition string) <-chan *segchanman.Seg {
	if len(t.data) == 0 {
		panic("test file empty")
	}
	ch := make(chan *segchanman.Seg)
	go func() {
		if t.count == 5 {
			ch <- &segchanman.Seg{
				Data:     nil,
				Filepath: "",
			}
		} else {
			fmt.Println("writing segment " + strconv.Itoa(t.count) + " with random number " + strconv.Itoa(rand.Intn(100)))
			ch <- &segchanman.Seg{
				Data:     t.data,
				Filepath: t.fileName,
			}
			t.count += 1
		}
	}()
	return ch
}

func (t *TestConcatStreamer) UnsubscribeSegment(ctx context.Context, user string, rendition string, ch <-chan *segchanman.Seg) {
}

func TestConcat(t *testing.T) {
	gstinit.InitGST()
	// before := getLeakCount(t)
	// defer checkGStreamerLeaks(t, before)
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)

	innnerTestConcat(t)
	after := getLeakCount(t)
	if after != 6 {
		fmt.Println("leaks", after)
	}
}

// This function remains in scope for the duration of a single users' playback
func innnerTestConcat(t *testing.T) {

	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatStream": 9, "TestConcat": 9}})
	ctx = log.WithLogValues(ctx, "func", "TestConcat")
	ctx, cancel := context.WithCancel(ctx)
	// defer cancel()

	pipeline, err := gst.NewPipeline("TestConcat")
	require.NoError(t, err)

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	filename := getFixture("sample-segment.mp4")
	inputFile, err := os.Open(filename)
	require.NoError(t, err)
	bs, err := io.ReadAll(inputFile)
	require.NoError(t, err)
	tcs := &TestConcatStreamer{
		fileName: getFixture("sample-segment.mp4"),
		data:     bs,
	}

	outputQueue, done, err := ConcatStream(ctx, pipeline, "fakeuser", "fakerendition", tcs)
	require.NoError(t, err)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-done:
			cancel()
		}
	}()

	videoPad := outputQueue.GetStaticPad("src_0")
	require.NotNil(t, videoPad)

	audioPad := outputQueue.GetStaticPad("src_1")
	require.NotNil(t, audioPad)

	videoAppSink, err := gst.NewElementWithProperties("appsink", map[string]interface{}{
		"name":        "videoappsink",
		"sync":        false,
		"wait-on-eos": false,
	})
	require.NoError(t, err)
	err = pipeline.Add(videoAppSink)
	require.NoError(t, err)

	videoAppSinkPadSink := videoAppSink.GetStaticPad("sink")
	require.NotNil(t, videoAppSinkPadSink)

	audioAppSink, err := gst.NewElementWithProperties("appsink", map[string]interface{}{
		"name":        "audioappsink",
		"sync":        false,
		"wait-on-eos": false,
	})
	require.NoError(t, err)
	err = pipeline.Add(audioAppSink)
	require.NoError(t, err)

	audioAppSinkPadSink := audioAppSink.GetStaticPad("sink")
	require.NotNil(t, audioAppSinkPadSink)

	ok := videoPad.Link(videoAppSinkPadSink)
	require.Equal(t, gst.PadLinkOK, ok)

	ok = audioPad.Link(audioAppSinkPadSink)
	require.Equal(t, gst.PadLinkOK, ok)

	videoTotalBytes := 0
	audioTotalBytes := 0

	videoappsink := app.SinkFromElement(videoAppSink)
	videoappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			videoTotalBytes += len(samples)

			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			log.Warn(ctx, "videoappsink EOSFunc")
			cancel()
		},
	})

	audioappsink := app.SinkFromElement(audioAppSink)
	audioappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			audioTotalBytes += len(samples)

			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			log.Warn(ctx, "audioappsink EOSFunc")
			cancel()
		},
	})

	// Start the pipeline

	err = pipeline.SetState(gst.StatePlaying)
	require.NoError(t, err)

	<-ctx.Done()

	err = pipeline.BlockSetState(gst.StateNull)
	require.NoError(t, err)
	pipeline.Remove(videoAppSink)
	pipeline.Remove(audioAppSink)
	videoAppSink.SetState(gst.StateNull)
	audioAppSink.SetState(gst.StateNull)
	videoappsink.SetCallbacks(&app.SinkCallbacks{})
	audioappsink.SetCallbacks(&app.SinkCallbacks{})
	pipeline.Clear()

	require.Greater(t, videoTotalBytes, 1000000)
	require.Greater(t, audioTotalBytes, 40000)
}
