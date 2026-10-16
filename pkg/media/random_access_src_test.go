package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/log"
)

// TestRandomAccessSrcBin_Passthrough verifies that the bin drains the
// supplied ReaderAt end-to-end and the bytes arriving at an appsink
// exactly match the source data, both byte-count and content.
func TestRandomAccessSrcBin_Passthrough(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = log.WithLogValues(ctx, "test", "TestRandomAccessSrcBin_Passthrough")

		fixture, err := os.ReadFile(getFixture("sample-segment.mp4"))
		require.NoError(t, err)
		require.NotEmpty(t, fixture)

		pipeline := gst.NewPipeline("ra-src-passthrough").(gst.Pipeline)

		srcBin, err := RandomAccessSrcBin(ctx, "ra-src", bytes.NewReader(fixture), int64(len(fixture)))
		require.NoError(t, err)
		require.True(t, pipeline.Add(srcBin))

		sinkEle := gst.ElementFactoryMakeWithProperties("appsink", map[string]interface{}{
			"name": "ra-sink",
			"sync": false,
		})
		require.NotNil(t, sinkEle)
		require.True(t, pipeline.Add(sinkEle))

		ghostPad := srcBin.GetStaticPad("src")
		require.NotNil(t, ghostPad)
		sinkPad := sinkEle.GetStaticPad("sink")
		require.NotNil(t, sinkPad)
		require.Equal(t, gst.PadLinkOK, ghostPad.Link(sinkPad))

		out := &bytes.Buffer{}
		sink := sinkEle.(app.AppSink)
		sink.SetCallbacks(&app.AppSinkCallbacks{
			NewSample: WriterNewSample(ctx, out),
		})

		errCh := make(chan error, 1)
		go func() { errCh <- HandleBusMessages(ctx, pipeline) }()

		require.NotEqual(t, gst.StateChangeFailure, pipeline.SetState(gst.StatePlaying))
		require.NoError(t, <-errCh)
		require.Equal(t, gst.StateChangeSuccess, pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone))

		require.Equal(t, fixture, out.Bytes())
	})
}

// TestRandomAccessSrcBin_Seek exercises the random-access path by having a
// downstream identity element issue manual seek events. We hash the bytes
// arriving at the appsink and compare against a known good full-file hash;
// if any seek lands at the wrong offset or returns the wrong slice of
// bytes, this catches it.
func TestRandomAccessSrcBin_Seek(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = log.WithLogValues(ctx, "test", "TestRandomAccessSrcBin_Seek")

		fixture, err := os.ReadFile(getFixture("sample-segment.mp4"))
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(fixture), 1024)

		// Read three disjoint regions via two seeks. Compare to direct
		// fixture slices.
		reader := bytes.NewReader(fixture)
		size := int64(len(fixture))

		check := func(name string, offset, length int64) {
			pipeline := gst.NewPipeline("ra-src-seek-" + name).(gst.Pipeline)

			srcBin, err := RandomAccessSrcBin(ctx, "ra-src", reader, size)
			require.NoError(t, err)
			require.True(t, pipeline.Add(srcBin))

			sinkEle := gst.ElementFactoryMakeWithProperties("appsink", map[string]interface{}{
				"name": "ra-sink",
				"sync": false,
			})
			require.NotNil(t, sinkEle)
			require.True(t, pipeline.Add(sinkEle))

			require.Equal(t, gst.PadLinkOK, srcBin.GetStaticPad("src").Link(sinkEle.GetStaticPad("sink")))

			out := &bytes.Buffer{}
			sink := sinkEle.(app.AppSink)
			sink.SetCallbacks(&app.AppSinkCallbacks{
				NewSample: WriterNewSample(ctx, out),
			})

			errCh := make(chan error, 1)
			go func() { errCh <- HandleBusMessages(ctx, pipeline) }()
			require.NotEqual(t, gst.StateChangeFailure, pipeline.SetState(gst.StatePaused))

			// Issue a byte-format seek into the desired range.
			ok := pipeline.SeekSimple(gst.FormatBytes, gst.SeekFlagFlush, offset)
			require.True(t, ok, "%s: seek to %d failed", name, offset)

			require.NotEqual(t, gst.StateChangeFailure, pipeline.SetState(gst.StatePlaying))
			require.NoError(t, <-errCh)
			require.Equal(t, gst.StateChangeSuccess, pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone))

			got := out.Bytes()
			require.GreaterOrEqual(t, len(got), int(length), "%s: expected at least %d bytes, got %d", name, length, len(got))

			wantH := sha256.Sum256(fixture[offset : offset+length])
			gotH := sha256.Sum256(got[:length])
			require.Equal(t, hex.EncodeToString(wantH[:]), hex.EncodeToString(gotH[:]), "%s: byte mismatch at offset %d len %d", name, offset, length)
		}

		// Tail slice
		check("tail", size-1024, 1024)
		// Middle slice
		check("middle", size/2, 1024)
		// Head slice
		check("head", 0, 1024)
	})
}
