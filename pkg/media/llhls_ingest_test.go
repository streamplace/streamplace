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
		frame := llhlsEventToFrame(test.event)
		var err error
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
		Data:         []byte("video-init"),
	}
	frame := llhlsEventToFrame(initEvent)
	var err error
	payload, err := ingestframe.EncodeLLFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.observeWorkerLLHLSFrame(streamer, ingestframe.LLInit, payload); err != nil {
		t.Fatal(err)
	}
	endEvent := initEvent
	endEvent.Kind = llhls.SessionEnd
	frame = llhlsEventToFrame(endEvent)
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
