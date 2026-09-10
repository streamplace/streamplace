package media

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/llhls"
)

func TestISOFMP4MuxDoesNotPublishShortNonTerminalParts(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil || gst.Find("x264enc") == nil || gst.Find("fdkaacenc") == nil {
		t.Skip("static GStreamer build with isofmp4mux, x264enc, and fdkaacenc is required")
	}

	pipeline, err := gst.NewPipelineFromString(
		"isofmp4mux name=mux fragment-duration=2000000000 chunk-duration=1000000000 send-force-keyunit=false ! appsink name=sink sync=false\n" +
			"videotestsrc num-buffers=300 is-live=true pattern=ball ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency speed-preset=ultrafast bframes=0 key-int-max=30 ! h264parse ! video/x-h264,stream-format=avc,alignment=au ! queue ! mux.\n" +
			"audiotestsrc num-buffers=480 is-live=true samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc bitrate=128000 ! aacparse ! audio/mpeg,mpegversion=4,stream-format=raw,rate=48000,channels=2 ! queue ! mux.",
	)
	if err != nil {
		t.Fatal(err)
	}
	sinkElement, err := pipeline.GetElementByName("sink")
	if err != nil {
		t.Fatal(err)
	}

	state := &cmafTrackSink{
		presentation: "test",
		track:        "video",
		window:       llhls.NewWindow(),
		partDuration: time.Second,
		partTarget:   1100 * time.Millisecond,
	}
	sink := app.SinkFromElement(sinkElement)
	sink.SetBufferListSupport(true)
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

	snapshot := state.window.Snapshot("test", "video")
	if len(snapshot.Segments) < 2 {
		t.Fatalf("expected multiple parent segments, got %d", len(snapshot.Segments))
	}

	const partTarget = 1100 * time.Millisecond
	minimumNonTerminal := partTarget * 85 / 100
	var previous []byte
	for _, segment := range snapshot.Segments {
		for i, part := range segment.Parts {
			if part.Duration > partTarget {
				t.Fatalf("parent segment %d part %d exceeds PART-TARGET: %s > %s", segment.MSN, part.Index, part.Duration, partTarget)
			}
			if err := walkCMAFBoxes(part.Data, func(string, []byte) error { return nil }); err != nil {
				t.Fatalf("parent segment %d part %d is not parseable CMAF: %v", segment.MSN, part.Index, err)
			}
			if i+1 < len(segment.Parts) && part.Duration < minimumNonTerminal {
				t.Fatalf("parent segment %d part %d is short but non-terminal: %s, minimum %s", segment.MSN, part.Index, part.Duration, minimumNonTerminal)
			}
			if len(part.Data) == 0 {
				t.Fatalf("parent segment %d part %d has no CMAF bytes", segment.MSN, part.Index)
			}
			if previous != nil && bytes.Equal(previous, part.Data) {
				t.Fatalf("parent segment %d part %d aliases the preceding part bytes", segment.MSN, part.Index)
			}
			previous = append(previous[:0], part.Data...)
		}
	}
}

