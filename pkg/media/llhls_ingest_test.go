package media

import (
	"testing"
	"time"

	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/llhls"
)

func TestObserveWorkerLLHLSFramePopulatesWindow(t *testing.T) {
	const streamer = "did:key:z6MkWorkerLLHLSWindowTest"
	mm := &MediaManager{}
	window := mm.replaceLLWindow(streamer)

	frames := []struct {
		typ   ingestframe.Type
		event llhls.Event
	}{
		{typ: ingestframe.LLInit, event: llhls.Event{
			Kind:         llhls.Init,
			Presentation: "whip-1-test",
			Session:      1,
			Track:        "video",
			Generation:   1,
			Timescale:    90000,
			FrameRate:    120,
			VideoCodec:   "avc1.64002a",
			VideoWidth:   1280,
			VideoHeight:  720,
			Data:         []byte("video-init"),
		}},
		{typ: ingestframe.LLInit, event: llhls.Event{
			Kind:          llhls.Init,
			Presentation:  "whip-1-test",
			Session:       1,
			Track:         "audio",
			Generation:    1,
			Timescale:     48000,
			AudioChannels: 2,
			Data:          []byte("audio-init"),
		}},
		{typ: ingestframe.LLPart, event: llhls.Event{
			Kind:         llhls.Part,
			Presentation: "whip-1-test",
			Session:      1,
			Track:        "video",
			Generation:   1,
			Timescale:    90000,
			MSN:          0,
			Part:         0,
			Start:        0,
			Duration:     time.Second,
			Independent:  true,
			Data:         []byte("video-part"),
		}},
		{typ: ingestframe.LLSegmentComplete, event: llhls.Event{
			Kind:         llhls.SegmentComplete,
			Presentation: "whip-1-test",
			Session:      1,
			Track:        "video",
			Generation:   1,
			Timescale:    90000,
			MSN:          0,
			Start:        0,
			Duration:     time.Second,
			Data:         []byte("video-segment"),
		}},
	}

	for _, test := range frames {
		frame, err := llhlsEventToFrame(test.event)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := ingestframe.EncodeLLFrame(frame)
		if err != nil {
			t.Fatal(err)
		}
		if err := mm.observeWorkerLLHLSFrame(streamer, test.typ, payload); err != nil {
			t.Fatal(err)
		}
	}

	config := window.VideoConfig()
	if config.FrameRate != 120 {
		t.Fatalf("worker video frame rate = %v, want 120", config.FrameRate)
	}
	if config.Codec != "avc1.64002a" || config.Width != 1280 || config.Height != 720 {
		t.Fatalf("worker video metadata = %+v", config)
	}
	if got := window.AudioConfig().Channels; got != 2 {
		t.Fatalf("worker audio channels = %d, want 2", got)
	}
	snapshot := window.Snapshot("whip-1-test", "video")
	if len(snapshot.Init) == 0 || len(snapshot.Segments) != 1 || len(snapshot.Segments[0].Parts) != 1 {
		t.Fatalf("worker LL-HLS window = %+v", snapshot)
	}
}

func TestObserveWorkerLLHLSFrameRemovesWindowAtSessionEnd(t *testing.T) {
	const streamer = "did:key:z6MkWorkerLLHLSEndTest"
	mm := &MediaManager{}
	mm.replaceLLWindow(streamer)
	initEvent := llhls.Event{
		Kind:         llhls.Init,
		Presentation: "whip-1-test",
		Session:      1,
		Track:        "video",
		Generation:   1,
		VideoCodec:   "avc1.64002a",
		VideoWidth:   1280,
		VideoHeight:  720,
		Data:         []byte("video-init"),
	}
	frame, err := llhlsEventToFrame(initEvent)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := ingestframe.EncodeLLFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.observeWorkerLLHLSFrame(streamer, ingestframe.LLInit, payload); err != nil {
		t.Fatal(err)
	}
	endEvent := initEvent
	endEvent.Kind = llhls.SessionEnd
	frame, err = llhlsEventToFrame(endEvent)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = ingestframe.EncodeLLFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.observeWorkerLLHLSFrame(streamer, ingestframe.LLSessionEnd, payload); err != nil {
		t.Fatal(err)
	}
	if mm.GetLLWindow(streamer) != nil {
		t.Fatal("LL-HLS window survived session end")
	}
}

func TestDurationFromLLTicksRejectsOverflow(t *testing.T) {
	if _, err := durationFromLLTicks(^uint64(0), 1); err == nil {
		t.Fatal("overflowing LL-HLS tick value was accepted")
	}
	if _, err := durationFromLLTicks(^uint64(0), 0); err == nil {
		t.Fatal("overflowing unscaled LL-HLS tick value was accepted")
	}
}

func TestDurationToLLTicksAvoidsNanosecondMultiplicationOverflow(t *testing.T) {
	value := 3 * 24 * time.Hour
	got, err := durationToLLTicks(value, 90_000)
	if err != nil {
		t.Fatal(err)
	}
	want := uint64(value/time.Second) * 90_000
	if got != want {
		t.Fatalf("durationToLLTicks(%s, 90000) = %d, want %d", value, got, want)
	}
}

func TestDurationToLLTicksRejectsTickOverflow(t *testing.T) {
	if _, err := durationToLLTicks(time.Duration(1<<63-1), ^uint32(0)); err == nil {
		t.Fatal("durationToLLTicks accepted an overflowing tick value")
	}
}
