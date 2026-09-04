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

func completionHoldURI(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }
func completionHoldSegment(msn uint64) string          { return fmt.Sprintf("%d.m4s", msn) }

func waitForSegmentComplete(t *testing.T, w *Window, presentation, track string, msn uint64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, seg := range w.Snapshot(presentation, track).Segments {
			if seg.MSN == msn && seg.Complete {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("segment %d did not complete after hold", msn)
}

func TestWindowCompletionHoldKeepsFinalPartListed(t *testing.T) {
	w := NewWindow(WithSegmentCompletionDelay(50 * time.Millisecond))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 3, Part: 0, Start: 0, Duration: time.Second, Data: []byte("a")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 3, Part: 1, Start: time.Second, Duration: time.Second, Data: []byte("b")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 3, Start: 0, Duration: 2 * time.Second, Data: []byte("ab")})

	// During the hold the parent still reads as open: the final part stays
	// listed and fetchable so a blocking reload for it resolves with the part
	// instead of rolling over to the completed segment.
	s := w.Snapshot("p", "v")
	if len(s.Segments) != 1 || s.Segments[0].Complete {
		t.Fatalf("segment completed during hold: %+v", s.Segments)
	}
	playlist := w.Playlist("p", "v", completionHoldURI, completionHoldSegment, "init.mp4", nil)
	if !strings.Contains(playlist, "3/1.m4s") {
		t.Fatalf("final part not listed during hold:\n%s", playlist)
	}
	if strings.Contains(playlist, "#EXTINF") {
		t.Fatalf("segment listed as complete during hold:\n%s", playlist)
	}
	if err := w.Wait(context.Background(), "p", "v", 3, 1); err != nil {
		t.Fatalf("blocking reload for held part: %v", err)
	}
	if err := w.WaitForPart(context.Background(), "p", "v", 3, 1); err != nil {
		t.Fatalf("held part not fetchable: %v", err)
	}
	if got := w.Data("p", "v", 3, 1); !bytes.Equal(got, []byte("b")) {
		t.Fatalf("held part data = %q", got)
	}

	waitForSegmentComplete(t, w, "p", "v", 3)
	playlist = w.Playlist("p", "v", completionHoldURI, completionHoldSegment, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXTINF") || strings.Contains(playlist, "3/1.m4s") {
		t.Fatalf("completed segment not published after hold:\n%s", playlist)
	}
	if got := w.SegmentData("p", "v", 3); !bytes.Equal(got, []byte("ab")) {
		t.Fatalf("segment data = %q", got)
	}
	// The part stays fetchable after completion; only eviction removes it.
	if err := w.WaitForPart(context.Background(), "p", "v", 3, 1); err != nil {
		t.Fatalf("part after completion = %v, want still fetchable", err)
	}
}

func TestWindowCompletionHoldClosesWhenNextParentStarts(t *testing.T) {
	w := NewWindow(WithSegmentCompletionDelay(50 * time.Millisecond))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 3, Part: 0, Start: 0, Duration: time.Second, Data: []byte("a")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 3, Part: 1, Start: time.Second, Duration: time.Second, Data: []byte("b")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 3, Start: 0, Duration: 2 * time.Second, Data: []byte("ab")})

	// The next parent starts before the completion hold expires. The previous
	// parent must become complete immediately so the playlist has no gap.
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Start: 2 * time.Second, Duration: time.Second, Data: []byte("c")})

	playlist := w.Playlist("p", "v", completionHoldURI, completionHoldSegment, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXTINF:2.000000,") || !strings.Contains(playlist, "4/0.m4s") {
		t.Fatalf("playlist lost the held parent when the next parent started:\n%s", playlist)
	}
	if strings.Contains(playlist, "3/1.m4s") {
		t.Fatalf("completed parent still advertised a part:\n%s", playlist)
	}
}

func TestWindowCompletionHoldEvictsAfterTimer(t *testing.T) {
	w := NewWindow(WithMaxBytes(2), WithSegmentCompletionDelay(20*time.Millisecond))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("a")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Data: []byte("abc")})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && w.Bytes() != 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := w.Bytes(); got != 0 {
		t.Fatalf("bytes after delayed eviction = %d, want 0", got)
	}
}

