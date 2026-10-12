package media

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSegmentBufferNilMap guards the crash CI caught: distributeSegment runs
// under MediaManager values from constructors that don't seed segBuffers
// (ingest workers, tests), so bufferSegment must lazily initialize the map
// rather than panic assigning into a nil one.
func TestSegmentBufferNilMap(t *testing.T) {
	mm := &MediaManager{} // segBuffers deliberately nil
	require.NotPanics(t, func() {
		mm.bufferSegment("did:x", time.Unix(0, 0), true, []byte("seg"))
	})
	require.Len(t, mm.BufferedSegments("did:x"), 1)
}

// TestSegmentBufferOrder verifies segments fed out of order come back oldest
// first, carrying their published flag.
func TestSegmentBufferOrder(t *testing.T) {
	mm := &MediaManager{}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, i := range []int{2, 0, 4, 1, 3} {
		mm.bufferSegment("did:x", base.Add(time.Duration(i)*2*time.Second), i > 0, []byte(fmt.Sprintf("seg%d", i)))
	}
	all := mm.BufferedSegments("did:x")
	require.Len(t, all, 5)
	for i, s := range all {
		require.Equal(t, []byte(fmt.Sprintf("seg%d", i)), s.Data)
		require.Equal(t, i > 0, s.Published)
	}
}

// TestSegmentBufferCountCap verifies the hard segment cap keeps the newest
// entries and drops the oldest.
func TestSegmentBufferCountCap(t *testing.T) {
	mm := &MediaManager{}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limit := int(segmentBufferWindow / segmentBufferMinAverage)
	total := limit + 50
	for i := 0; i < total; i++ {
		mm.bufferSegment("did:x", base.Add(time.Duration(i)*time.Millisecond), true, []byte(fmt.Sprintf("s%d", i)))
	}
	got := mm.BufferedSegments("did:x")
	require.Len(t, got, limit)
	require.Equal(t, []byte(fmt.Sprintf("s%d", total-1)), got[len(got)-1].Data)
	require.Equal(t, []byte(fmt.Sprintf("s%d", total-limit)), got[0].Data)
}

// TestSegmentBufferRetain verifies RetainSegments holds a stream's segments
// past the default window (a recording node relies on it to cover the object
// it is still uploading), while other streams still age out on the default.
func TestSegmentBufferRetain(t *testing.T) {
	mm := &MediaManager{}
	mm.bufferSegment("did:recorded", time.Now(), true, []byte("a"))
	mm.bufferSegment("did:other", time.Now(), true, []byte("b"))
	mm.RetainSegments("did:recorded", 12*time.Minute)

	later := time.Now().Add(5 * time.Minute)
	mm.segBuffers["did:recorded"].evict(later)
	mm.segBuffers["did:other"].evict(later)
	require.Len(t, mm.segBuffers["did:recorded"].segs, 1)
	require.Empty(t, mm.segBuffers["did:other"].segs)

	mm.segBuffers["did:recorded"].evict(time.Now().Add(13 * time.Minute))
	require.Empty(t, mm.segBuffers["did:recorded"].segs)
}
