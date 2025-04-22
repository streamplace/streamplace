package media

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

var streamplaceTestRuns = 50

func init() {
	testRunsStr := os.Getenv("STREAMPLACE_TEST_RUNS")
	if testRunsStr != "" {
		var err error
		streamplaceTestRuns, err = strconv.Atoi(testRunsStr)
		if err != nil {
			panic(fmt.Sprintf("STREAMPLACE_TEST_RUNS is not a number: %s", testRunsStr))
		}
	}
}

func TestWriterNewSample(t *testing.T) {
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)
	before := getLeakCount(t)
	defer checkGStreamerLeaks(t, before)
	g, ctx := errgroup.WithContext(context.Background())
	ctx = log.WithDebugValue(ctx, map[string]map[string]int{"func": {"TestWriterNewSample": 9}})
	for i := 0; i < streamplaceTestRuns; i++ {
		g.Go(func() error {
			return writerNewSampleInner(ctx)
		})
	}
	err := g.Wait()
	require.NoError(t, err)
}

func writerNewSampleInner(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "func", "TestWriterNewSample")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipeline, err := gst.NewPipeline("TestWriterNewSample")
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

	fakeSink, err := gst.NewElementWithProperties("fakesink", map[string]any{
		"sync": false,
	})
	if err != nil {
		return err
	}
	err = pipeline.Add(fakeSink)
	if err != nil {
		return err
	}

	err = fileSrc.Link(fakeSink)
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
