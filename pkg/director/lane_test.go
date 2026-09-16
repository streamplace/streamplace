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
			turns[i].wait(context.Background())
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
	go func() { b.wait(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("b never got its turn")
	}
	b.release()
}

// A segment stuck longer than the patience is abandoned: the ones behind
// it go, and its late release is harmless.
func TestLanePatience(t *testing.T) {
	l := newLane()
	stuck, next := l.turn(), l.turn()
	start := time.Now()
	l.wait(context.Background(), next.ticket, 50*time.Millisecond)
	require.Less(t, time.Since(start), time.Second)
	next.release()
	stuck.release() // late, ignored
	third := l.turn()
	l.wait(context.Background(), third.ticket, time.Second)
	third.release()
}

func TestNilTurnIsNoop(t *testing.T) {
	var tn *turn
	tn.wait(context.Background())
	tn.release()
}
