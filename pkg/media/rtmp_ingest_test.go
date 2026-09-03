package media

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/gstinit"
)

func TestH264VideoConfigUsesSPSMetadata(t *testing.T) {
	sps := []byte{
		0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50,
		0x05, 0xbb, 0x01, 0x6c, 0x80, 0x00, 0x00, 0x03,
		0x00, 0x80, 0x00, 0x00, 0x1e, 0x07, 0x8c, 0x18,
		0xcb,
	}
	sps[3] = 0x2a

	config := h264VideoConfig(&format.H264{SPS: sps})

	if config.Codec != "avc1.64002a" || config.Width != 1280 || config.Height != 720 || config.FrameRate <= 0 {
		t.Fatalf("video config = %+v", config)
	}
}

func TestFiniteH264FrameRateRejectsInvalidValues(t *testing.T) {
	for _, fps := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := finiteH264FrameRate(fps); got != 0 {
			t.Fatalf("finiteH264FrameRate(%v) = %v, want 0", fps, got)
		}
	}
	if got := finiteH264FrameRate(59.94); got != 59.94 {
		t.Fatalf("finiteH264FrameRate(59.94) = %v", got)
	}
}

func TestCMAFVideoFrameRateFromTiming(t *testing.T) {
	if got := cmafVideoFrameRate(cmafFragmentTiming{SampleCount: 60, Duration: 90000}, 90000); got != 60 {
		t.Fatalf("CMAF frame rate = %v, want 60", got)
	}
	for _, timing := range []cmafFragmentTiming{{}, {SampleCount: 60, Duration: 0}} {
		if got := cmafVideoFrameRate(timing, 90000); got != 0 {
			t.Fatalf("invalid CMAF timing frame rate = %v, want 0", got)
		}
	}
}

func TestLLAudioSplitUsesGstClockTimeSignalType(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil {
		t.Skip("static GStreamer build with isofmp4mux is required")
	}

	pipeline, err := gst.NewPipelineFromString("isofmp4mux name=mux ! fakesink")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pipeline.SetState(gst.StateNull); err != nil {
			t.Logf("set pipeline to NULL: %v", err)
		}
	}()
	mux, err := pipeline.GetElementByName("mux")
	if err != nil {
		t.Fatal(err)
	}

	if err := emitLLAudioSplit(mux, gst.ClockTime(2*time.Second)); err != nil {
		t.Fatalf("audio split signal: %v", err)
	}
}

func TestLLAudioSplitterFollowsMediaTimeline(t *testing.T) {
	gstinit.InitGST()

	pipeline, err := gst.NewPipelineFromString("appsrc name=src is-live=true format=time caps=audio/x-raw,format=S16LE,layout=interleaved,rate=48000,channels=1 ! queue name=ll_audio_queue max-size-time=0 ! fakesink name=ll_audio_mux sync=false async=false")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pipeline.SetState(gst.StateNull); err != nil {
			t.Logf("set pipeline to NULL: %v", err)
		}
	}()

	splitEvents := make(chan bool, 3)
	mux, err := pipeline.GetElementByName("ll_audio_mux")
	if err != nil {
		t.Fatal(err)
	}
	muxSink := mux.GetStaticPad("sink")
	if muxSink == nil {
		t.Fatal("LL-HLS audio mux test sink pad is missing")
	}
	muxSink.AddProbe(gst.PadProbeTypeEventDownstream, func(_ *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		event := info.GetEvent()
		if event == nil || event.Type() != gst.EventTypeCustomDownstream || !event.HasName("FMP4MuxSplitNow") {
			return gst.PadProbeOK
		}
		value, err := event.GetStructure().GetValue("chunk")
		if err != nil {
			t.Errorf("read audio split event: %v", err)
			return gst.PadProbeOK
		}
		chunk, ok := value.(bool)
		if !ok {
			t.Errorf("audio split event chunk = %T(%v), want bool", value, value)
			return gst.PadProbeOK
		}
		splitEvents <- chunk
		return gst.PadProbeOK
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := startLLAudioSplitter(ctx, pipeline); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		t.Fatal(err)
	}

	srcElement, err := pipeline.GetElementByName("src")
	if err != nil {
		t.Fatal(err)
	}
	src := app.SrcFromElement(srcElement)
	if src == nil {
		t.Fatal("source element is not appsrc")
	}
	for i := 0; i <= 6; i++ {
		buffer := gst.NewBufferWithSize(1)
		buffer.SetPresentationTimestamp(gst.ClockTime(time.Duration(i) * 500 * time.Millisecond))
		buffer.SetDuration(gst.ClockTime(500 * time.Millisecond))
		if result := src.PushBuffer(buffer); result != gst.FlowOK {
			t.Fatalf("push audio buffer %d: %s", i, result)
		}
	}

	want := []bool{true, false, true}
	for i, expected := range want {
		select {
		case got := <-splitEvents:
			if got != expected {
				t.Fatalf("split event %d chunk = %v, want %v", i, got, expected)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for split event %d", i)
		}
	}
}
