package media

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/localdb"
)

type logCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logCapture) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logCapture) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func captureLogs(tb testing.TB, verbosity string) *logCapture {
	tb.Helper()
	var logs logCapture
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	tb.Cleanup(func() { slog.SetDefault(previousLogger) })
	previousVerbosity := flag.Lookup("v").Value.String()
	require.NoError(tb, flag.Set("v", verbosity))
	tb.Cleanup(func() { require.NoError(tb, flag.Set("v", previousVerbosity)) })
	return &logs
}

// Validated segments reach a subscriber in validation order.
func TestNewSegmentFanoutKeepsOrder(t *testing.T) {
	mm := &MediaManager{}
	ch := mm.NewSegment()
	const n = 200
	go func() {
		for i := 0; i < n; i++ {
			mm.notifySubscribers(context.Background(), &NewSegmentNotification{Segment: &localdb.Segment{ID: fmt.Sprint(i)}})
		}
	}()
	for i := 0; i < n; i++ {
		require.Equal(t, fmt.Sprint(i), (<-ch).Segment.ID)
	}
}

func TestSegmentSubscriberTimeoutAndRecovery(t *testing.T) {
	logs := captureLogs(t, "2")

	synctest.Test(t, func(t *testing.T) {
		sub := &segmentSubscriber{
			ch:    make(chan *NewSegmentNotification),
			queue: make(chan *NewSegmentNotification, 1),
		}
		done := make(chan struct{})
		go func() {
			sub.forward()
			close(done)
		}()
		not := &NewSegmentNotification{Segment: &localdb.Segment{ID: "stalled"}}
		sub.queue <- not
		synctest.Wait()
		time.Sleep(time.Minute - time.Second)
		synctest.Wait()
		require.Empty(t, logs.String(), "delivery must get a full minute")
		time.Sleep(time.Second)
		synctest.Wait()
		require.Contains(t, logs.String(), "segment subscriber did not take a segment")
		require.Contains(t, logs.String(), "segmentID=stalled")

		// An idle interval must not make the next delivery expire immediately.
		time.Sleep(2 * time.Minute)
		not = &NewSegmentNotification{Segment: &localdb.Segment{ID: "recovered"}}
		sub.queue <- not
		synctest.Wait()
		time.Sleep(time.Minute - time.Second)
		synctest.Wait()
		require.Equal(t, 1, strings.Count(logs.String(), "dropping it"))
		require.Same(t, not, <-sub.ch)
		close(sub.queue)
		<-done
	})
}

func BenchmarkSegmentSubscriberForward(b *testing.B) {
	sub := &segmentSubscriber{
		ch:    make(chan *NewSegmentNotification),
		queue: make(chan *NewSegmentNotification, segmentQueueSize),
	}
	done := make(chan struct{})
	go func() {
		sub.forward()
		close(done)
	}()
	b.Cleanup(func() {
		close(sub.queue)
		<-done
	})
	not := &NewSegmentNotification{Segment: &localdb.Segment{ID: "benchmark"}}
	b.ReportAllocs()
	for b.Loop() {
		sub.queue <- not
		<-sub.ch
	}
}
