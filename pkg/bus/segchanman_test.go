package bus

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/require"
)

// Segments reach a subscriber in the order they were published, and the
// recent ones are replayed to a subscriber that arrives late.
func TestPublishSegmentKeepsOrder(t *testing.T) {
	b := NewBus()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		b.PublishSegment(ctx, "u", "source", &Seg{Filepath: fmt.Sprint("early", i)})
	}
	sub := b.SubscribeSegmentBuf(ctx, "u", "source", 2)
	defer b.UnsubscribeSegment(ctx, "u", "source", sub)
	const n = 500
	for i := 0; i < n; i++ {
		b.PublishSegment(ctx, "u", "source", &Seg{Filepath: fmt.Sprint(i)})
	}
	require.Equal(t, "early1", (<-sub.C).Filepath, "the last two are replayed")
	require.Equal(t, "early2", (<-sub.C).Filepath)
	for i := 0; i < n; i++ {
		require.Equal(t, fmt.Sprint(i), (<-sub.C).Filepath)
	}
}

// A subscriber that never reads does not hold the publisher.
func TestPublishSegmentDropsForStalledSubscriber(t *testing.T) {
	b := NewBus()
	ctx := context.Background()
	sub := b.SubscribeSegment(ctx, "u", "source")
	defer b.UnsubscribeSegment(ctx, "u", "source", sub)
	done := make(chan struct{})
	go func() {
		for i := 0; i < chanSize+50; i++ {
			b.PublishSegment(ctx, "u", "source", &Seg{})
		}
		close(done)
	}()
	<-done
	require.Len(t, sub.C, chanSize)
}

func TestUnsubscribeSegmentReleasesQueuedSegments(t *testing.T) {
	for _, tt := range []struct{ viewers, removed int }{{1, 0}, {3, 0}, {3, 1}, {3, 2}} {
		t.Run(fmt.Sprintf("%d/%d", tt.viewers, tt.removed), func(t *testing.T) {
			b := NewBus()
			ctx := context.Background()
			var survivors []*SegChan
			released := func() weak.Pointer[Seg] {
				var removed *SegChan
				for i := range tt.viewers {
					sub := b.SubscribeSegment(ctx, "u", "source")
					if i == tt.removed {
						removed = sub
					} else {
						survivors = append(survivors, sub)
					}
				}
				// Keep this payload out of the bus's intentional replay cache.
				queued := &Seg{Data: make([]byte, 32*1024)}
				removed.C <- queued
				ref := weak.Make(queued)
				b.UnsubscribeSegment(ctx, "u", "source", removed)
				b.UnsubscribeSegment(ctx, "u", "source", removed)
				return ref
			}()
			for _, sub := range survivors {
				defer b.UnsubscribeSegment(ctx, "u", "source", sub)
			}
			for i := range 2 {
				seg := &Seg{Filepath: fmt.Sprint(i)}
				b.PublishSegment(ctx, "u", "source", seg)
				for _, sub := range survivors {
					require.Len(t, sub.C, 1)
					require.Same(t, seg, <-sub.C)
				}
			}
			runtime.GC()
			runtime.GC()
			collected := released.Value() == nil
			runtime.KeepAlive(b)
			require.True(t, collected, "unsubscribed queues must be collectible while the bus remains alive")
			if tt.viewers == 1 {
				require.Empty(t, b.segChans, "idle stream keys must be released")
			}
		})
	}
}