func TestWindowCompletionHoldCanceledByPresentationReset(t *testing.T) {
	w := NewWindow(WithSegmentCompletionDelay(30 * time.Millisecond))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p1", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p1", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("a")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p1", Track: "v", Generation: 1, MSN: 1, Start: 0, Duration: time.Second, Data: []byte("a")})

	observeEvent(t, w, Event{Kind: Init, Presentation: "p2", Track: "v", Generation: 1, Data: []byte("init2")})
	time.Sleep(60 * time.Millisecond)
	if got := w.Snapshot("p1", "v"); len(got.Segments) != 0 {
		t.Fatalf("stale segments survived presentation reset: %+v", got.Segments)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p2", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("x")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p2", Track: "v", Generation: 1, MSN: 1, Start: 0, Duration: time.Second, Data: []byte("x")})
	waitForSegmentComplete(t, w, "p2", "v", 1)
	if got := w.SegmentData("p2", "v", 1); !bytes.Equal(got, []byte("x")) {
		t.Fatalf("segment data after reset = %q", got)
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
	if err := w.Observe(Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 1}); err != nil {
		t.Fatal(err)
	}
	if err := w.Observe(Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 2}); err != nil {
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

func TestWindowWaitForMasterWaitsForCompleteMetadata(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1, Data: []byte("video-init")})
	w.SetVideoConfig(VideoConfig{FrameRate: 30, Bandwidth: 5000000, AverageBandwidth: 4000000})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- w.WaitForMaster(ctx, "p") }()
	select {
	case err := <-result:
		t.Fatalf("master wait returned before audio metadata: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "audio", Generation: 1, Data: []byte("audio-init")})
	w.SetAudioConfig(AudioConfig{Channels: 2, Bandwidth: 128000, AverageBandwidth: 128000})
	if err := <-result; err != nil {
		t.Fatalf("master wait error = %v", err)
	}
}

func TestWindowReloadPointAllowsTwoAheadButRejectsThree(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 7, Part: 0, Data: []byte("part")})

	if w.reloadPointUnavailableLocked("p", "v", 9) {
		t.Fatal("reload two parents ahead was rejected")
	}
	if !w.reloadPointUnavailableLocked("p", "v", 10) {
		t.Fatal("reload three parents ahead was accepted")
	}
}

func TestWindowRejectsReloadPartBeyondAdvanceLimit(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 7, Part: 0, Duration: time.Second, Data: []byte("part")})

	if w.reloadPartUnavailableLocked("p", "v", 7, 3) {
		t.Fatal("reload part at the advance limit was rejected")
	}
	if !w.reloadPartUnavailableLocked("p", "v", 7, 4) {
		t.Fatal("reload part beyond the advance limit was accepted")
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

func TestWindowWaitForFirstPartAfterInit(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})

	done := make(chan error, 1)
	go func() {
		done <- w.WaitForPart(context.Background(), "p", "v", 7, 0)
	}()
	select {
	case err := <-done:
		t.Fatalf("WaitForPart returned before first part: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 7, Part: 0, Data: []byte("part")})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForPart error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitForPart did not return after first part")
	}
}

