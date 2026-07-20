package director

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spmetrics"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestExceedsMaxBitrate(t *testing.T) {
	oneSec := time.Second.Nanoseconds()

	// 1 MB over 1s = 8 Mbit/s.
	const eightMbit = 8 * 1000 * 1000
	megabyte := 1000 * 1000

	for _, tc := range []struct {
		name       string
		dataLen    int
		durationNS int64
		max        int
		wantRate   int
		wantKick   bool
	}{
		{"disabled when max is zero", megabyte, oneSec, 0, 0, false},
		{"well under the limit", megabyte, oneSec, 16_000_000, eightMbit, false},
		{"at the limit is fine", megabyte, oneSec, eightMbit, eightMbit, false},
		{"within the 10% margin is fine", megabyte, oneSec, 7_500_000, eightMbit, false},
		{"beyond the 10% margin kicks", megabyte, oneSec, 7_000_000, eightMbit, true},
		{"far over the limit kicks", megabyte, oneSec, 1_000_000, eightMbit, true},
		{"zero duration never kicks", megabyte, 0, 1, 0, false},
		{"negative duration never kicks", megabyte, -1, 1, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rate, kick := exceedsMaxBitrate(tc.dataLen, tc.durationNS, tc.max)
			require.Equal(t, tc.wantRate, rate)
			require.Equal(t, tc.wantKick, kick)
		})
	}
}

// 7_000_000 * 1.1 = 7_700_000 < 8_000_000, so it kicks; 7_500_000 * 1.1 =
// 8_250_000 > 8_000_000, so it's within the margin. The two cases bracket the
// 10% wiggle exactly.
func TestExceedsMaxBitrateMarginBoundary(t *testing.T) {
	const eightMbit = 8 * 1000 * 1000
	megabyte := 1000 * 1000
	_, justInside := exceedsMaxBitrate(megabyte, time.Second.Nanoseconds(), 7_300_000)  // *1.1 = 8.03M
	_, justOutside := exceedsMaxBitrate(megabyte, time.Second.Nanoseconds(), 7_200_000) // *1.1 = 7.92M
	require.False(t, justInside, "8Mbit within 10%% of 7.3Mbit max should not kick")
	require.True(t, justOutside, "8Mbit beyond 10%% of 7.2Mbit max should kick")
	require.Equal(t, eightMbit, func() int { r, _ := exceedsMaxBitrate(megabyte, time.Second.Nanoseconds(), 1); return r }())
}

func newTestStreamSession(t *testing.T) (*StreamSession, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)
	started := make(chan struct{})
	close(started)
	ss := &StreamSession{
		cli:     &config.CLI{},
		bus:     bus.NewBus(),
		g:       g,
		ctx:     gctx,
		started: started,
	}
	return ss, cancel
}

// Playback segments must be packetized + published strictly in enqueue order:
// the old ss.Go dispatch ran each segment's packetize concurrently, so a slow
// segment N could publish after segment N+1 — and WebRTC playback, which
// consumes segments strictly in arrival order, showed the swap as frame loss
// at every keyframe.
func TestPlaybackWorkerPublishesInSegmentOrder(t *testing.T) {
	ss, cancel := newTestStreamSession(t)
	segChan := ss.bus.SubscribeSegment(context.Background(), "did:test:streamer", "source")
	ss.addToWebRTCFn = func(ctx context.Context, spseg *placestream.Segment, rendition string, seg *bus.Seg) error {
		ss.bus.PublishSegment(ctx, spseg.Creator, rendition, seg)
		return nil
	}
	// under the queue cap so the drop-on-full policy can't fire — this test
	// isolates ordering
	const n = 50
	for i := 0; i < n; i++ {
		ss.AddPlaybackSegment(context.Background(), &placestream.Segment{Creator: "did:test:streamer"}, "source", &bus.Seg{Filepath: fmt.Sprintf("seg-%05d", i)})
	}
	cancel()
	require.NoError(t, ss.g.Wait())
	for i := 0; i < n; i++ {
		select {
		case seg := <-segChan.C:
			require.Equal(t, fmt.Sprintf("seg-%05d", i), seg.Filepath)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for segment %d", i)
		}
	}
}

// A wedged packetize pipeline must not stall the director's segment loop:
// once a rendition's queue is full, new segments for it drop (loudly) instead
// of blocking enqueue.
func TestAddPlaybackSegmentDropsWhenQueueFull(t *testing.T) {
	ss, cancel := newTestStreamSession(t)
	gate := make(chan struct{})
	entered := make(chan struct{})
	var once sync.Once
	processed := make(chan string, 128)
	ss.addToWebRTCFn = func(ctx context.Context, spseg *placestream.Segment, rendition string, seg *bus.Seg) error {
		once.Do(func() { close(entered) })
		<-gate
		processed <- seg.Filepath
		return nil
	}
	enqueue := func(i int) {
		ss.AddPlaybackSegment(context.Background(), &placestream.Segment{Creator: "did:test:streamer"}, "source", &bus.Seg{Filepath: fmt.Sprintf("seg-%05d", i)})
	}
	enqueue(0)
	<-entered // worker now wedged inside packetize; queue empty
	const queueCap = 64
	for i := 1; i <= queueCap; i++ {
		enqueue(i)
	}
	enqueue(queueCap + 1) // queue full — this one drops
	require.Equal(t, float64(1), testutil.ToFloat64(spmetrics.PlaybackQueueDropped.WithLabelValues("did:test:streamer", "source")))
	close(gate)
	cancel()
	require.NoError(t, ss.g.Wait())
	got := []string{}
	for {
		select {
		case fp := <-processed:
			got = append(got, fp)
		default:
			require.Len(t, got, queueCap+1)
			require.NotContains(t, got, fmt.Sprintf("seg-%05d", queueCap+1))
			return
		}
	}
}

// Each rendition gets its own worker: one rendition's wedged queue must not
// delay another rendition's segments.
func TestPlaybackWorkersArePerRendition(t *testing.T) {
	ss, cancel := newTestStreamSession(t)
	gate := make(chan struct{})
	entered := make(chan struct{})
	var once sync.Once
	processed := make(chan string, 16)
	ss.addToWebRTCFn = func(ctx context.Context, spseg *placestream.Segment, rendition string, seg *bus.Seg) error {
		if rendition == "source" {
			once.Do(func() { close(entered) })
			<-gate
		}
		processed <- rendition + ":" + seg.Filepath
		return nil
	}
	ss.AddPlaybackSegment(context.Background(), &placestream.Segment{Creator: "did:test:streamer"}, "source", &bus.Seg{Filepath: "seg-00000"})
	<-entered // source worker now wedged
	ss.AddPlaybackSegment(context.Background(), &placestream.Segment{Creator: "did:test:streamer"}, "other", &bus.Seg{Filepath: "seg-00001"})
	select {
	case got := <-processed:
		require.Equal(t, "other:seg-00001", got)
	case <-time.After(5 * time.Second):
		t.Fatal("one rendition's wedged queue blocked another rendition")
	}
	close(gate)
	cancel()
	require.NoError(t, ss.g.Wait())
}
