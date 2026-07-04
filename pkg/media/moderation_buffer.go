package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"stream.place/streamplace/pkg/muxl"
)

// modClipWindow is how long a user's recent canonical segments are kept in the
// in-memory moderation buffer. Moderation/report clips grab the last ~60s (see
// the makeClip / /clip handlers), so this is that window plus margin. It mirrors
// the old on-disk moderationRetention (120s) — the point at which a reported
// stream's evidence is still available — but holds the bytes in RAM instead of
// on disk. A hard modClipMaxSegments cap bounds memory against pathologically
// short segments.
const (
	modClipWindow      = 2 * time.Minute
	modClipMaxSegments = 240
)

// modSegment is one canonical segment retained for moderation clipping.
type modSegment struct {
	startTime time.Time // media start time (what clips filter by)
	addedAt   time.Time // wall-clock arrival, for age eviction
	data      []byte    // bare canonical .m4s bytes
}

// modBuffer is a per-user time-bounded ring of recent canonical segments. All
// access goes through MediaManager (guarded by modBuffersMut), so it needs no
// lock of its own.
type modBuffer struct {
	segs []modSegment
}

// feedModerationBuffer appends a canonical segment to the user's moderation
// buffer and evicts anything older than modClipWindow. Called for every
// validated segment — published or not, so moderation can clip pre-live content
// exactly as the on-disk archive used to allow. The bytes are copied since the
// caller reuses its buffer.
func (mm *MediaManager) feedModerationBuffer(did string, startTime time.Time, segment []byte) {
	now := time.Now()
	mm.modBuffersMut.Lock()
	defer mm.modBuffersMut.Unlock()
	b := mm.modBuffers[did]
	if b == nil {
		b = &modBuffer{}
		mm.modBuffers[did] = b
	}
	b.segs = append(b.segs, modSegment{
		startTime: startTime,
		addedAt:   now,
		data:      append([]byte(nil), segment...),
	})
	b.evict(now)
}

// evict drops segments older than the retention window and enforces the hard
// count cap (keeping the newest). Caller holds modBuffersMut.
func (b *modBuffer) evict(now time.Time) {
	cutoff := now.Add(-modClipWindow)
	drop := 0
	for drop < len(b.segs) && b.segs[drop].addedAt.Before(cutoff) {
		drop++
	}
	if drop > 0 {
		b.segs = append(b.segs[:0:0], b.segs[drop:]...)
	}
	if len(b.segs) > modClipMaxSegments {
		b.segs = append(b.segs[:0:0], b.segs[len(b.segs)-modClipMaxSegments:]...)
	}
}

// moderationSegments returns copies of the user's retained segments whose media
// start time falls in [after, before] (either bound nil = open), oldest first.
// Stale buffers (all segments aged out — a stream that ended) are dropped from
// the map so they're freed.
func (mm *MediaManager) moderationSegments(user string, before, after *time.Time) [][]byte {
	now := time.Now()
	mm.modBuffersMut.Lock()
	defer mm.modBuffersMut.Unlock()
	b := mm.modBuffers[user]
	if b == nil {
		return nil
	}
	b.evict(now)
	if len(b.segs) == 0 {
		delete(mm.modBuffers, user)
		return nil
	}
	picked := make([]modSegment, 0, len(b.segs))
	for _, s := range b.segs {
		if after != nil && s.startTime.Before(*after) {
			continue
		}
		if before != nil && s.startTime.After(*before) {
			continue
		}
		picked = append(picked, s)
	}
	sort.Slice(picked, func(i, j int) bool {
		return picked[i].startTime.Before(picked[j].startTime)
	})
	out := make([][]byte, len(picked))
	for i, s := range picked {
		out[i] = s.data
	}
	return out
}

// ClipUser concatenates a user's recent canonical .m4s segments over the given
// time window into a single flat MP4 written to writer (moderation/report
// clips). Segments come from the in-memory moderation buffer — on-disk archival
// was removed — so the window is bounded by modClipWindow; a request for a
// longer span yields only what is still retained. The segments are blindly
// concatenatable MUXL, so the clip is their bytes wrapped in a synthesized
// ftyp+moov, and each segment's C2PA signature passes through verbatim.
func (mm *MediaManager) ClipUser(ctx context.Context, user string, writer io.Writer, before, after *time.Time) error {
	segs := mm.moderationSegments(user, before, after)
	if len(segs) == 0 {
		return fmt.Errorf("no segments found")
	}
	readers := make([]io.Reader, len(segs))
	for i, data := range segs {
		readers[i] = bytes.NewReader(data)
	}
	if err := muxl.RunMuxlWrap(ctx, io.MultiReader(readers...), "flat", writer); err != nil {
		return fmt.Errorf("unable to clip segments: %w", err)
	}
	return nil
}