func TestWindowPreloadHintURIBecomesThePublishedPartURI(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("part-0")})
	partURI := func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }
	segmentURI := func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }
	playlist := w.Playlist("p", "v", partURI, segmentURI, "init.mp4", nil)
	if !strings.Contains(playlist, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="1/1.m4s"`) {
		t.Fatalf("playlist omitted preload hint:\n%s", playlist)
	}
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 1, Data: []byte("part-1")})
	playlist = w.Playlist("p", "v", partURI, segmentURI, "init.mp4", nil)
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

func TestWindowExactPartWaitRejectsPartBeyondAdvanceLimit(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Data: []byte("parent")})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.WaitForPart(ctx, "p", "v", 4, 4); !errors.Is(err, ErrPartUnavailable) {
		t.Fatalf("exact future part wait error = %v, want ErrPartUnavailable", err)
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
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 1})
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

func TestPlaylistContainsLLTagsAndOnlyCompleteParentURI(t *testing.T) {
	w := NewWindow()
	if err := w.Observe(Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1, Data: []byte("init")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 4, Part: 0, Duration: 500 * time.Millisecond, Independent: true, Data: []byte("p")}); err != nil {
		t.Fatal(err)
	}
	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
	for _, want := range []string{"#EXT-X-VERSION:10", "#EXT-X-PART-INF:", "#EXT-X-SERVER-CONTROL:", `URI="4/0.m4s"`, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="4/1.m4s"`, "INDEPENDENT=YES"} {
		if !strings.Contains(playlist, want) {
			t.Errorf("playlist missing %q:\n%s", want, playlist)
		}
	}
	if strings.Contains(playlist, "#EXT-X-RENDITION-REPORT:") {
		t.Fatalf("single-rendition playlist must not contain a self-referential rendition report:\n%s", playlist)
	}
	if strings.Contains(playlist, "\n4.m4s\n") {
		t.Error("incomplete parent must not be published")
	}
}

func TestPlaylistEmitsIndependentSegmentsOnlyWhenAllParentsStartIndependently(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Independent: true, Data: []byte("key")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Duration: time.Second, Data: []byte("segment")})

	playlist := w.Playlist("p", "video", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("playlist omitted independent-segments declaration for an independent parent:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Part: 0, Duration: time.Second, Independent: false, Data: []byte("delta")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Duration: time.Second, Data: []byte("segment")})

	playlist = w.Playlist("p", "video", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
	if strings.Contains(playlist, "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("playlist declared independent segments despite a non-independent parent:\n%s", playlist)
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

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-PROGRAM-DATE-TIME:2026-08-31T22:36:12.351Z") {
		t.Fatalf("playlist missing program date time:\n%s", playlist)
	}
}

func TestPlaylistsKeepProgramDateTimeAlignedAcrossTrackDurations(t *testing.T) {
	w := NewWindow()
	base := time.Date(2026, time.August, 31, 22, 36, 12, 0, time.UTC)
	for _, track := range []struct {
		name      string
		parentDur []time.Duration
	}{
		{name: "video", parentDur: []time.Duration{2 * time.Second, 2 * time.Second}},
		{name: "audio", parentDur: []time.Duration{1984 * time.Millisecond, 2005 * time.Millisecond}},
	} {
		observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: track.name, Generation: 1})
		var start time.Duration
		for msn, duration := range track.parentDur {
			observeEvent(t, w, Event{
				Kind:            Part,
				Presentation:    "p",
				Track:           track.name,
				Generation:      1,
				MSN:             uint64(msn),
				Start:           start,
				Duration:        duration,
				ProgramDateTime: base.Add(time.Duration(msn) * 2 * time.Second),
				Data:            []byte("part"),
			})
			observeEvent(t, w, Event{
				Kind:         SegmentComplete,
				Presentation: "p",
				Track:        track.name,
				Generation:   1,
				MSN:          uint64(msn),
				Start:        start,
				Duration:     duration,
				Data:         []byte("segment"),
			})
			start += duration
		}
	}

	playlistDates := func(playlist string) []string {
		var dates []string
		for _, line := range strings.Split(playlist, "\n") {
			if strings.HasPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:") {
				dates = append(dates, strings.TrimPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:"))
			}
		}
		return dates
	}
	partURI := func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }
	segmentURI := func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }
	videoDates := playlistDates(w.Playlist("p", "video", partURI, segmentURI, "video-init.mp4", nil))
	audioDates := playlistDates(w.Playlist("p", "audio", partURI, segmentURI, "audio-init.mp4", nil))
	if len(videoDates) != 2 || len(audioDates) != 2 {
		t.Fatalf("program date time counts = video %v, audio %v", videoDates, audioDates)
	}
	if videoDates[1] != audioDates[1] {
		t.Fatalf("corresponding parent dates diverged: video=%s audio=%s", videoDates[1], audioDates[1])
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

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
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

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
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

func TestWindowResetsVideoConfigAcrossPresentationReset(t *testing.T) {
	w := NewWindow()
	config := VideoConfig{Codec: "avc1.64002a", Width: 1280, Height: 720, FrameRate: 30}
	w.SetVideoConfig(config)
	observeEvent(t, w, Event{Kind: Init, Presentation: "p1", Track: "video", Generation: 1})

	observeEvent(t, w, Event{Kind: Init, Presentation: "p2", Track: "video", Generation: 1})
	if got := w.VideoConfig(); got != (VideoConfig{}) {
		t.Fatalf("video config after reconnect = %+v, want empty config", got)
	}
}

func TestWindowRejectsStalePresentationSession(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p2", Session: 2, Track: "video", Generation: 1})
	if err := w.Observe(Event{Kind: Init, Presentation: "p1", Session: 1, Track: "video", Generation: 1}); err != ErrStalePresentation {
		t.Fatalf("stale presentation error = %v, want %v", err, ErrStalePresentation)
	}
	if got := w.Presentation(); got != "p2" {
		t.Fatalf("stale presentation replaced current session with %q", got)
	}
}

func TestWindowRejectsEqualSessionStalePresentation(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "current", Session: 7, Track: "video", Generation: 1})
	if err := w.Observe(Event{Kind: Init, Presentation: "stale", Session: 7, Track: "video", Generation: 1}); err != ErrStalePresentation {
		t.Fatalf("equal-session stale presentation error = %v", err)
	}
}

