package llhls

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

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
	for _, want := range []string{"#EXT-X-VERSION:9", "#EXT-X-PART-INF:", "#EXT-X-SERVER-CONTROL:", `URI="4/0.m4s"`, "INDEPENDENT=YES"} {
		if !strings.Contains(playlist, want) {
			t.Errorf("playlist missing %q:\n%s", want, playlist)
		}
	}
	if strings.Contains(playlist, "\n4.m4s\n") {
		t.Error("incomplete parent must not be published")
	}
}
