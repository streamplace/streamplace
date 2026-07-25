package media

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestMM() *MediaManager {
	return &MediaManager{modBuffers: map[string]*modBuffer{}}
}

// TestModerationBufferNilMap guards the crash CI caught: distributeSegment runs
// under MediaManager values from constructors that don't seed modBuffers (ingest
// workers, tests), so feedModerationBuffer must lazily initialize the map rather
// than panic assigning into a nil one.
func TestModerationBufferNilMap(t *testing.T) {
	mm := &MediaManager{} // modBuffers deliberately nil
	require.NotPanics(t, func() {
		mm.feedModerationBuffer("did:x", time.Unix(0, 0), []byte("seg"))
	})
	require.Len(t, mm.moderationSegments("did:x", nil, nil), 1)
}

// TestModerationBufferWindow verifies fed segments come back oldest-first and
// that the [after, before] filter selects the right slice by media start time.
func TestModerationBufferWindow(t *testing.T) {
	mm := newTestMM()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Feed 5 segments at t=base+0s..+8s (2s apart), out of insertion order.
	for _, i := range []int{2, 0, 4, 1, 3} {
		mm.feedModerationBuffer("did:x", base.Add(time.Duration(i)*2*time.Second), []byte(fmt.Sprintf("seg%d", i)))
	}

	all := mm.moderationSegments("did:x", nil, nil)
	require.Len(t, all, 5)
	require.Equal(t, []byte("seg0"), all[0], "oldest first")
	require.Equal(t, []byte("seg4"), all[4], "newest last")

	// after = base+3s should drop seg0 (0s) and seg1 (2s), keep seg2..seg4.
	after := base.Add(3 * time.Second)
	got := mm.moderationSegments("did:x", nil, &after)
	require.Len(t, got, 3)
	require.Equal(t, []byte("seg2"), got[0])

	// before = base+3s should keep only seg0 (0s) and seg1 (2s).
	before := base.Add(3 * time.Second)
	got = mm.moderationSegments("did:x", &before, nil)
	require.Len(t, got, 2)
	require.Equal(t, []byte("seg1"), got[1])
}

// TestModerationBufferCountCap verifies the hard segment cap keeps the newest
// modClipMaxSegments entries and drops the oldest.
func TestModerationBufferCountCap(t *testing.T) {
	mm := newTestMM()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	total := modClipMaxSegments + 50
	for i := 0; i < total; i++ {
		mm.feedModerationBuffer("did:x", base.Add(time.Duration(i)*time.Millisecond), []byte(fmt.Sprintf("s%d", i)))
	}
	got := mm.moderationSegments("did:x", nil, nil)
	require.Len(t, got, modClipMaxSegments)
	// The very first (oldest) segment must have been evicted; the newest kept.
	require.Equal(t, []byte(fmt.Sprintf("s%d", total-1)), got[len(got)-1])
	require.Equal(t, []byte(fmt.Sprintf("s%d", total-modClipMaxSegments)), got[0])
}

// TestClipUserNoSegments verifies ClipUser reports "no segments" (so the
// moderation report still goes through) when the buffer is empty for a user.
func TestClipUserNoSegments(t *testing.T) {
	mm := newTestMM()
	err := mm.ClipUser(context.Background(), "did:unknown", nil, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no segments")
}
