package director

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/media"
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

// The rolling window averages the last bitrateWindowSize segments: a single
// spiky GoP that would have tripped the margin on its own is absorbed, while a
// sustained over-limit stream still kicks once the window catches up.
func TestRecordBitrateSampleRollingWindow(t *testing.T) {
	oneSec := time.Second.Nanoseconds()
	const max = 8_000_000 // 8 Mbit/s; kicks past 8.8 Mbit/s
	const compliant = 1_000_000

	// One 1.3x spiky GoP (scene cut) amid compliant segments: per-segment it
	// would be 10.4 Mbit/s — over the margin — but the windowed average
	// ((4*1 + 1.3)/5 = 1.06 MB/s = 8.48 Mbit/s) stays compliant.
	t.Run("single spiky segment does not kick", func(t *testing.T) {
		ss := &StreamSession{}
		for i := 0; i < bitrateWindowSize; i++ {
			_, kick := ss.recordBitrateSample(compliant, oneSec, max)
			require.False(t, kick)
		}
		rate, kick := ss.recordBitrateSample(compliant*13/10, oneSec, max)
		require.False(t, kick, "one spiky GoP should be absorbed by the window (rate %d)", rate)
	})

	// A sustained over-limit stream kicks immediately — the partial window at
	// session start is judged on its own.
	t.Run("sustained over-limit kicks", func(t *testing.T) {
		ss := &StreamSession{}
		_, kick := ss.recordBitrateSample(1_500_000, oneSec, max) // 12 Mbit/s
		require.True(t, kick)
	})

	// A stream that goes over-limit mid-session kicks within a couple of
	// segments as the window turns over.
	t.Run("mid-session over-limit kicks once window turns", func(t *testing.T) {
		ss := &StreamSession{}
		for i := 0; i < bitrateWindowSize; i++ {
			_, kick := ss.recordBitrateSample(compliant, oneSec, max)
			require.False(t, kick)
		}
		_, kick := ss.recordBitrateSample(1_500_000, oneSec, max)
		require.False(t, kick, "first over-limit segment is still averaged down")
		_, kick = ss.recordBitrateSample(1_500_000, oneSec, max)
		require.True(t, kick, "sustained over-limit must kick")
	})

	// Duration-less samples can't yield a rate: skipped, never kick, and don't
	// dilute the window.
	t.Run("zero-duration samples are skipped", func(t *testing.T) {
		ss := &StreamSession{}
		rate, kick := ss.recordBitrateSample(100_000_000, 0, max)
		require.False(t, kick)
		require.Equal(t, 0, rate)
		require.Empty(t, ss.bitrateWindow)
	})
}

