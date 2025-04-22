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
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
)

func TestConcat2(t *testing.T) {
	gstinit.InitGST()
	before := getLeakCount(t)
	defer checkGStreamerLeaks(t, before)
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)

	innnerTestConcat2(t)
}

// This function remains in scope for the duration of a single users' playback
func innnerTestConcat2(t *testing.T) {

	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatStream": 9, "TestConcat2": 9}})
	ctx = log.WithLogValues(ctx, "func", "TestConcat2")
	ctx, cancel := context.WithCancel(ctx)
	// defer cancel()

	pipeline, err := gst.NewPipeline("TestConcat2")
	require.NoError(t, err)

	busDone := make(chan struct{})
	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
		busDone <- struct{}{}
	}()

	defer func() {
		cancel()
		<-busDone
		err = pipeline.BlockSetState(gst.StateNull)
		require.NoError(t, err)
	}()

	filename := getFixture("sample-segment.mp4")
	inputFile, err := os.Open(filename)
	require.NoError(t, err)
	defer inputFile.Close()
	bs, err := io.ReadAll(inputFile)
	require.NoError(t, err)

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
	require.NoError(t, err)
	err = pipeline.Add(concatBin.Element)
	require.NoError(t, err)

	videoPad := concatBin.GetStaticPad("src")
	require.NotNil(t, videoPad)

	// audioPad := outputQueue.GetStaticPad("src_1")
	// require.NotNil(t, audioPad)

	videoAppSink, err := gst.NewElementWithProperties("appsink", map[string]interface{}{
		"name": "videoappsink",
		"sync": false,
	})
	require.NoError(t, err)
	err = pipeline.Add(videoAppSink)
	require.NoError(t, err)

	videoAppSinkPadSink := videoAppSink.GetStaticPad("sink")
	require.NotNil(t, videoAppSinkPadSink)

	// audioAppSink, err := gst.NewElementWithProperties("appsink", map[string]interface{}{
	// 	"name": "audioappsink",
	// 	"sync": false,
	// })
	// require.NoError(t, err)
	// err = pipeline.Add(audioAppSink)
	// require.NoError(t, err)

	// audioAppSinkPadSink := audioAppSink.GetStaticPad("sink")
	// require.NotNil(t, audioAppSinkPadSink)

	ok := videoPad.Link(videoAppSinkPadSink)
	require.Equal(t, gst.PadLinkOK, ok)

	// ok = audioPad.Link(audioAppSinkPadSink)
	// require.Equal(t, gst.PadLinkOK, ok)

	videoBuf := bytes.Buffer{}
	// audioBuf := bytes.Buffer{}

	videoappsink := app.SinkFromElement(videoAppSink)
	videoappsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &videoBuf),
	})

	// audioappsink := app.SinkFromElement(audioAppSink)
	// audioappsink.SetCallbacks(&app.SinkCallbacks{
	// 	NewSampleFunc: WriterNewSample(ctx, &audioBuf),
	// })

	// Start the pipeline

	err = pipeline.SetState(gst.StatePlaying)
	require.NoError(t, err)

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

	require.Equal(t, videoBuf.Len(), 347001)
	// require.Greater(t, audioBuf.Len(), 40000)
}
