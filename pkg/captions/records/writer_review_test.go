package records

import (
	"context"
	"fmt"
	"github.com/bluesky-social/indigo/xrpc"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
)

func TestWriterReconcilesDroppedFinalEventsAtTeardown(t *testing.T) {
	h := newHarness(t, nil)
	h.w.StartSession(context.Background(), streamer, t0)
	h.w.mu.Lock()
	s := h.w.sessions[streamer]
	h.w.mu.Unlock()
	// Block the collector, not the nonblocking viewer path, to force overflow.
	s.mu.Lock()
	for i := range 1000 {
		h.hub.Publish(streamer, autoTrack, captions.Cue{ID: fmt.Sprint(i), Text: fmt.Sprintf("word%d", i), Start: t0.Add(time.Duration(i) * time.Millisecond), End: t0.Add(time.Duration(i+1) * time.Millisecond), Final: true})
	}
	s.mu.Unlock()
	h.w.StopSession(streamer)
	calls := h.repos.snapshot()
	total := 0
	for _, call := range calls {
		total += len(decode(call.rec))
	}
	require.Equal(t, 1000, total, "archival delivery reconciles the hub after a lossy viewer subscription overflows")
}

func TestWriterShutdownWaitsForReplacedFinishingSession(t *testing.T) {
	h := newHarness(t, nil)
	h.repos.block = make(chan struct{})
	defer close(h.repos.block)
	ctx, cancel := context.WithCancel(context.Background())
	h.w.StartSession(ctx, streamer, t0)
	say(h, autoTrack, "old", 0, "old", "speech")
	h.buffered(streamer, 2)
	h.w.mu.Lock()
	old := h.w.sessions[streamer]
	h.w.mu.Unlock()
	cancel()
	require.Eventually(t, func() bool { return old.pending() == 1 }, time.Second, time.Millisecond)
	h.w.StartSession(context.Background(), streamer, t0.Add(time.Hour))
	done := make(chan struct{})
	go func() { h.w.Stop(); close(done) }()
	select {
	case <-done:
		t.Fatal("shutdown returned while the replaced session's transcript was still blocked")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestWriterRateLimitedFinalBatchSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	reset := time.Now().Add(2 * time.Second)
	h := newHarness(t, func(cfg *Config) { cfg.OutboxDir = dir; cfg.Now = time.Now; cfg.StopTimeout = 20 * time.Millisecond })
	h.repos.fail = func(int) error {
		return &xrpc.Error{StatusCode: http.StatusTooManyRequests, Ratelimit: &xrpc.RatelimitInfo{Reset: reset}}
	}
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "last", 0, "last", "words")
	h.w.StopSession(streamer)
	h.w.Stop()
	calls := h.repos.snapshot()
	require.Len(t, calls, 1, "a known reset must not consume final retry attempts early")
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1, "the deadline leaves a durable final record")
	resumed := newHarness(t, func(cfg *Config) { cfg.OutboxDir = dir; cfg.Now = time.Now })
	require.Empty(t, resumed.repos.snapshot(), "recovery also honors the saved reset")
	require.Eventually(t, func() bool { return len(resumed.repos.snapshot()) == 1 }, 4*time.Second, time.Millisecond)
	require.Equal(t, calls[0].rkey, resumed.repos.snapshot()[0].rkey, "recovery retains the record key")
	require.Equal(t, calls[0].rec, resumed.repos.snapshot()[0].rec)
	require.Eventually(t, func() bool { files, err := os.ReadDir(dir); return err == nil && len(files) == 0 }, time.Second, time.Millisecond)
}

func TestWriterBoundsDedupHistoryWithoutRearchivingExpiredReplay(t *testing.T) {
	h := newHarness(t, nil)
	s := &session{w: h.w, streamer: streamer, mediaStart: t0, origin: true, tracks: map[string]*trackBuf{}}
	var first captions.Event
	for i := range 100 {
		ev := captions.Event{Streamer: streamer, Track: autoTrack, Cue: captions.Cue{ID: fmt.Sprint(i), Text: fmt.Sprintf("word%d", i), Start: t0.Add(time.Duration(i) * time.Minute), End: t0.Add(time.Duration(i)*time.Minute + time.Second), Final: true}}
		if i == 0 {
			first = ev
		}
		s.take(ev)
		s.flush(context.Background(), false)
		h.advance(time.Minute)
	}
	require.LessOrEqual(t, len(s.tracks[autoTrack.ID].seen), 16, "deduplication retains a bounded replay window, not a stream-lifetime ledger")
	s.take(first)
	s.flush(context.Background(), true)
	calls := h.repos.snapshot()
	require.Len(t, calls, 100, "an expired replay does not create another transcript")
	require.Equal(t, "word99", calls[99].rec.Text)
}

func TestWriterExtendedCanonicalIDStaysDeduplicated(t *testing.T) {
	h := newHarness(t, func(cfg *Config) { cfg.Hub = captions.NewHub(time.Second) })
	h.hub = h.w.cfg.Hub
	h.w.StartSession(context.Background(), streamer, t0)
	cue := captions.Cue{ID: "long", Text: "A long line", Start: t0.Add(100 * time.Millisecond), End: t0.Add(300 * time.Millisecond), Final: true}
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.buffered(streamer, 3)
	cue.Start = cue.End
	cue.End = t0.Add(2 * time.Second)
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.flushed()
	h.flushed()
	h.w.StopSession(streamer)
	calls := h.repos.snapshot()
	require.Len(t, calls, 1, "extending a canonical cue updates its deduplication lifetime, not just its pending words")
	require.Equal(t, int64(2000), decode(calls[0].rec)[2].EndMs)
}
