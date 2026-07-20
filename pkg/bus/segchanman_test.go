package bus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func publishSegments(b *Bus, user, rendition string, n, start int) {
	for i := start; i < start+n; i++ {
		b.PublishSegment(context.Background(), user, rendition, &Seg{Filepath: fmt.Sprintf("seg-%05d", i)})
	}
}

func receiveSegments(t *testing.T, ch *SegChan, n int) []*Seg {
	t.Helper()
	out := []*Seg{}
	for len(out) < n {
		select {
		case seg := <-ch.C:
			out = append(out, seg)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for segment %d of %d", len(out)+1, n)
		}
	}
	return out
}

// A subscriber that joins after segments were published gets the last bufSize
// cached segments replayed, oldest first — the warmup buffer WebRTC playback
// relies on.
func TestSubscribeSegmentBufReplaysCachedSegments(t *testing.T) {
	b := NewBus()
	publishSegments(b, "user", "source", 5, 0)
	segChan := b.SubscribeSegmentBuf(context.Background(), "user", "source", 2)
	segs := receiveSegments(t, segChan, 2)
	require.Equal(t, "seg-00003", segs[0].Filepath)
	require.Equal(t, "seg-00004", segs[1].Filepath)
}

// Segments published back-to-back — faster than any reader drains — are
// delivered in publish order. The old per-subscriber goroutine fanout over an
// unbuffered channel made this a scheduling coin flip.
func TestPublishSegmentDeliversInOrder(t *testing.T) {
	b := NewBus()
	segChan := b.SubscribeSegment(context.Background(), "user", "source")
	publishSegments(b, "user", "source", 500, 0)
	segs := receiveSegments(t, segChan, 500)
	for i, seg := range segs {
		require.Equal(t, fmt.Sprintf("seg-%05d", i), seg.Filepath)
	}
}

// A subscriber that falls a full buffer behind loses its oldest queued
// segments, not the live edge: with ordered delivery a skip recovers at the
// next keyframe, while falling ever-further behind does not.
func TestPublishSegmentDropsOldestForLaggingSubscriber(t *testing.T) {
	b := NewBus()
	segChan := b.SubscribeSegment(context.Background(), "user", "source")
	publishSegments(b, "user", "source", chanSize+2, 0)
	segs := receiveSegments(t, segChan, chanSize)
	require.Equal(t, "seg-00002", segs[0].Filepath)
	require.Equal(t, fmt.Sprintf("seg-%05d", chanSize+1), segs[chanSize-1].Filepath)
}
