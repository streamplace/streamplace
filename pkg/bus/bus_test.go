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

func TestPublishClosesSlowSubscriberAtBacklogLimit(t *testing.T) {
	b := NewBus()
	overflowed := false
	ch := b.SubscribeWithBacklogLimit("repo", func() {
		overflowed = true
	})
	defer b.Unsubscribe("repo", ch)

	for i := 0; i <= maxQueuedMessages+cap(ch)+1; i++ {
		b.Publish("repo", i)
	}

	count := 0
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				if !overflowed {
					t.Fatal("overflow callback was not called")
				}
				if count > maxQueuedMessages+cap(ch) {
					t.Fatalf("received %d messages before subscription closed", count)
				}
				return
			}
			count++
		case <-timer.C:
			t.Fatal("timed out waiting for slow subscription to close")
		}
	}
}
