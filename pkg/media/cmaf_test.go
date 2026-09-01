package media

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
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
	playlist := state.window.Playlist("test", "video", func(msn uint64, part uint32) string {
		return fmt.Sprintf("%d/%d.m4s", msn, part)
	}, func(msn uint64) string {
		return fmt.Sprintf("%d.m4s", msn)
	}, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-PROGRAM-DATE-TIME:") {
		t.Fatalf("CMAF playlist is missing program date time:\n%s", playlist)
	}
}

func TestISOFMP4MuxCombinesAudioAndVideo(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil || gst.Find("x264enc") == nil || gst.Find("fdkaacenc") == nil {
		t.Skip("static GStreamer build with isofmp4mux, x264enc, and fdkaacenc is required")
	}
	pipeline, err := gst.NewPipelineFromString("isofmp4mux name=mux fragment-duration=2000000000 chunk-duration=1000000000 ! appsink name=sink sync=false\n" +
		"videotestsrc num-buffers=180 is-live=true ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency key-int-max=30 ! h264parse ! video/x-h264,stream-format=avc,alignment=au ! tee name=video_tee\n" +
		"video_tee. ! queue ! mux.\n" +
		"video_tee. ! queue ! fakesink sync=false\n" +
		"audiotestsrc num-buffers=300 is-live=true samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc bitrate=128000 ! aacparse ! audio/mpeg,mpegversion=4,stream-format=raw,rate=48000,channels=2 ! tee name=audio_tee\n" +
		"audio_tee. ! queue ! mux.\n" +
		"audio_tee. ! queue ! fakesink sync=false")
	if err != nil {
		t.Fatal(err)
	}
	sinkElement, err := pipeline.GetElementByName("sink")
	if err != nil {
		t.Fatal(err)
	}
	sink := app.SinkFromElement(sinkElement)
	sink.SetBufferListSupport(true)
	state := &cmafTrackSink{presentation: "test", track: "av", window: llhls.NewWindow()}
	callbackErr := make(chan error, 1)
	done := make(chan struct{})
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			if err := state.sample(sample); err != nil {
				callbackErr <- err
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
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for combined isofmp4mux EOS")
	}
	_ = pipeline.SetState(gst.StateNull)
	select {
	case err := <-callbackErr:
		t.Fatal(err)
	default:
	}

	snapshot := state.window.Snapshot("test", "av")
	if len(snapshot.Init) == 0 || !bytes.Contains(snapshot.Init, []byte("vide")) || !bytes.Contains(snapshot.Init, []byte("soun")) {
		t.Fatalf("combined init does not describe both tracks: %q", snapshot.Init)
	}
	if len(snapshot.Segments) < 2 {
		t.Fatalf("expected multiple combined segments, got %d", len(snapshot.Segments))
	}
	if len(snapshot.Segments[0].Parts) == 0 || snapshot.Segments[0].Parts[0].Start != 0 {
		t.Fatalf("combined HLS timeline should start at zero: %+v", snapshot.Segments[0].Parts)
	}
	for i, segment := range snapshot.Segments {
		if i == len(snapshot.Segments)-1 && bytes.Count(segment.Data, []byte("tfhd")) == 0 {
			continue
		}
		if len(segment.Parts) < 2 {
			t.Fatalf("segment %d was not assembled from multiple LL-HLS chunks", segment.MSN)
		}
		if bytes.Count(segment.Data, []byte("tfhd")) < 2 {
			t.Fatalf("segment %d does not contain both audio and video track fragments", segment.MSN)
		}
		for _, part := range segment.Parts {
			if bytes.Count(part.Data, []byte("tfhd")) < 2 {
				t.Fatalf("segment %d part %d does not contain both audio and video track fragments", segment.MSN, part.Index)
			}
		}
	}
	var previousEnd time.Duration
	havePart := false
	previousTiming := make(map[uint32]cmafFragmentTiming)
	for _, segment := range snapshot.Segments {
		for _, part := range segment.Parts {
			if havePart && part.Start != previousEnd {
				t.Fatalf("part timeline is not contiguous: got start=%s, want=%s", part.Start, previousEnd)
			}
			previousEnd = part.Start + part.Duration
			havePart = true
			timings, err := inspectCMAFFragment(part.Data)
			if err != nil {
				t.Fatalf("combined segment %d part %d has invalid CMAF timing: %v", segment.MSN, part.Index, err)
			}
			for _, timing := range timings {
				if previous, ok := previousTiming[timing.TrackID]; ok {
					expected := previous.DecodeTime + previous.Duration
					if timing.DecodeTime != expected {
						t.Fatalf("combined track %d decode timeline is not contiguous at segment %d part %d: got %d, want %d", timing.TrackID, segment.MSN, part.Index, timing.DecodeTime, expected)
					}
				}
				previousTiming[timing.TrackID] = timing
			}
		}
	}
}

