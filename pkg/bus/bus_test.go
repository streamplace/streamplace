package bus

import (
	"testing"
	"time"
)

func TestPublishPreservesSubscriptionOrder(t *testing.T) {
	b := NewBus()
	ch := b.Subscribe("repo")
	defer b.Unsubscribe("repo", ch)

	b.Publish("repo", "teleport")
	b.Publish("repo", "canceled")

	for _, want := range []Message{"teleport", "canceled"} {
		select {
		case got := <-ch:
			if got != want {
				t.Fatalf("received %v, want %v", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %v", want)
		}
	}
}

func TestPublishCallsOverflowForSlowSubscriber(t *testing.T) {
	b := NewBus()
	overflowed := false
	ch := b.SubscribeWithBacklogLimit("repo", func() {
		overflowed = true
	})
	b.mu.Lock()
	sub := b.clients["repo"][0]
	b.mu.Unlock()
	defer b.Unsubscribe("repo", ch)

	for i := 0; i <= maxQueuedMessages+cap(ch)+1; i++ {
		b.Publish("repo", i)
	}

	if !overflowed {
		t.Fatal("overflow callback was not called")
	}
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if !sub.closed {
		t.Fatal("slow subscriber remained active")
	}
	if len(sub.queue) != 0 {
		t.Fatalf("slow subscriber retained %d queued messages after overflow", len(sub.queue))
	}
}
