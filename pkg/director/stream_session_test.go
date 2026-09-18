package director

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/placestream"
)

func TestAddToWebRTCPublishesSourceAndPrivatePlaybackCopies(t *testing.T) {
	gstinit.InitGST()
	_, filename, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "test", "fixtures", "sample-segment.mp4"))
	require.NoError(t, err)

	ctx := context.Background()
	const streamer = "phase2-dual-codec-playback"
	b := bus.NewBus()
	ss := &StreamSession{bus: b, cli: &config.CLI{}}
	timing := &bus.SegmentTiming{SegmentID: "phase2-dual-segment", SourceStart: time.Now()}
	sourceSub := b.SubscribeSegment(ctx, streamer, "source")
	defer b.UnsubscribeSegment(ctx, streamer, "source", sourceSub)
	webrtcSub := b.SubscribeSegment(ctx, streamer, media.WebRTCSourceRendition)
	defer b.UnsubscribeSegment(ctx, streamer, media.WebRTCSourceRendition, webrtcSub)

	require.NoError(t, ss.AddToWebRTC(ctx, &placestream.Segment{Creator: streamer}, "source", &bus.Seg{
		Data:      data,
		Published: true,
		Streamer:  streamer,
		Rendition: "source",
		Timing:    timing,
	}))

	select {
	case got := <-sourceSub.C:
		require.Equal(t, "source", got.Rendition)
		require.NotNil(t, got.PacketizedData)
	case <-time.After(5 * time.Second):
		t.Fatal("canonical source was not published")
	}
	select {
	case got := <-webrtcSub.C:
		require.Equal(t, media.WebRTCSourceRendition, got.Rendition)
		require.NotNil(t, got.PacketizedData)
		require.NotSame(t, timing, got.Timing, "fan-out timing must be independent")
	case <-time.After(5 * time.Second):
		t.Fatal("private WebRTC source was not published")
	}
}

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