func TestISOFMP4MuxProducesSeparateAudioAndVideoTracks(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil || gst.Find("x264enc") == nil || gst.Find("fdkaacenc") == nil {
		t.Skip("static GStreamer build with isofmp4mux, x264enc, and fdkaacenc is required")
	}
	pipeline, err := gst.NewPipelineFromString(
		"isofmp4mux name=video_mux fragment-duration=2000000000 chunk-duration=1000000000 ! appsink name=video_sink sync=false\n" +
			"videotestsrc num-buffers=180 is-live=true ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency key-int-max=60 ! h264parse ! video/x-h264,stream-format=avc,alignment=au ! tee name=video_tee\n" +
			"video_tee. ! queue ! video_mux.\n" +
			"video_tee. ! queue ! fakesink sync=false\n" +
			"isofmp4mux name=audio_mux fragment-duration=2000000000 chunk-duration=1000000000 ! appsink name=audio_sink sync=false\n" +
			"audiotestsrc num-buffers=600 is-live=true samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc bitrate=128000 ! aacparse ! audio/mpeg,mpegversion=4,stream-format=raw,rate=48000,channels=2 ! tee name=audio_tee\n" +
			"audio_tee. ! queue ! audio_mux.\n" +
			"audio_tee. ! queue ! fakesink sync=false",
	)
	if err != nil {
		t.Fatal(err)
	}
	videoSinkElement, err := pipeline.GetElementByName("video_sink")
	if err != nil {
		t.Fatal(err)
	}
	audioSinkElement, err := pipeline.GetElementByName("audio_sink")
	if err != nil {
		t.Fatal(err)
	}
	window := llhls.NewWindow()
	videoState := &cmafTrackSink{presentation: "test", track: "video", window: window, partDuration: time.Second}
	audioState := &cmafTrackSink{presentation: "test", track: "audio", window: window, partDuration: time.Second}
	callbackErr := make(chan error, 2)
	var done sync.WaitGroup
	done.Add(2)
	installTestCMAFSink := func(element *gst.Element, state *cmafTrackSink) {
		sink := app.SinkFromElement(element)
		sink.SetBufferListSupport(true)
		sink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
				sample := sink.PullSample()
				if sample == nil {
					return gst.FlowEOS
				}
				if err := state.sample(sample); err != nil {
					callbackErr <- err
					return gst.FlowError
				}
				return gst.FlowOK
			},
			EOSFunc: func(*app.Sink) { done.Done() },
		})
	}
	installTestCMAFSink(videoSinkElement, videoState)
	installTestCMAFSink(audioSinkElement, audioState)
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		t.Fatal(err)
	}
	doneWaiting := make(chan struct{})
	go func() {
		done.Wait()
		close(doneWaiting)
	}()
	select {
	case <-doneWaiting:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for separate isofmp4mux EOS")
	}
	_ = pipeline.SetState(gst.StateNull)
	select {
	case err := <-callbackErr:
		t.Fatal(err)
	default:
	}
	videoSnapshot := window.Snapshot("test", "video")
	audioSnapshot := window.Snapshot("test", "audio")
	if len(videoSnapshot.Init) == 0 || !bytes.Contains(videoSnapshot.Init, []byte("vide")) || bytes.Contains(videoSnapshot.Init, []byte("soun")) {
		t.Fatalf("video init is not video-only: %q", videoSnapshot.Init)
	}
	if len(audioSnapshot.Init) == 0 || !bytes.Contains(audioSnapshot.Init, []byte("soun")) || bytes.Contains(audioSnapshot.Init, []byte("vide")) {
		t.Fatalf("audio init is not audio-only: %q", audioSnapshot.Init)
	}
	for track, snapshot := range map[string]llhls.Snapshot{"video": videoSnapshot, "audio": audioSnapshot} {
		if len(snapshot.Segments) < 2 {
			t.Fatalf("expected multiple %s segments, got %d", track, len(snapshot.Segments))
		}
		if len(snapshot.Segments[0].Parts) < 2 || snapshot.Segments[0].Parts[0].Start != 0 {
			t.Fatalf("%s timeline is not chunked from zero: %+v", track, snapshot.Segments[0].Parts)
		}
		for _, segment := range snapshot.Segments {
			for _, part := range segment.Parts {
				if bytes.Count(part.Data, []byte("tfhd")) != 1 {
					t.Fatalf("%s segment %d part %d does not contain exactly one track fragment", track, segment.MSN, part.Index)
				}
			}
		}
		var previous cmafFragmentTiming
		havePrevious := false
		for _, segment := range snapshot.Segments {
			for _, part := range segment.Parts {
				timings, err := inspectCMAFFragment(part.Data)
				if err != nil {
					t.Fatalf("%s segment %d part %d has invalid CMAF timing: %v", track, segment.MSN, part.Index, err)
				}
				if len(timings) != 1 {
					t.Fatalf("%s segment %d part %d has %d track fragments, want one", track, segment.MSN, part.Index, len(timings))
				}
				if havePrevious {
					expected := previous.DecodeTime + previous.Duration
					if timings[0].DecodeTime != expected {
						t.Fatalf("%s CMAF decode timeline is not contiguous at segment %d part %d: got %d, want %d", track, segment.MSN, part.Index, timings[0].DecodeTime, expected)
					}
				}
				previous = timings[0]
				havePrevious = true
			}
		}
	}
}
