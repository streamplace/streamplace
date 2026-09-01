package llhls

import (
	"bytes"
	"context"
	"errors"
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

func TestWindowWaitForPartHonorsCancellation(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("part")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.WaitForPart(ctx, "p", "v", 1, 1); err != context.Canceled {
		t.Fatalf("WaitForPart error = %v", err)
	}
}

func TestWindowPreloadHintURIBecomesThePublishedPartURI(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("part-0")})
	partURI := func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }
	segmentURI := func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }
	playlist := w.Playlist("p", "v", partURI, segmentURI, "init.mp4")
	if !strings.Contains(playlist, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="1/1.m4s"`) {
		t.Fatalf("playlist omitted preload hint:\n%s", playlist)
	}
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 1, Data: []byte("part-1")})
	playlist = w.Playlist("p", "v", partURI, segmentURI, "init.mp4")
	if !strings.Contains(playlist, `#EXT-X-PART:DURATION=0.000000,URI="1/1.m4s"`) {
		t.Fatalf("published part did not retain hinted URI:\n%s", playlist)
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

func TestWindowPlaylistReloadMapsPastPartToFollowingParent(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Data: []byte("parent")})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Wait(ctx, "p", "v", 4, 1) }()
	select {
	case err := <-done:
		t.Fatalf("playlist reload resolved before rollover part: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 5, Part: 0, Data: []byte("next")})
	if err := <-done; err != nil {
		t.Fatalf("playlist reload error = %v", err)
	}
}

func TestWindowExactPartWaitReturnsUnavailableAfterParentClose(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Data: []byte("parent")})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.WaitForPart(ctx, "p", "v", 4, 1); !errors.Is(err, ErrPartUnavailable) {
		t.Fatalf("exact part wait error = %v, want ErrPartUnavailable", err)
	}
}

func TestWindowExactPartWaitDoesNotResolveFromFollowingParent(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 5, Part: 0, Data: []byte("next")})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.WaitForPart(ctx, "p", "v", 4, 1); !errors.Is(err, ErrPartUnavailable) {
		t.Fatalf("exact rollover part wait error = %v, want ErrPartUnavailable", err)
	}
	if got := w.Data("p", "v", 4, 1); got != nil {
		t.Fatalf("exact rollover part data = %q, want unavailable", got)
	}
}

func TestWindowExactPartWaitReturnsUnavailableAfterEviction(t *testing.T) {
	w := NewWindow(WithMaxSegments(1))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("old")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 2, Part: 0, Data: []byte("new")})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.WaitForPart(ctx, "p", "v", 1, 0); !errors.Is(err, ErrPartUnavailable) {
		t.Fatalf("evicted part wait error = %v, want ErrPartUnavailable", err)
	}
}

func TestWindowPartIdentitySurvivesParentTimingUpdate(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Start: 0, Duration: time.Second, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Start: 100 * time.Millisecond, Duration: 2*time.Second + 1400*time.Microsecond, Data: []byte("segment")})

	snapshot := w.Snapshot("p", "v")
	if len(snapshot.Segments) != 1 || len(snapshot.Segments[0].Parts) != 1 || snapshot.Segments[0].Parts[0].Index != 0 {
		t.Fatalf("part identity changed with parent timing: %+v", snapshot)
	}
}

func TestWindowIndependentSegmentsRequiresEveryAdvertisedParent(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Independent: true, Data: []byte("one")})
	if !w.IndependentSegments("p", "video") {
		t.Fatal("independent metadata should be true for one independent parent")
	}
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Part: 0, Independent: false, Data: []byte("two")})
	if w.IndependentSegments("p", "video") {
		t.Fatal("independent metadata should be false when an advertised parent is not independent")
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

func TestWindowDoesNotAliasPastParentPartToNextParent(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Data: []byte("parent")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 5, Part: 0, Data: []byte("next")})

	if got := w.Data("p", "v", 4, 1); got != nil {
		t.Fatalf("past-parent part = %q, want unavailable", got)
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

func TestPlaylistDurationsFreezeForPresentationAndRoundTargetDuration(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Duration: 2400 * time.Millisecond, Data: []byte("segment")})

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:2") {
		t.Fatalf("playlist did not use nearest-integer target duration:\n%s", playlist)
	}
	if !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=1.000000") {
		t.Fatalf("playlist did not freeze initial part target:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 2, Part: 0, Duration: 2 * time.Second, Data: []byte("later-part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 2, Duration: 8 * time.Second, Data: []byte("later-segment")})
	playlist = w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:2") || !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=1.000000") {
		t.Fatalf("playlist durations changed during presentation:\n%s", playlist)
	}
}

func TestPlaylistUsesConfiguredPartTargetWithoutGrowing(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Duration: 500 * time.Millisecond, Data: []byte("part")})

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=1.000000") {
		t.Fatalf("playlist did not preserve the presentation part target:\n%s", playlist)
	}
}

func TestPlaylistDurationContractRoundsAndResetsPerPresentation(t *testing.T) {
	w := NewWindow(WithPlaylistDurations(2500*time.Millisecond, 750*time.Millisecond))
	observeEvent(t, w, Event{Kind: Init, Presentation: "first", Track: "v", Generation: 1})
	playlist := w.Playlist("first", "v", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:3") || !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=0.750000") {
		t.Fatalf("playlist did not use configured rounded durations:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Init, Presentation: "second", Track: "v", Generation: 1})
	playlist = w.Playlist("second", "v", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4")
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:3") || !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=0.750000") {
		t.Fatalf("new presentation did not retain configured durations:\n%s", playlist)
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
		if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:2") {
			t.Errorf("%s playlist did not use the window-wide target duration:\n%s", track, playlist)
		}
	}
}
