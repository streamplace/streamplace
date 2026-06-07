package director

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
