package media

import (
	"bytes"
	"context"
	"math"
	"os"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
)

// vtSPS is a real VideoToolbox SPS (High, level 5.1, 2560x1440) — crucially
// with NO VUI parameters, which is what sends h264timestamper down its
// full-DPB reorder fallback.
var vtSPS = []byte{0x27, 0x64, 0x00, 0x33, 0xac, 0x56, 0x80, 0x28, 0x00, 0xb5, 0xa6, 0xa0, 0x20, 0x20, 0x20, 0x40}

// makeNoVUIH264 encodes 60fps H264 and replaces the encoder's SPS with the
// no-VUI VideoToolbox SPS, so the stream signals nothing about reordering
// (exactly what VT sends). Slices ride along unmodified; the timestamp path
// is all this test exercises.
func makeNoVUIH264(t *testing.T, frames int) []byte {
	t.Helper()
	gstinit.InitGST()
	ctx := context.Background()
	pipeline, err := gst.NewPipelineFromString(
		"videotestsrc num-buffers=60 ! video/x-raw,width=2560,height=1440,framerate=60/1 ! x264enc tune=zerolatency key-int-max=60 ! video/x-h264,stream-format=byte-stream ! appsink name=sink sync=false")
	require.NoError(t, err)
	sinkEle, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	var out bytes.Buffer
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &out),
	})
	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	require.NoError(t, <-busErr)
	// splice the VT SPS in place of every SPS NAL (annex-b, type 7) — x264
	// can repeat SPS at mid-stream IDRs
	buf := out.Bytes()
	var spliced bytes.Buffer
	i := 0
	replaced := 0
	for i+4 < len(buf) {
		var hdr int
		if bytes.Equal(buf[i:i+4], []byte{0, 0, 0, 1}) {
			hdr = 4
		} else if bytes.Equal(buf[i:i+3], []byte{0, 0, 1}) {
			hdr = 3
		} else {
			spliced.WriteByte(buf[i])
			i++
			continue
		}
		// find this NAL's end
		end := len(buf)
		for j := i + hdr; j+3 < len(buf); j++ {
			if bytes.Equal(buf[j:j+3], []byte{0, 0, 1}) {
				end = j
				break
			}
		}
		spliced.Write(buf[i : i+hdr])
		if buf[i+hdr]&0x1f == 7 {
			spliced.Write(vtSPS)
			replaced++
		} else {
			spliced.Write(buf[i+hdr : end])
		}
		i = end
	}
	require.Greater(t, replaced, 0, "no SPS NAL found in x264 output")
	if dump := os.Getenv("SPLICE_DUMP"); dump != "" {
		_ = os.WriteFile(dump, spliced.Bytes(), 0o644)
	}
	return spliced.Bytes()
}

func timestamperDeltas(t *testing.T, bytestream []byte, extraProps string) []float64 {
	t.Helper()
	desc := "appsrc name=src caps=video/x-h264,stream-format=byte-stream,framerate=60/1 ! h264parse ! h264timestamper " + extraProps + " name=ts ! appsink name=vsink sync=false"
	pipeline, err := gst.NewPipelineFromString(desc)
	require.NoError(t, err)
	ctx := context.Background()
	srcEle, err := pipeline.GetElementByName("src")
	require.NoError(t, err)
	src := app.SrcFromElement(srcEle)
	go func() {
		buf := gst.NewBufferWithSize(int64(len(bytestream)))
		buf.Map(gst.MapWrite).WriteData(bytestream)
		buf.Unmap()
		buf.SetPresentationTimestamp(0)
		src.PushBuffer(buf)
		src.EndStream()
	}()
	sinkEle, err := pipeline.GetElementByName("vsink")
	require.NoError(t, err)
	var deltas []float64
	var noneCount, sampleCount int
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			buf := sample.GetBuffer()
			pts, dts := buf.PresentationTimestamp(), buf.DecodingTimestamp()
			sampleCount++
			if pts != gst.ClockTimeNone && dts != gst.ClockTimeNone {
				deltas = append(deltas, float64(int64(pts)-int64(dts))/1e9)
			} else {
				noneCount++
				if noneCount <= 3 {
					t.Logf("sample %d: pts=%v dts=%v", sampleCount, pts, dts)
				}
			}
			return gst.FlowOK
		},
	})
	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	require.NoError(t, <-busErr)
	return deltas
}

// h264timestamper's SPS fallback invents a full-DPB reorder delay for streams
// without VUI parameters (VideoToolbox), even when the stream never reorders.
// max-reorder-frames=0 must pin DTS = PTS for those streams.
func TestH264TimestamperMaxReorderFramesOverride(t *testing.T) {
	bytestream := makeNoVUIH264(t, 60)

	auto := timestamperDeltas(t, bytestream, "")
	require.NotEmpty(t, auto)
	autoMax := 0.0
	for _, d := range auto {
		autoMax = math.Max(autoMax, d)
	}
	require.Greater(t, autoMax, 0.1, "auto mode: no-VUI stream gets a spurious reorder delay")

	forced := timestamperDeltas(t, bytestream, "max-reorder-frames=0")
	require.NotEmpty(t, forced)
	for i, d := range forced {
		require.Equal(t, 0.0, d, "frame %d: forced no-reorder must give DTS = PTS", i)
	}
}