func TestWindowResetsBitrateOnGenerationChange(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	w.SetVideoFrameRate(30)
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Duration: time.Second, Data: make([]byte, 1000)})
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 2})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 2, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 2, MSN: 1, Duration: time.Second, Data: make([]byte, 100)})

	if got := w.VideoConfig().AverageBandwidth; got != 800 {
		t.Fatalf("generation-two average bandwidth = %d, want 800", got)
	}
	w.SetVideoFrameRate(24)
	if got := w.VideoConfig().FrameRate; got != 24 {
		t.Fatalf("generation-two frame rate = %v, want 24", got)
	}
}

func TestWindowTracksPerTrackBandwidth(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1, Data: []byte("init")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Duration: time.Second, Data: make([]byte, 1000)})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Part: 0, Duration: 2 * time.Second, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Duration: 2 * time.Second, Data: make([]byte, 3000)})

	config := w.VideoConfig()
	if config.Bandwidth != 12000 {
		t.Fatalf("peak video bandwidth = %d, want 12000", config.Bandwidth)
	}
	if config.AverageBandwidth != 10667 {
		t.Fatalf("average video bandwidth = %d, want 10667", config.AverageBandwidth)
	}
}

func TestWindowMetadataUpdatePreservesMeasuredBandwidth(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Duration: time.Second, Data: make([]byte, 1000)})

	w.SetVideoConfig(VideoConfig{Codec: "avc1.64002a", Width: 1280, Height: 720, FrameRate: 30})
	config := w.VideoConfig()
	if config.Bandwidth != 8000 || config.AverageBandwidth != 8000 {
		t.Fatalf("metadata update erased measured bandwidth: %+v", config)
	}
}

func TestCeilBitrateAvoidsIntermediateOverflow(t *testing.T) {
	if got := ceilBitrate(2_000_000_000, time.Second); got != 16_000_000_000 {
		t.Fatalf("ceilBitrate = %d, want 16000000000", got)
	}
}

func TestWindowWaiterLimit(t *testing.T) {
	w := NewWindow(withMaxWaiters(1))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})

	if err := acquireWaiter(w.waiters); err != nil {
		t.Fatalf("first waiter acquisition error = %v", err)
	}
	defer releaseWaiter(w.waiters)
	if err := w.Wait(context.Background(), "p", "video", 1, 0); err != ErrWaiterLimit {
		t.Fatalf("second waiter error = %v, want %v", err, ErrWaiterLimit)
	}
}

