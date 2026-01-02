package aigateway

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
)

const (
	// MaxTranscriptEvents is the maximum number of events to retain per streamer.
	MaxTranscriptEvents = 1000
)

// TranscriptStore provides thread-safe storage for transcript segments (timed cues)
// per streamer. Segments are automatically deduplicated and limited to
// MaxTranscriptEvents per streamer.
type TranscriptStore struct {
	mu     sync.RWMutex
	segs   map[string][]TranscriptSegment
	seen   map[string]map[string]struct{}
}

// NewTranscriptStore creates a new empty TranscriptStore.
func NewTranscriptStore() *TranscriptStore {
	return &TranscriptStore{
		segs: make(map[string][]TranscriptSegment),
		seen: make(map[string]map[string]struct{}),
	}
}

func stableSegmentID(seg TranscriptSegment) string {
	if seg.ID != "" {
		return seg.ID
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seg.Text))
	return fmt.Sprintf("%d-%d-%08x", seg.StartMS, seg.EndMS, h.Sum32())
}

// AddEvent ingests a transcript event for the given streamer.
// If the event contains structured Segments, they are added as timed cues.
// Otherwise, a legacy Text-only event is converted into a best-effort segment.
// If the segment count exceeds MaxTranscriptEvents, older segments are discarded.
func (ts *TranscriptStore) AddEvent(streamer string, event TranscriptEvent) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if _, ok := ts.segs[streamer]; !ok {
		ts.segs[streamer] = make([]TranscriptSegment, 0, MaxTranscriptEvents)
	}
	if _, ok := ts.seen[streamer]; !ok {
		ts.seen[streamer] = make(map[string]struct{}, MaxTranscriptEvents)
	}

	abs64 := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}

	addSeg := func(seg TranscriptSegment) {
		if seg.EndMS <= seg.StartMS {
			return
		}
		seg.ID = stableSegmentID(seg)

		// If a producer revises the same logical segment (same time range) with slightly
		// different text/ID, replace the old entry in-place.
		// Scan from the end since new segments tend to be recent.
		replaceIdx := -1
		for i := len(ts.segs[streamer]) - 1; i >= 0; i-- {
			old := ts.segs[streamer][i]
			if abs64(old.StartMS-seg.StartMS) <= 50 && abs64(old.EndMS-seg.EndMS) <= 50 {
				replaceIdx = i
				break
			}
		}
		if replaceIdx >= 0 {
			old := ts.segs[streamer][replaceIdx]
			if old.ID == seg.ID {
				ts.segs[streamer][replaceIdx] = seg
				return
			}
			if _, ok := ts.seen[streamer][seg.ID]; ok {
				return
			}
			delete(ts.seen[streamer], old.ID)
			ts.seen[streamer][seg.ID] = struct{}{}
			ts.segs[streamer][replaceIdx] = seg
			return
		}

		if _, ok := ts.seen[streamer][seg.ID]; ok {
			return
		}
		ts.seen[streamer][seg.ID] = struct{}{}
		ts.segs[streamer] = append(ts.segs[streamer], seg)
		if len(ts.segs[streamer]) > MaxTranscriptEvents {
			// Trim to max segments, keeping the most recent
			removeCount := len(ts.segs[streamer]) - MaxTranscriptEvents
			for _, old := range ts.segs[streamer][:removeCount] {
				delete(ts.seen[streamer], old.ID)
			}
			ts.segs[streamer] = ts.segs[streamer][removeCount:]
		}
	}

	if len(event.Segments) == 0 {
		return
	}
	for _, seg := range event.Segments {
		if strings.TrimSpace(seg.Text) == "" {
			continue
		}
		addSeg(seg)
	}
}

// GetSegments returns a copy of all transcript segments for the given streamer.
// Returns nil if no segments exist for the streamer.
func (ts *TranscriptStore) GetSegments(streamer string) []TranscriptSegment {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	segs, ok := ts.segs[streamer]
	if !ok {
		return nil
	}

	result := make([]TranscriptSegment, len(segs))
	copy(result, segs)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].StartMS == result[j].StartMS {
			return result[i].EndMS < result[j].EndMS
		}
		return result[i].StartMS < result[j].StartMS
	})
	return result
}

// Clear removes all transcript segments for the given streamer.
func (ts *TranscriptStore) Clear(streamer string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.segs, streamer)
	delete(ts.seen, streamer)
}
