package bus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublishSegmentPreservesSubscriberOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewBus()
	sub := b.SubscribeSegment(ctx, "ordered", "source")
	defer b.UnsubscribeSegment(ctx, "ordered", "source", sub)

	const count = 128
	for i := 0; i < count; i++ {
		b.PublishSegment(ctx, "ordered", "source", &Seg{Filepath: string(rune(i))})
	}

	for i := 0; i < count; i++ {
		select {
		case got := <-sub.C:
			require.Equal(t, string(rune(i)), got.Filepath)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for segment %d", i)
		}
	}
}

func TestPublishSegmentDoesNotBlockOnSlowSubscriber(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewBus()
	slow := b.SubscribeSegment(ctx, "busy", "source")
	fast := b.SubscribeSegment(ctx, "busy", "source")
	defer b.UnsubscribeSegment(ctx, "busy", "source", slow)
	defer b.UnsubscribeSegment(ctx, "busy", "source", fast)

	for i := 0; i < cap(slow.C); i++ {
		slow.C <- &Seg{}
	}
	for {
		select {
		case slow.publish <- &Seg{}:
		default:
			goto full
		}
	}

full:
	want := &Seg{Filepath: "live"}
	done := make(chan struct{})
	go func() {
		b.PublishSegment(ctx, "busy", "source", want)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publishing to a slow subscriber blocked the bus")
	}

	select {
	case got := <-fast.C:
		require.Same(t, want, got)
	case <-time.After(time.Second):
		t.Fatal("healthy subscriber did not receive the segment")
	}
}
