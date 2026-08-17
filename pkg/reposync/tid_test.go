package reposync

import (
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
)

// TestTIDForTimeIsAValidTID: the whole point is producing a key that can sit in
// an MST range next to real record keys, so it has to be TID syntax.
func TestTIDForTimeIsAValidTID(t *testing.T) {
	for _, ts := range []time.Time{
		time.Now(),
		time.Now().Add(-24 * time.Hour),
		time.Now().Add(-10 * 365 * 24 * time.Hour),
		time.Unix(0, 0),
		time.Date(2023, 6, 1, 12, 0, 0, 0, time.UTC),
	} {
		tid := TIDForTime(ts)
		_, err := syntax.ParseTID(tid)
		require.NoError(t, err, "TIDForTime(%s) = %q", ts, tid)
		require.Len(t, tid, 13)
	}
}

// TestTIDForTimeOrdering: the sort order of the strings has to match the order
// of the instants, because that equivalence is what makes a time window a key
// range.
func TestTIDForTimeOrdering(t *testing.T) {
	now := time.Now()
	spans := []time.Duration{
		0,
		-time.Microsecond,
		-time.Second,
		-time.Hour,
		-24 * time.Hour,
		-7 * 24 * time.Hour,
		-30 * 24 * time.Hour,
		-180 * 24 * time.Hour,
		-5 * 365 * 24 * time.Hour,
	}
	prev := TIDForTime(now.Add(spans[0]))
	for _, span := range spans[1:] {
		tid := TIDForTime(now.Add(span))
		require.Less(t, tid, prev, "an earlier instant must produce a smaller TID (span %s)", span)
		prev = tid
	}
}

// TestTIDForTimeBoundary is the property the windowed backfill relies on: a
// record stamped at or after t is inside the range that starts at TIDForTime(t),
// and one stamped before it is outside -- for every clock id, since we have no
// say over which one a remote PDS uses.
func TestTIDForTimeBoundary(t *testing.T) {
	t0 := time.Now().Add(-36 * time.Hour).Truncate(time.Microsecond)
	floor := TIDForTime(t0)

	for _, clockID := range []uint{0, 1, 7, 512, 1023} {
		at := string(syntax.NewTIDFromTime(t0, clockID))
		require.GreaterOrEqual(t, at, floor,
			"a TID stamped exactly at the floor instant (clock %d) must be in range", clockID)

		after := string(syntax.NewTIDFromTime(t0.Add(time.Microsecond), clockID))
		require.Greater(t, after, floor, "a TID stamped after the floor must be in range")

		before := string(syntax.NewTIDFromTime(t0.Add(-time.Microsecond), clockID))
		require.Less(t, before, floor, "a TID stamped before the floor must be out of range")
	}
}

// TestTIDForTimeAgainstNewTIDNow: a TID minted right now sorts above a floor
// taken a moment ago and below one taken a moment hence.
func TestTIDForTimeAgainstNewTIDNow(t *testing.T) {
	before := TIDForTime(time.Now().Add(-time.Second))
	now := string(syntax.NewTIDNow(0))
	after := TIDForTime(time.Now().Add(time.Second))

	require.Greater(t, now, before)
	require.Less(t, now, after)
}

// TestTimeForTIDRoundTrip: the watermark stored in a repo row has to be
// readable back as a timestamp, since that is how the sweep decides which
// window to walk next.
func TestTimeForTIDRoundTrip(t *testing.T) {
	for _, ts := range []time.Time{
		time.Now().Truncate(time.Microsecond),
		time.Now().Add(-90 * 24 * time.Hour).Truncate(time.Microsecond),
		time.Unix(0, 0),
	} {
		got, err := TimeForTID(TIDForTime(ts))
		require.NoError(t, err)
		require.True(t, got.Equal(ts.UTC()), "round trip of %s gave %s", ts.UTC(), got)
	}

	// A real TID keeps its timestamp too, clock id and all.
	tid := syntax.NewTIDNow(42)
	got, err := TimeForTID(string(tid))
	require.NoError(t, err)
	require.True(t, got.Equal(tid.Time()))

	for _, bad := range []string{"", "not-a-tid", "3jui7kd54zh2", "3JUI7KD54ZH2Y"} {
		_, err := TimeForTID(bad)
		require.Error(t, err, "TimeForTID(%q) should not parse", bad)
	}
}

// TestTIDForTimeMonotonic: repeatedly slicing a window off the front never
// walks backwards, which is what keeps the deepening ladder terminating.
func TestTIDForTimeMonotonic(t *testing.T) {
	now := time.Now()
	last := TIDForTime(now)
	for i := 1; i <= 100; i++ {
		tid := TIDForTime(now.Add(-time.Duration(i) * time.Minute))
		require.Less(t, tid, last)
		last = tid
	}
}
