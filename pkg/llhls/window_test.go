package llhls

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func observeEvent(t *testing.T, w *Window, event Event) {
	t.Helper()
	if err := w.Observe(event); err != nil {
		t.Fatal(err)
	}
}

func TestWindowPublishesPartsAndOnlyCompletesParentsWhenAllPartsArrive(t *testing.T) {
	w := NewWindow(WithMaxSegments(2), WithMaxBytes(100))

	if err := w.Observe(Event{Kind: Init, Presentation: "p1", Track: "v", Generation: 1, Data: []byte("init")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Observe(Event{Kind: Part, Presentation: "p1", Track: "v", Generation: 1, MSN: 7, Part: 0, Start: 0, Duration: time.Second, Data: []byte("a")}); err != nil {
		t.Fatal(err)
	}

	s := w.Snapshot("p1", "v")
	if len(s.Segments) != 1 || len(s.Segments[0].Parts) != 1 || s.Segments[0].Complete {
		t.Fatalf("part should be visible without completing parent: %+v", s)
	}
	if got := w.Data("p1", "v", 7, 0); !bytes.Equal(got, []byte("a")) {
		t.Fatalf("part data = %q", got)
	}

	if err := w.Observe(Event{Kind: SegmentComplete, Presentation: "p1", Track: "v", Generation: 1, MSN: 7, Start: 0, Duration: 2 * time.Second, Data: []byte("ab")}); err != nil {
		t.Fatal(err)
	}
	s = w.Snapshot("p1", "v")
	if !s.Segments[0].Complete || !bytes.Equal(w.SegmentData("p1", "v", 7), []byte("ab")) {
		t.Fatalf("parent was not completed: %+v", s.Segments[0])
	}
}

func TestWindowRejectsStalePresentationAndOutOfOrderParts(t *testing.T) {
	w := NewWindow()
	if err := w.Observe(Event{Kind: Init, Presentation: "new", Track: "v", Generation: 1, Data: []byte("init")}); err != nil {
		t.Fatal(err)
	}

	if err := w.Observe(Event{Kind: Part, Presentation: "old", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("old")}); err != ErrStalePresentation {
		t.Fatalf("stale presentation error = %v", err)
	}
	if err := w.Observe(Event{Kind: Part, Presentation: "new", Track: "v", Generation: 1, MSN: 1, Part: 1, Data: []byte("late")}); err != ErrPartOrder {
		t.Fatalf("out-of-order error = %v", err)
	}
}

func TestWindowEvictsBySegmentsAndBytesAndWakesWaiters(t *testing.T) {
	w := NewWindow(WithMaxSegments(2), WithMaxBytes(4))
	if err := w.Observe(Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	changed := w.Changed()
	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("aa")}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("publication did not wake waiter")
	}
	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 2, Part: 0, Data: []byte("bb")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 3, Part: 0, Data: []byte("cc")}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("publication did not wake waiter")
	}
	if got := w.Snapshot("p", "v").Segments; len(got) != 2 || got[0].MSN != 2 || got[1].MSN != 3 {
		t.Fatalf("segment eviction = %+v", got)
	}
	if w.Bytes() != 4 {
		t.Fatalf("byte bound = %d, want 4", w.Bytes())
	}
}

func TestWindowWaitHonorsCancellation(t *testing.T) {
	w := NewWindow()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.Wait(ctx, "p", "v", 1, 0); err != context.Canceled {
		t.Fatalf("Wait error = %v", err)
	}
}

func TestWindowWaitReturnsWhenRequestedPartIsPublished(t *testing.T) {
	w := NewWindow()
	if err := w.Observe(Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Wait(ctx, "p", "v", 4, 1) }()

	select {
	case err := <-done:
		t.Fatalf("Wait returned before requested part: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Data: []byte("a")}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatalf("Wait returned after only the first part: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 1, Data: []byte("b")}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Wait error = %v", err)
	}
}

