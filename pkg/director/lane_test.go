package director

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Turns claimed in order publish in order even when their work finishes
// backwards.
func TestLaneOrdersCompletions(t *testing.T) {
	l := newLane()
	const n = 20
	turns := make([]*turn, n)
	for i := range turns {
		turns[i] = l.turn()
	}
	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup
	for i := n - 1; i >= 0; i-- { // finish backwards, staggered
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			time.Sleep(time.Duration(n-i) * 2 * time.Millisecond)
			require.True(t, turns[i].wait(context.Background()))
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
			turns[i].release()
		}(i)
	}
	wg.Wait()
	for i, v := range order {
		require.Equal(t, i, v, "publication order")
	}
}

// A turn released without waiting (a failed transcode) does not hold the
// ones behind it.
func TestLaneReleaseWithoutWait(t *testing.T) {
	l := newLane()
	a, b := l.turn(), l.turn()
	a.release()
	done := make(chan struct{})
	go func() { require.True(t, b.wait(context.Background())); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("b never got its turn")
	}
	b.release()
}

// A segment stuck longer than the patience is abandoned: the ones behind
// it go, its own wait says so when it finally finishes (so it is
// discarded, never published after newer segments), and its late release
// is harmless.
func TestLanePatience(t *testing.T) {
	l := newLane()
	stuck, next := l.turn(), l.turn()
	start := time.Now()
	require.True(t, l.wait(context.Background(), next.ticket, 50*time.Millisecond))
	require.Less(t, time.Since(start), time.Second)
	next.release()
	// The stuck segment's work finishes now: its turn is gone.
	require.False(t, l.wait(context.Background(), stuck.ticket, time.Second))
	stuck.release() // late, ignored
	third := l.turn()
	require.True(t, l.wait(context.Background(), third.ticket, time.Second))
	third.release()
}

// A segment already waiting for its turn when a later one gives up is
// woken and told its turn is gone.
func TestLaneAbandonedWhileWaiting(t *testing.T) {
	l := newLane()
	stuck, waiting, impatient := l.turn(), l.turn(), l.turn()
	_ = stuck // never releases in time
	got := make(chan bool, 1)
	go func() { got <- l.wait(context.Background(), waiting.ticket, time.Minute) }()
	time.Sleep(10 * time.Millisecond)
	require.True(t, l.wait(context.Background(), impatient.ticket, 50*time.Millisecond))
	select {
	case ok := <-got:
		require.False(t, ok, "abandoned while waiting")
	case <-time.After(2 * time.Second):
		t.Fatal("the abandoned waiter was never woken")
	}
	impatient.release()
	waiting.release() // late, ignored
}

func TestNilTurnIsNoop(t *testing.T) {
	var tn *turn
	require.True(t, tn.wait(context.Background()))
	tn.release()
}
