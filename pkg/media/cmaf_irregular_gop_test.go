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

func TestISOFMP4MuxIrregularGOPWithAudio(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil || gst.Find("x264enc") == nil || gst.Find("fdkaacenc") == nil {
		t.Skip("static GStreamer build with isofmp4mux, x264enc, and fdkaacenc is required")
	}

	// A 47-frame GOP is about 1.57 seconds at 30 fps, so it divides neither the
	// 2-second parent nor the 1-second part. Disabling force-keyunit preserves
	// that irregular cadence and makes the muxer handle GOPs crossing fragment
	// boundaries.
	pipeline, err := gst.NewPipelineFromString(
		"isofmp4mux name=mux fragment-duration=2000000000 chunk-duration=1000000000 send-force-keyunit=false ! appsink name=sink sync=false\n" + "videotestsrc num-buffers=300 is-live=true pattern=ball ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency speed-preset=ultrafast bframes=0 key-int-max=47 ! h264parse ! video/x-h264,stream-format=avc,alignment=au ! queue ! mux.\n" + "audiotestsrc num-buffers=480 is-live=true samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc bitrate=128000 ! aacparse ! audio/mpeg,mpegversion=4,stream-format=raw,rate=48000,channels=2 ! queue ! mux.",
	)
	if err != nil {
		t.Fatal(err)
	}
	sinkElement, err := pipeline.GetElementByName("sink")
	if err != nil {
		t.Fatal(err)
	}

	sink := app.SinkFromElement(sinkElement)
	sink.SetBufferListSupport(true)
	state := &cmafTrackSink{
		presentation: "test",
		track:        "av",
		window:       llhls.NewWindow(),
		partDuration: time.Second,
	}
	callbackErr := make(chan error, 1)
	done := make(chan struct{})
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			if err := state.sample(sample); err != nil {
				select {
				case callbackErr <- err:
				default:
				}
				return gst.FlowError
			}
			return gst.FlowOK
		},
		EOSFunc: func(*app.Sink) { close(done) },
	})

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = pipeline.SetState(gst.StateNull)
		t.Fatal("timed out waiting for irregular-GOP isofmp4mux EOS")
	}
	_ = pipeline.SetState(gst.StateNull)
	select {
	case err := <-callbackErr:
		t.Fatal(err)
	default:
	}

	snapshot := state.window.Snapshot("test", "av")
	if len(snapshot.Init) == 0 || !bytes.Contains(snapshot.Init, []byte("vide")) || !bytes.Contains(snapshot.Init, []byte("soun")) {
		t.Fatalf("muxed init does not describe both tracks: %q", snapshot.Init)
	}
	if len(snapshot.Segments) < 2 {
		t.Fatalf("expected at least two completed parent segments, got %d", len(snapshot.Segments))
	}

	var previousEnd time.Duration
	havePart := false
	previousTiming := make(map[uint32]cmafFragmentTiming)
	const maxSingleTrackTail = 50 * time.Millisecond
	singleTrackTailDuration := time.Duration(0)
	completedSegments := 0
	for _, segment := range snapshot.Segments {
		if segment.Complete {
			completedSegments++
			if segment.Duration <= 0 || len(segment.Data) == 0 {
				t.Fatalf("parent segment %d has no completed CMAF data: duration=%s data=%d", segment.MSN, segment.Duration, len(segment.Data))
			}
			if len(segment.Parts) < 2 {
				t.Fatalf("parent segment %d has %d parts, want multiple parts", segment.MSN, len(segment.Parts))
			}
			parentTimings, err := inspectCMAFFragment(segment.Data)
			if err != nil {
				t.Fatalf("parent segment %d is not parseable CMAF: %v", segment.MSN, err)
			}
			if len(parentTimings) < 2 {
				t.Fatalf("parent segment %d does not contain both track fragments", segment.MSN)
			}
		}

		for _, part := range segment.Parts {
			if part.Duration <= 0 || len(part.Data) == 0 {
				t.Fatalf("parent segment %d part %d is empty or has invalid duration %s", segment.MSN, part.Index, part.Duration)
			}
			if part.Duration > 1100*time.Millisecond {
				t.Fatalf("parent segment %d part %d exceeds the configured 1s chunk duration: %s", segment.MSN, part.Index, part.Duration)
			}
			partTimings, err := inspectCMAFFragment(part.Data)
			if err != nil {
				t.Fatalf("parent segment %d part %d is not parseable CMAF: %v", segment.MSN, part.Index, err)
			}
			switch len(partTimings) {
			case 1:
				singleTrackTailDuration += part.Duration
				if part.Duration > maxSingleTrackTail || singleTrackTailDuration > maxSingleTrackTail {
					t.Fatalf("parent segment %d part %d has a meaningful single-track tail: duration=%s consecutive=%s", segment.MSN, part.Index, part.Duration, singleTrackTailDuration)
				}
			case 2:
				singleTrackTailDuration = 0
			default:
				t.Fatalf("parent segment %d part %d has %d track fragments, want audio and video", segment.MSN, part.Index, len(partTimings))
			}
			if havePart && part.Start != previousEnd {
				t.Fatalf("part timeline is not contiguous: parent=%d part=%d got start=%s, want=%s", segment.MSN, part.Index, part.Start, previousEnd)
			}
			previousEnd = part.Start + part.Duration
			havePart = true

			boxTypes := make(map[string]bool)
			if err := walkCMAFBoxes(part.Data, func(boxType string, _ []byte) error {
				boxTypes[boxType] = true
				return nil
			}); err != nil {
				t.Fatalf("parent segment %d part %d has invalid CMAF boxes: %v", segment.MSN, part.Index, err)
			}
			if !boxTypes["moof"] || !boxTypes["mdat"] {
				t.Fatalf("parent segment %d part %d is missing moof or mdat: %v", segment.MSN, part.Index, boxTypes)
			}

			seenTracks := make(map[uint32]bool)
			for _, timing := range partTimings {
				if seenTracks[timing.TrackID] {
					t.Fatalf("parent segment %d part %d repeats track id %d", segment.MSN, part.Index, timing.TrackID)
				}
				seenTracks[timing.TrackID] = true
				if previous, ok := previousTiming[timing.TrackID]; ok {
					expected := previous.DecodeTime + previous.Duration
					if timing.DecodeTime != expected {
						t.Fatalf("track %d decode timeline is not contiguous at parent %d part %d: got %d, want %d", timing.TrackID, segment.MSN, part.Index, timing.DecodeTime, expected)
					}
				}
				previousTiming[timing.TrackID] = timing
			}
		}
	}
	if completedSegments < 2 {
		t.Fatalf("expected at least two completed parent segments, got %d", completedSegments)
	}
	if len(previousTiming) != 2 {
		t.Fatalf("expected exactly two contiguous decode timelines, got %d: %v", len(previousTiming), previousTiming)
	}
}