// effectiveSegmentDuration defends the bitrate math against encoders whose
// video PTS collapses (frames stamped near-identically), which makes a ~1s GoP
// measure as milliseconds and inflates the computed bitrate by orders of
// magnitude. The cadence of signed segment start times is the fallback clock.
func TestEffectiveSegmentDuration(t *testing.T) {
	oneSec := time.Second.Nanoseconds()
	base := time.Date(2026, 8, 2, 1, 59, 52, 0, time.UTC)
	mkNotif := func(start time.Time, mediaDur int64) *media.NewSegmentNotification {
		return &media.NewSegmentNotification{
			Segment: &localdb.Segment{
				StartTime: start,
				MediaData: &localdb.SegmentMediaData{Duration: mediaDur},
			},
		}
	}

	t.Run("first segment is not enforceable without a cadence reference", func(t *testing.T) {
		ss := &StreamSession{}
		dur, ok := ss.effectiveSegmentDuration(mkNotif(base, oneSec))
		require.False(t, ok)
		require.Equal(t, int64(0), dur)
	})

	t.Run("collapsed PTS span rescued by cadence", func(t *testing.T) {
		ss := &StreamSession{}
		ss.effectiveSegmentDuration(mkNotif(base, oneSec))
		// Second segment: encoder says 8ms, cadence says 1s -> cadence wins.
		dur, ok := ss.effectiveSegmentDuration(mkNotif(base.Add(time.Second), 8_000_000))
		require.True(t, ok)
		require.Equal(t, oneSec, dur)
	})

	t.Run("out-of-order start times never shrink the duration", func(t *testing.T) {
		ss := &StreamSession{}
		ss.effectiveSegmentDuration(mkNotif(base, oneSec))
		dur, ok := ss.effectiveSegmentDuration(mkNotif(base.Add(-time.Minute), oneSec))
		require.True(t, ok)
		require.Equal(t, oneSec, dur)
	})

	// A wall-clock gap far beyond any GoP interval is a stop (reconnect or
	// pause), not cadence: billing it would deflate the window and mask an
	// over-limit stream, so the gap segment is unenforced and enforcement
	// restarts fresh.
	t.Run("a long gap resets enforcement instead of deflating the window", func(t *testing.T) {
		ss := &StreamSession{}
		ss.effectiveSegmentDuration(mkNotif(base, oneSec))
		dur, ok := ss.effectiveSegmentDuration(mkNotif(base.Add(time.Second), oneSec))
		require.True(t, ok)
		require.Equal(t, oneSec, dur)
		ss.recordBitrateSample(1_000_000, oneSec, 8_000_000)
		require.Len(t, ss.bitrateWindow, 1)

		// A 30s gap: segment excluded, window cleared.
		dur, ok = ss.effectiveSegmentDuration(mkNotif(base.Add(31*time.Second), oneSec))
		require.False(t, ok)
		require.Equal(t, int64(0), dur)
		require.Empty(t, ss.bitrateWindow, "gap reset clears the window")

		// The next segment begins a fresh cadence chain and is enforceable.
		dur, ok = ss.effectiveSegmentDuration(mkNotif(base.Add(32*time.Second), oneSec))
		require.True(t, ok)
		require.Equal(t, oneSec, dur)
	})

	// The production case: a ~2 Mbps stream (250 KB/s of segments) whose video
	// PTS span measured ~8ms per ~1s GoP reported a 251 Mbps bitrate and was
	// kicked against a 30 Mbps cap. With the cadence cross-check it streams on.
	t.Run("pathological encoder timestamps do not kick a compliant stream", func(t *testing.T) {
		ss := &StreamSession{}
		const max = 30_000_000
		const segBytes = 250_000 // ~2 Mbps at a 1s cadence
		for i := 0; i < bitrateWindowSize+2; i++ {
			notif := mkNotif(base.Add(time.Duration(i)*time.Second), 8_000_000)
			dur, ok := ss.effectiveSegmentDuration(notif)
			if !ok {
				require.Equal(t, 0, i, "only the first segment lacks a cadence reference")
				continue
			}
			rate, kick := ss.recordBitrateSample(segBytes, dur, max)
			require.False(t, kick, "segment %d must not kick (rate %d)", i, rate)
		}
	})

	// The same cross-check must not shield a genuinely over-limit stream: at a
	// true 40 Mbps the cadence-corrected window still exceeds the cap.
	t.Run("genuinely over-limit stream still kicks", func(t *testing.T) {
		ss := &StreamSession{}
		const max = 30_000_000
		const segBytes = 5_000_000 // ~40 Mbps at a 1s cadence
		kicked := false
		for i := 0; i < bitrateWindowSize+2; i++ {
			notif := mkNotif(base.Add(time.Duration(i)*time.Second), 8_000_000)
			dur, ok := ss.effectiveSegmentDuration(notif)
			if !ok {
				continue
			}
			if _, kick := ss.recordBitrateSample(segBytes, dur, max); kick {
				kicked = true
			}
		}
		require.True(t, kicked, "sustained 40 Mbps must kick against a 30 Mbps cap")
	})
}
