package media

import (
	"sort"
	"time"
)

// segmentBufferWindow is how long a stream's recent canonical segments are
// kept in memory by default. Moderation/report clips grab the last ~60s (see
// the makeClip / /clip handlers), so this is that window plus margin. A
// stream this node records keeps its segments longer (see RetainSegments).
const segmentBufferWindow = 2 * time.Minute

// segmentBufferMinAverage bounds a buffer's memory against pathologically
// short segments: a buffer holds at most its retention divided by this many
// segments (240 for the default window), keeping the newest.
const segmentBufferMinAverage = 500 * time.Millisecond

// BufferedSegment is one canonical segment held in memory.
type BufferedSegment struct {
	Start     time.Time // media start time
	Published bool
	Data      []byte // bare canonical .m4s bytes; never modified once buffered
	addedAt   time.Time
}

// segmentBuffer is a stream's time-bounded run of recent segments. All
// access goes through MediaManager (guarded by segBuffersMut), so it needs
// no lock of its own.
type segmentBuffer struct {
	retain time.Duration
	segs   []BufferedSegment
}

// bufferSegment appends a canonical segment to the stream's buffer and evicts
// what has aged out. Called for every validated segment — published or not,
// local or replicated — so whatever this node has seen of a stream recently
// can be clipped. The bytes are copied since the caller reuses its buffer.
func (mm *MediaManager) bufferSegment(did string, start time.Time, published bool, segment []byte) {
	now := time.Now()
	mm.segBuffersMut.Lock()
	defer mm.segBuffersMut.Unlock()
	b := mm.segmentBufferLocked(did)
	b.segs = append(b.segs, BufferedSegment{
		Start:     start,
		Published: published,
		Data:      append([]byte(nil), segment...),
		addedAt:   now,
	})
	b.evict(now)
}

// RetainSegments keeps the stream's segments in memory for at least d
// instead of the default window. A node recording a stream uses it to hold
// the segments of the object it is still uploading, which can't be read back
// until the upload completes.
func (mm *MediaManager) RetainSegments(did string, d time.Duration) {
	mm.segBuffersMut.Lock()
	defer mm.segBuffersMut.Unlock()
	b := mm.segmentBufferLocked(did)
	b.retain = max(b.retain, d)
}

// segmentBufferLocked returns the stream's buffer, creating it if needed.
// Caller holds segBuffersMut.
func (mm *MediaManager) segmentBufferLocked(did string) *segmentBuffer {
	// Lazily initialize: distributeSegment runs under MediaManager values
	// built by several constructors besides MakeMediaManager (isolated
	// ingest/whip/rtmp workers, tests), not all of which seed the map.
	if mm.segBuffers == nil {
		mm.segBuffers = map[string]*segmentBuffer{}
	}
	b := mm.segBuffers[did]
	if b == nil {
		b = &segmentBuffer{retain: segmentBufferWindow}
		mm.segBuffers[did] = b
	}
	return b
}

// evict drops segments older than the retention and enforces the count cap
// (keeping the newest). Caller holds segBuffersMut.
func (b *segmentBuffer) evict(now time.Time) {
	cutoff := now.Add(-b.retain)
	drop := 0
	for drop < len(b.segs) && b.segs[drop].addedAt.Before(cutoff) {
		drop++
	}
	if maxSegs := int(b.retain / segmentBufferMinAverage); len(b.segs)-drop > maxSegs {
		drop = len(b.segs) - maxSegs
	}
	if drop > 0 {
		b.segs = append(b.segs[:0:0], b.segs[drop:]...)
	}
}

// BufferedSegments returns the stream's segments held in memory, oldest
// first. A buffer whose segments have all aged out (a stream that ended) is
// dropped so it's freed.
func (mm *MediaManager) BufferedSegments(did string) []BufferedSegment {
	now := time.Now()
	mm.segBuffersMut.Lock()
	defer mm.segBuffersMut.Unlock()
	b := mm.segBuffers[did]
	if b == nil {
		return nil
	}
	b.evict(now)
	if len(b.segs) == 0 {
		delete(mm.segBuffers, did)
		return nil
	}
	out := append([]BufferedSegment(nil), b.segs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}
