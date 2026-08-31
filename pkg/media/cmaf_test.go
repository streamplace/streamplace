package media

import (
	"bytes"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/llhls"
)

func TestIsCMAFInit(t *testing.T) {
	if !isCMAFInit(append([]byte{0, 0, 0, 8}, []byte("ftyp")...)) {
		t.Fatal("ftyp box was not recognized as an init")
	}
	if isCMAFInit(append([]byte{0, 0, 0, 8}, []byte("moof")...)) {
		t.Fatal("moof box was recognized as an init")
	}
	if isCMAFInit([]byte("short")) {
		t.Fatal("short data was recognized as an init")
	}
}

func TestCMAFMuxEmitsFragmentedBufferLists(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("cmafmux") == nil || gst.Find("x264enc") == nil {
		t.Skip("static GStreamer build with cmafmux and x264enc is required")
	}
	pipeline, err := gst.NewPipelineFromString("videotestsrc num-buffers=60 is-live=true ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency key-int-max=30 ! h264parse ! video/x-h264,stream-format=avc,alignment=au ! cmafmux fragment-duration=1000000000 chunk-duration=200000000 ! appsink name=sink sync=false")
	if err != nil {
		t.Fatal(err)
	}
	sinkElement, err := pipeline.GetElementByName("sink")
	if err != nil {
		t.Fatal(err)
	}
	sink := app.SinkFromElement(sinkElement)
	sink.SetBufferListSupport(true)
	state := &cmafTrackSink{presentation: "test", track: "video", window: llhls.NewWindow()}
	callbackErr := make(chan error, 1)
	lists := 0
	chunks := 0
	done := make(chan struct{})
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			list := sample.GetBufferList()
			if list == nil || list.Length() == 0 {
				t.Errorf("cmafmux returned a sample without a buffer list")
				return gst.FlowError
			}
			if lists < 2 {
				list.ForEach(func(buffer *gst.Buffer, i uint) bool {
					data := buffer.Bytes()
					box := ""
					if len(data) >= 8 {
						box = string(data[4:8])
					}
					duration := "unset"
					if d := buffer.Duration().AsDuration(); d != nil {
						duration = d.String()
					}
					t.Logf("cmaf list=%d buffer=%d box=%s size=%d pts=%s duration=%s", lists, i, box, len(data), buffer.PresentationTimestamp(), duration)
					return true
				})
			}
			if err := state.sample(sample); err != nil {
				callbackErr <- err
				return gst.FlowError
			}
			lists++
			chunks += int(list.Length())
			return gst.FlowOK
		},
		EOSFunc: func(*app.Sink) { close(done) },
	})
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for cmafmux EOS")
	}
	_ = pipeline.SetState(gst.StateNull)
	select {
	case err := <-callbackErr:
		t.Fatal(err)
	default:
	}
	if lists < 2 {
		t.Fatalf("expected initialization plus multiple CMAF fragments, got %d lists", lists)
	}
	if chunks <= lists {
		t.Fatalf("expected chunked CMAF output, got %d chunks across %d lists", chunks, lists)
	}
	snapshot := state.window.Snapshot("test", "video")
	if len(snapshot.Init) == 0 || len(snapshot.Segments) < 2 {
		t.Fatalf("expected CMAF init and completed segments, got init=%d segments=%d", len(snapshot.Init), len(snapshot.Segments))
	}
	if len(snapshot.Init) < 8 || !bytes.Equal(snapshot.Init[4:8], []byte("ftyp")) {
		t.Fatalf("stored init does not begin with ftyp: %q", snapshot.Init[:min(len(snapshot.Init), 8)])
	}
	for _, segment := range snapshot.Segments {
		if segment.Duration <= 0 {
			t.Fatalf("segment %d has invalid duration %s", segment.MSN, segment.Duration)
		}
		if len(segment.Parts) <= 1 {
			t.Fatalf("segment %d was not assembled from multiple CMAF chunks", segment.MSN)
		}
	}
}
