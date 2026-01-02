package aigateway

import (
	"strings"
	"sync"
)

const (
	// MaxTranscriptEvents is the maximum number of events to retain per streamer.
	MaxTranscriptEvents = 1000
)

// TranscriptStore provides thread-safe storage for transcript segments (timed cues)
// per streamer. Segments are limited to MaxTranscriptEvents per streamer.
type TranscriptStore struct {
	mu     sync.RWMutex
	segs   map[string][]TranscriptSegment
}

// NewTranscriptStore creates a new empty TranscriptStore.
func NewTranscriptStore() *TranscriptStore {
	return &TranscriptStore{
		segs: make(map[string][]TranscriptSegment),
	}
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

	addSeg := func(seg TranscriptSegment) {
		if seg.EndMS <= seg.StartMS {
			return
		}
		ts.segs[streamer] = append(ts.segs[streamer], seg)
		if len(ts.segs[streamer]) > MaxTranscriptEvents {
			removeCount := len(ts.segs[streamer]) - MaxTranscriptEvents
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
	return result
}

// Clear removes all transcript segments for the given streamer.
func (ts *TranscriptStore) Clear(streamer string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.segs, streamer)
}