func TestPlaylistContainsLLTagsAndOnlyCompleteParentURI(t *testing.T) {
	w := NewWindow()
	if err := w.Observe(Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1, Data: []byte("init")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Duration: 500 * time.Millisecond, Independent: true, Data: []byte("p")}); err != nil {
		t.Fatal(err)
	}
	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	for _, want := range []string{"#EXT-X-VERSION:10", "#EXT-X-PART-INF:", "#EXT-X-SERVER-CONTROL:", `URI="4/0.m4s"`, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="4/1.m4s"`, "INDEPENDENT=YES"} {
		if !strings.Contains(playlist, want) {
			t.Errorf("playlist missing %q:\n%s", want, playlist)
		}
	}
	if strings.Contains(playlist, "\n4.m4s\n") {
		t.Error("incomplete parent must not be published")
	}
}

func TestPlaylistIncludesProgramDateTimeForLiveSegments(t *testing.T) {
	w := NewWindow()
	programDateTime := time.Date(2026, time.August, 31, 22, 36, 12, 351000000, time.UTC)
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{
		Kind:            Part,
		Presentation:    "p",
		Track:           "v",
		Generation:      1,
		MSN:             4,
		Part:            0,
		Start:           2 * time.Second,
		Duration:        time.Second,
		ProgramDateTime: programDateTime,
		Data:            []byte("part"),
	})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Start: 2 * time.Second, Duration: time.Second, Data: []byte("segment")})

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-PROGRAM-DATE-TIME:2026-08-31T22:36:12.351Z") {
		t.Fatalf("playlist missing program date time:\n%s", playlist)
	}
}

func TestPlaylistOnlyPublishesPartsForOpenParent(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	for msn := uint64(1); msn <= 2; msn++ {
		observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: msn, Part: 0, Duration: time.Second, Data: []byte("part")})
		if msn == 1 {
			observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: msn, Duration: time.Second, Data: []byte("segment")})
		}
	}

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	if strings.Contains(playlist, `URI="1/0.m4s"`) {
		t.Fatalf("completed parent still has a part:\n%s", playlist)
	}
	if !strings.Contains(playlist, `URI="2/0.m4s"`) {
		t.Fatalf("open parent is missing its part:\n%s", playlist)
	}
}

func TestWindowResolvesPastParentPartToNextParent(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 5, Part: 0, Data: []byte("next")})

	if got := w.Data("p", "v", 4, 1); !bytes.Equal(got, []byte("next")) {
		t.Fatalf("past-parent part = %q, want next parent part %q", got, "next")
	}
}

func TestPlaylistRendersAfterInitBeforeFirstPart(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-MAP:URI=\"init.mp4\"") {
		t.Fatalf("playlist omitted init map:\n%s", playlist)
	}
}

func TestWindowStoresVideoConfig(t *testing.T) {
	w := NewWindow()
	w.SetVideoConfig(VideoConfig{Codec: "avc1.64002a", Width: 1280, Height: 720})

	if got := w.VideoConfig(); got != (VideoConfig{Codec: "avc1.64002a", Width: 1280, Height: 720}) {
		t.Fatalf("video config = %+v", got)
	}
}

func TestPlaylistTargetDurationScalesWithParentSegments(t *testing.T) {
	for _, duration := range []time.Duration{time.Second, 4 * time.Second, 8 * time.Second} {
		t.Run(duration.String(), func(t *testing.T) {
			w := NewWindow()
			observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
			observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})
			observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Duration: duration, Data: []byte("segment")})

			playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
			wantTarget := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", int(duration/time.Second))
			wantHoldBack := fmt.Sprintf("HOLD-BACK=%.6f", 3*duration.Seconds())
			if !strings.Contains(playlist, wantTarget) {
				t.Errorf("playlist missing %q:\n%s", wantTarget, playlist)
			}
			if !strings.Contains(playlist, wantHoldBack) {
				t.Errorf("playlist missing %q:\n%s", wantHoldBack, playlist)
			}
		})
	}
}

func TestPlaylistAdvertisesActualPartTarget(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Duration: 200 * time.Millisecond, Data: []byte("part")})

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=0.200000") {
		t.Fatalf("playlist did not advertise the actual part target:\n%s", playlist)
	}
}

func TestPlaylistUsesOneTargetDurationAcrossTracks(t *testing.T) {
	w := NewWindow()
	for _, track := range []string{"video", "audio"} {
		observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: track, Generation: 1})
		observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: track, Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})
	}
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Duration: 4 * time.Second, Data: []byte("video")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Duration: 8 * time.Second, Data: []byte("audio")})

	for _, track := range []string{"video", "audio"} {
		playlist := w.Playlist("p", track, func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
		if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:8") {
			t.Errorf("%s playlist did not use the window-wide target duration:\n%s", track, playlist)
		}
	}
}