func TestCMAFPartCoalescingKeepsNonIndependentVideoPrefix(t *testing.T) {
	const (
		videoTrackID      = 2
		partTarget        = 1100 * time.Millisecond
		minimumPartLength = partTarget * 85 / 100
	)
	prefix := cmafPartDurationTestFragment(videoTrackID, cmafTestNonSyncSampleFlags)
	key := cmafPartDurationTestFragment(videoTrackID, cmafTestSyncSampleFlags)
	following := cmafPartDurationTestFragment(videoTrackID, cmafTestSyncSampleFlags)
	wantMerged := append(append([]byte(nil), prefix...), key...)

	state := &cmafTrackSink{
		presentation:  "test",
		track:         "video",
		window:        llhls.NewWindow(),
		generation:    1,
		partDuration:  time.Second,
		partTarget:    partTarget,
		videoTrackIDs: map[uint32]bool{videoTrackID: true},
		hasParent:     true,
	}
	if err := state.window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "test", Track: "video", Generation: 1, Data: []byte("init")}); err != nil {
		t.Fatal(err)
	}
	state.parent.Write(prefix)
	state.parent.Write(key)
	state.parent.Write(following)
	state.parentLength = 2 * time.Second

	if err := state.queuePart(cmafPendingPart{
		data:        prefix,
		start:       0,
		duration:    34 * time.Millisecond,
		hasVideo:    true,
		independent: false,
		set:         true,
	}); err != nil {
		t.Fatal(err)
	}
	if got := state.window.Snapshot("test", "video"); len(got.Segments) != 0 {
		t.Fatalf("short prefix was published before its successor: %+v", got.Segments)
	}
	if err := state.queuePart(cmafPendingPart{
		data:        key,
		start:       34 * time.Millisecond,
		duration:    time.Second,
		hasVideo:    true,
		independent: true,
		set:         true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := state.queuePart(cmafPendingPart{
		data:        following,
		start:       1034 * time.Millisecond,
		duration:    966 * time.Millisecond,
		hasVideo:    true,
		independent: true,
		set:         true,
	}); err != nil {
		t.Fatal(err)
	}

	snapshot := state.window.Snapshot("test", "video")
	if len(snapshot.Segments) != 1 || len(snapshot.Segments[0].Parts) != 1 {
		t.Fatalf("expected only the merged part to be published, got %+v", snapshot.Segments)
	}
	part := snapshot.Segments[0].Parts[0]
	if part.Duration < minimumPartLength || part.Duration > partTarget {
		t.Fatalf("merged part duration = %s, want [%s, %s]", part.Duration, minimumPartLength, partTarget)
	}
	if part.Independent {
		t.Fatal("merged part inherited independence from a later keyframe despite its video prefix")
	}
	if !bytes.Equal(part.Data, wantMerged) {
		t.Fatal("merged part bytes do not preserve the original CMAF fragments")
	}
	if _, err := inspectCMAFFragment(part.Data); err != nil {
		t.Fatalf("merged part is not parseable CMAF: %v", err)
	}

	prefix[0] ^= 0xff
	key[0] ^= 0xff
	if !bytes.Equal(snapshot.Segments[0].Parts[0].Data, wantMerged) {
		t.Fatal("published merged part changed after source buffers were mutated")
	}
}

func TestCMAFCompleteParentPublishesPendingPart(t *testing.T) {
	state := &cmafTrackSink{
		presentation: "test",
		track:        "video",
		window:       llhls.NewWindow(),
		generation:   1,
		partDuration: time.Second,
		partTarget:   1100 * time.Millisecond,
		hasParent:    true,
		parentLength: time.Second,
		pendingPart: cmafPendingPart{
			data:     []byte("pending"),
			start:    0,
			duration: time.Second,
			set:      true,
		},
	}
	if err := state.window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "test", Track: "video", Generation: 1, Data: []byte("init")}); err != nil {
		t.Fatal(err)
	}
	state.parent.WriteString("pending")

	if err := state.completeParent(); err != nil {
		t.Fatal(err)
	}
	snapshot := state.window.Snapshot("test", "video")
	if len(snapshot.Segments) != 1 || !snapshot.Segments[0].Complete || len(snapshot.Segments[0].Parts) != 1 {
		t.Fatalf("EOS flush did not publish the pending parent: %+v", snapshot.Segments)
	}
	if !bytes.Equal(snapshot.Segments[0].Parts[0].Data, []byte("pending")) {
		t.Fatalf("flushed part data = %q", snapshot.Segments[0].Parts[0].Data)
	}
}