func TestWindowMasterWaitersDoNotStarveMediaWaiters(t *testing.T) {
	w := NewWindow(withMaxWaiters(1))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1, Data: []byte("init")})

	if err := acquireWaiter(w.masterWaiters); err != nil {
		t.Fatalf("master waiter acquisition error = %v", err)
	}
	defer releaseWaiter(w.masterWaiters)

	mediaCtx, cancelMedia := context.WithCancel(context.Background())
	cancelMedia()
	if err := w.Wait(mediaCtx, "p", "video", 1, 0); err != context.Canceled {
		t.Fatalf("media waiter error = %v, want %v", err, context.Canceled)
	}
}

func TestWindowStoresAudioConfig(t *testing.T) {
	w := NewWindow()
	w.SetAudioConfig(AudioConfig{Channels: 1})

	if got := w.AudioConfig(); got != (AudioConfig{Channels: 1}) {
		t.Fatalf("audio config = %+v", got)
	}
}

func TestPlaylistDurationsFreezeForPresentationAndRoundTargetDuration(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Duration: 1050 * time.Millisecond, Data: []byte("part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Duration: 5400 * time.Millisecond, Data: []byte("segment")})

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:6") {
		t.Fatalf("playlist did not use the conservative parent duration contract:\n%s", playlist)
	}
	if !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=1.100000") {
		t.Fatalf("playlist did not use the conservative part duration contract:\n%s", playlist)
	}
	if !strings.Contains(playlist, "PART-HOLD-BACK=3.301000") {
		t.Fatalf("playlist did not leave a decimal margin above three part targets:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 2, Part: 0, Duration: time.Second, Data: []byte("later-part")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 2, Duration: 6 * time.Second, Data: []byte("later-segment")})
	playlist = w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:6") || !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=1.100000") {
		t.Fatalf("playlist durations changed during presentation:\n%s", playlist)
	}
}

func TestDynamicPlaylistTargetDurationUsesObservedParentDurations(t *testing.T) {
	w := NewWindow(WithDynamicTargetDuration(time.Second))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})

	playlist := w.Playlist("p", "video", completionHoldURI, completionHoldSegment, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:1") {
		t.Fatalf("initial target duration was not clamped to one second:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Duration: 500 * time.Millisecond, Data: []byte("short")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Duration: 500 * time.Millisecond, Data: []byte("short")})
	playlist = w.Playlist("p", "video", completionHoldURI, completionHoldSegment, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:1") {
		t.Fatalf("short parent lowered the one-second floor:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Part: 0, Duration: 4 * time.Second, Data: []byte("long")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Duration: 4 * time.Second, Data: []byte("long")})
	playlist = w.Playlist("p", "video", completionHoldURI, completionHoldSegment, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:4") {
		t.Fatalf("target duration did not follow the four-second parent:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Init, Presentation: "next", Track: "video", Generation: 1})
	playlist = w.Playlist("next", "video", completionHoldURI, completionHoldSegment, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:1") {
		t.Fatalf("target duration did not reset at the presentation boundary:\n%s", playlist)
	}
}

func TestDynamicPlaylistTargetDurationIsSharedAcrossTracks(t *testing.T) {
	w := NewWindow(WithDynamicTargetDuration(time.Second))
	for _, track := range []string{"video", "audio"} {
		observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: track, Generation: 1})
		observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: track, Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte(track)})
	}
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Duration: time.Second, Data: []byte("video")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Duration: 4 * time.Second, Data: []byte("audio")})

	for _, track := range []string{"video", "audio"} {
		playlist := w.Playlist("p", track, completionHoldURI, completionHoldSegment, "init.mp4", nil)
		if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:4") {
			t.Errorf("%s playlist did not use the window-wide observed target:\n%s", track, playlist)
		}
	}
}

func TestPlaylistUsesConfiguredPartTargetWithoutGrowing(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Duration: 500 * time.Millisecond, Data: []byte("part")})

	playlist := w.Playlist("p", "v", func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=1.100000") {
		t.Fatalf("playlist did not preserve the presentation part target:\n%s", playlist)
	}
}

