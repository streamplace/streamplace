package bus

import (
	"context"
	"fmt"
	"testing"

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