func TestISOFMP4AudioSplitterClosesParents(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil || gst.Find("fdkaacenc") == nil {
		t.Skip("static GStreamer build with isofmp4mux and fdkaacenc is required")
	}

	pipeline, err := gst.NewPipelineFromString(
		"isofmp4mux name=ll_audio_mux manual-split=true fragment-duration=2000000000 chunk-duration=1000000000 ! appsink name=ll_audio_sink sync=false async=false\n" +
			"audiotestsrc num-buffers=600 is-live=true timestamp-offset=0 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc bitrate=128000 ! aacparse ! audio/mpeg,mpegversion=4,stream-format=raw,rate=48000,channels=2 ! queue name=ll_audio_queue max-size-time=0 ! ll_audio_mux.",
	)
	if err != nil {
		t.Fatal(err)
	}
	queueElement, err := pipeline.GetElementByName("ll_audio_queue")
	if err != nil {
		t.Fatal(err)
	}
	maxSizeTime, err := queueElement.GObject().GetProperty("max-size-time")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := maxSizeTime.(uint64); !ok || got != 0 {
		t.Fatalf("manual-split audio queue max-size-time = %v, want 0", maxSizeTime)
	}
	sinkElement, err := pipeline.GetElementByName("ll_audio_sink")
	if err != nil {
		t.Fatal(err)
	}
	async, err := sinkElement.GObject().GetProperty("async")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := async.(bool); !ok || got {
		t.Fatalf("CMAF appsink async = %v, want false", async)
	}

	state := &cmafTrackSink{
		presentation: "test",
		track:        "audio",
		window:       llhls.NewWindow(),
		partDuration: time.Second,
		partTarget:   1100 * time.Millisecond,
		audioOnly:    true,
	}
	sink := app.SinkFromElement(sinkElement)
	sink.SetBufferListSupport(true)
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
		EOSFunc: func(*app.Sink) {
			if err := state.completeParent(); err != nil {
				select {
				case callbackErr <- err:
				default:
				}
			}
			close(done)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		t.Fatal(err)
	}
	if err := startLLAudioSplitter(ctx, pipeline); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = pipeline.SetState(gst.StateNull)
		t.Fatal("timed out waiting for audio splitter isofmp4mux EOS")
	}
	_ = pipeline.SetState(gst.StateNull)
	select {
	case err := <-callbackErr:
		t.Fatal(err)
	default:
	}

	snapshot := state.window.Snapshot("test", "audio")
	if len(snapshot.Segments) < 4 {
		t.Fatalf("audio splitter produced %d parent segments, want at least 4", len(snapshot.Segments))
	}
	var have93, have94 bool
	for i, segment := range snapshot.Segments {
		if i+1 < len(snapshot.Segments) && segment.Duration < 1900*time.Millisecond {
			t.Fatalf("audio parent %d is unexpectedly short: %s", segment.MSN, segment.Duration)
		}
		if i+1 < len(snapshot.Segments) && segment.Duration > 2100*time.Millisecond {
			partDurations := make([]time.Duration, 0, len(segment.Parts))
			for _, part := range segment.Parts {
				partDurations = append(partDurations, part.Duration)
			}
			t.Fatalf("audio parent %d is unexpectedly long: %s parts=%v", segment.MSN, segment.Duration, partDurations)
		}
		if len(segment.Parts) == 0 {
			t.Fatalf("audio parent %d has no parts", segment.MSN)
		}
		var samples uint32
		for _, part := range segment.Parts {
			timings, err := inspectCMAFFragment(part.Data)
			if err != nil {
				t.Fatalf("audio parent %d part %d has invalid timing: %v", segment.MSN, part.Index, err)
			}
			for _, timing := range timings {
				samples += timing.SampleCount
			}
		}
		if i > 0 && i+1 < len(snapshot.Segments) && samples != 93 && samples != 94 {
			t.Fatalf("audio parent %d has %d AAC samples, want a frame-aligned 93/94 pattern", segment.MSN, samples)
		}
		if i > 0 && i+1 < len(snapshot.Segments) {
			have93 = have93 || samples == 93
			have94 = have94 || samples == 94
		}
		for i, part := range segment.Parts {
			if i+1 < len(segment.Parts) && part.Duration < 935*time.Millisecond {
				t.Fatalf("audio parent %d part %d is short but non-terminal: %s", segment.MSN, part.Index, part.Duration)
			}
		}
	}
	if !have93 || !have94 {
		t.Fatalf("audio parents did not alternate AAC frame counts: have93=%v have94=%v", have93, have94)
	}
}

func TestCMAFAudioPartCoalescingKeepsShortPrefixNonTerminal(t *testing.T) {
	state := &cmafTrackSink{
		presentation: "test",
		track:        "audio",
		window:       llhls.NewWindow(),
		generation:   1,
		partDuration: time.Second,
		partTarget:   1100 * time.Millisecond,
		audioOnly:    true,
	}
	if err := state.window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "test", Track: "audio", Generation: 1, Data: []byte("init")}); err != nil {
		t.Fatal(err)
	}

	for _, part := range []cmafPendingPart{
		{data: []byte("prefix"), duration: 21 * time.Millisecond, set: true},
		{data: []byte("body"), duration: time.Second, set: true},
		{data: []byte("following"), duration: time.Second, set: true},
	} {
		if err := state.queuePart(part); err != nil {
			t.Fatal(err)
		}
	}

	snapshot := state.window.Snapshot("test", "audio")
	if len(snapshot.Segments) != 1 || len(snapshot.Segments[0].Parts) != 1 {
		t.Fatalf("audio short prefix was published separately: %+v", snapshot.Segments)
	}
	if got := snapshot.Segments[0].Parts[0].Duration; got != 1021*time.Millisecond {
		t.Fatalf("coalesced audio part duration = %s, want 1.021s", got)
	}
}

func cmafPartDurationTestFragment(trackID, flags uint32) []byte {
	traf := cmafTestIndependenceTraf(trackID, flags, nil, nil)
	return append(cmafTestBox("moof", traf), cmafTestBox("mdat", []byte{1, 2, 3, 4})...)
}