func TestPlaylistUsesConfiguredLiveHoldBack(t *testing.T) {
	w := NewWindow(WithPlaylistDurations(2*time.Second, 1100*time.Millisecond), WithPartHoldBack(5500*time.Millisecond))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})

	playlist := w.Playlist("p", "video", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:2") {
		t.Fatalf("playlist did not use the configured parent target:\n%s", playlist)
	}
	if !strings.Contains(playlist, "PART-HOLD-BACK=5.500000") {
		t.Fatalf("playlist did not use the configured part holdback:\n%s", playlist)
	}
	if !strings.Contains(playlist, "HOLD-BACK=6.000000") {
		t.Fatalf("playlist did not derive holdback from the parent target:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Init, Presentation: "next", Track: "video", Generation: 1})
	playlist = w.Playlist("next", "video", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4", nil)
	if !strings.Contains(playlist, "PART-HOLD-BACK=5.500000") || !strings.Contains(playlist, "#EXT-X-TARGETDURATION:2") {
		t.Fatalf("configured holdback did not survive a presentation reset:\n%s", playlist)
	}
}

func TestPlaylistRaisesShortHoldBackToPartMinimum(t *testing.T) {
	w := NewWindow(WithPlaylistDurations(2*time.Second, 1100*time.Millisecond), WithPartHoldBack(time.Second))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part")})

	playlist := w.Playlist("p", "video", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4", nil)
	if !strings.Contains(playlist, "PART-HOLD-BACK=3.301000") {
		t.Fatalf("playlist advertised a holdback below the part minimum:\n%s", playlist)
	}
}

func TestPlaylistDurationContractRoundsAndResetsPerPresentation(t *testing.T) {
	w := NewWindow(WithPlaylistDurations(2500*time.Millisecond, 750*time.Millisecond))
	observeEvent(t, w, Event{Kind: Init, Presentation: "first", Track: "v", Generation: 1})
	playlist := w.Playlist("first", "v", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4", nil)
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:3") || !strings.Contains(playlist, "#EXT-X-PART-INF:PART-TARGET=0.750000") {
		t.Fatalf("playlist did not use configured rounded durations:\n%s", playlist)
	}

	observeEvent(t, w, Event{Kind: Init, Presentation: "second", Track: "v", Generation: 1})
	playlist = w.Playlist("second", "v", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4", nil)
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
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Duration: 6 * time.Second, Data: []byte("audio")})

	for _, track := range []string{"video", "audio"} {
		playlist := w.Playlist("p", track, func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }, func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }, "init.mp4", nil)
		if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:6") {
			t.Errorf("%s playlist did not use the window-wide target duration:\n%s", track, playlist)
		}
	}
}

func TestWindowPlaylistRenditionReports(t *testing.T) {
	w := NewWindow()
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "audio", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Part: 0, Data: []byte("v")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: 1, Data: []byte("v")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Part: 0, Data: []byte("v")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: 2, Part: 1, Data: []byte("v")})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Part: 0, Data: []byte("a")})
	observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Data: []byte("a")})

	partURI := func(msn uint64, part uint32) string { return fmt.Sprintf("%d/%d.m4s", msn, part) }
	segmentURI := func(msn uint64) string { return fmt.Sprintf("%d.m4s", msn) }
	renditionURI := func(track string) string { return "/r/" + track + "/index.m3u8" }

	// Open audio segment with no parts yet falls back to its completed MSN.
	video := w.Playlist("p", "video", partURI, segmentURI, "init.mp4", renditionURI)
	if !strings.Contains(video, `#EXT-X-RENDITION-REPORT:URI="/r/audio/index.m3u8",LAST-MSN=1`) || strings.Contains(video, "LAST-PART") {
		t.Fatalf("video playlist audio report wrong:\n%s", video)
	}
	if strings.Contains(video, "/r/video/index.m3u8") {
		t.Fatalf("video playlist reports itself:\n%s", video)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "audio", Generation: 1, MSN: 2, Part: 0, Data: []byte("a")})
	audio := w.Playlist("p", "audio", partURI, segmentURI, "init.mp4", renditionURI)
	if !strings.Contains(audio, `#EXT-X-RENDITION-REPORT:URI="/r/video/index.m3u8",LAST-MSN=2,LAST-PART=1`) {
		t.Fatalf("audio playlist video report wrong:\n%s", audio)
	}
	audio = w.Playlist("p", "audio", partURI, segmentURI, "init.mp4", nil)
	if strings.Contains(audio, "RENDITION-REPORT") {
		t.Fatalf("nil rendition URI must not emit reports:\n%s", audio)
	}
}

