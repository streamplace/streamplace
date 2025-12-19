package aigateway

import (
	"sync"
	"time"
)

const (
	// MaxTranscriptEvents is the maximum number of events to retain per streamer.
	MaxTranscriptEvents = 100

	// TranscriptRetention is how long to retain transcript events before cleanup.
	TranscriptRetention = 5 * time.Minute
)

// TranscriptStore provides thread-safe storage for transcript events per streamer.
// Events are automatically limited to MaxTranscriptEvents per streamer.
type TranscriptStore struct {
	mu     sync.RWMutex
	events map[string][]TranscriptEvent
}

// NewTranscriptStore creates a new empty TranscriptStore.
func NewTranscriptStore() *TranscriptStore {
	return &TranscriptStore{
		events: make(map[string][]TranscriptEvent),
	}
}

// AddEvent adds a transcript event for the given streamer.
// If the event count exceeds MaxTranscriptEvents, older events are discarded.
func (ts *TranscriptStore) AddEvent(streamer string, event TranscriptEvent) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if _, ok := ts.events[streamer]; !ok {
		ts.events[streamer] = make([]TranscriptEvent, 0, MaxTranscriptEvents)
	}

	ts.events[streamer] = append(ts.events[streamer], event)

	// Trim to max events, keeping the most recent
	if len(ts.events[streamer]) > MaxTranscriptEvents {
		ts.events[streamer] = ts.events[streamer][len(ts.events[streamer])-MaxTranscriptEvents:]
	}
}

// GetEvents returns a copy of all transcript events for the given streamer.
// Returns nil if no events exist for the streamer.
func (ts *TranscriptStore) GetEvents(streamer string) []TranscriptEvent {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	events, ok := ts.events[streamer]
	if !ok {
		return nil
	}

	result := make([]TranscriptEvent, len(events))
	copy(result, events)
	return result
}

// GetEventsSince returns transcript events received after the given time.
// Returns nil if no matching events exist.
func (ts *TranscriptStore) GetEventsSince(streamer string, since time.Time) []TranscriptEvent {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	events, ok := ts.events[streamer]
	if !ok {
		return nil
	}

	var result []TranscriptEvent
	for _, e := range events {
		if e.ReceivedAt.After(since) {
			result = append(result, e)
		}
	}
	return result
}

// Clear removes all transcript events for the given streamer.
func (ts *TranscriptStore) Clear(streamer string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.events, streamer)
}

// Cleanup removes events older than TranscriptRetention and removes
// streamers with no remaining events. Should be called periodically.
func (ts *TranscriptStore) Cleanup() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	cutoff := time.Now().Add(-TranscriptRetention)
	for streamer, events := range ts.events {
		var kept []TranscriptEvent
		for _, e := range events {
			if e.ReceivedAt.After(cutoff) {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(ts.events, streamer)
		} else {
			ts.events[streamer] = kept
		}
	}
}