func TestWindowEvictionNeverStrandsOpenSegments(t *testing.T) {
	// The only segment of the track is still open when its own bytes exceed
	// the eviction threshold: it remains in place so its remaining parts and
	// completion can still be published.
	w := NewWindow(WithMaxBytes(100))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("aaa")})

	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 1, Data: []byte("aaa")}); err != nil {
		t.Fatalf("open segment was stranded by eviction: %v", err)
	}
	if err := w.Observe(Event{Kind: SegmentComplete, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Data: []byte("aaaaaa")}); err != nil {
		t.Fatalf("segment completion failed after eviction pressure: %v", err)
	}
}

func TestWindowRejectsUnboundedOpenSegmentGrowth(t *testing.T) {
	w := NewWindow(WithMaxBytes(2))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	if err := w.Observe(Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("aaa")}); err != ErrWindowCapacity {
		t.Fatalf("open segment overflow error = %v, want %v", err, ErrWindowCapacity)
	}
	if got := w.Bytes(); got != len("aaa") {
		t.Fatalf("bytes after rejected part = %d, want %d", got, len("aaa"))
	}
}

func TestWindowEvictionRetainsHistoryForSmallTracks(t *testing.T) {
	// A large track driving the shared byte budget over its limit must not
	// drain a small sibling track below the retained history floor.
	w := NewWindow(WithMaxBytes(200))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "video", Generation: 1})
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "audio", Generation: 1})
	for msn := uint64(1); msn <= 20; msn++ {
		observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "video", Generation: 1, MSN: msn, Part: 0, Data: bytes.Repeat([]byte("v"), 20)})
		observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "video", Generation: 1, MSN: msn, Data: bytes.Repeat([]byte("v"), 20)})
		observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "audio", Generation: 1, MSN: msn, Part: 0, Data: []byte("a")})
		observeEvent(t, w, Event{Kind: SegmentComplete, Presentation: "p", Track: "audio", Generation: 1, MSN: msn, Data: []byte("a")})
	}

	got := w.Snapshot("p", "audio").Segments
	if len(got) != minRetainedSegments {
		t.Fatalf("audio history after eviction = %d segments, want %d", len(got), minRetainedSegments)
	}
	if first := got[0].MSN; first != 21-minRetainedSegments {
		t.Fatalf("audio history starts at MSN %d, want %d", first, 21-minRetainedSegments)
	}
	if got := w.Snapshot("p", "video").Segments; len(got) == 0 {
		t.Fatal("video history was emptied")
	}
}

func TestWindowDiscontinuityReleasesSegmentBytes(t *testing.T) {
	w := NewWindow(WithMaxBytes(100))
	observeEvent(t, w, Event{Kind: Init, Presentation: "p", Track: "v", Generation: 1})
	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 1, Part: 0, Data: []byte("part")})
	if got := w.Bytes(); got != len("part") {
		t.Fatalf("bytes before discontinuity = %d, want %d", got, len("part"))
	}

	observeEvent(t, w, Event{Kind: Discontinuity, Presentation: "p", Track: "v", Generation: 1})
	if got := w.Bytes(); got != 0 {
		t.Fatalf("bytes after discontinuity = %d, want 0", got)
	}

	observeEvent(t, w, Event{Kind: Part, Presentation: "p", Track: "v", Generation: 1, MSN: 2, Part: 0, Data: []byte("new")})
	if got := w.Bytes(); got != len("new") {
		t.Fatalf("bytes after new part = %d, want %d", got, len("new"))
	}
}
