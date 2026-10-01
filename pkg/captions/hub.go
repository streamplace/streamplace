package captions

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultRetention is how much final caption history the hub keeps per track.
// It must cover the live HLS window plus the slowest player's lag behind it.
const DefaultRetention = 15 * time.Minute

const (
	subscriberBuffer     = 256
	maxTracksPerStream   = 64
	maxFinalCuesPerTrack = 8192
)

// Hub is the per-node registry of live caption tracks. Sources publish cues
// into it; outputs subscribe to cue events or read windows of final cues.
type Hub struct {
	retention time.Duration

	mu      sync.Mutex
	streams map[string]*streamCaptions
}

type streamCaptions struct {
	tracks map[string]*trackCaptions
	subs   map[chan Event]struct{}
}

type trackCaptions struct {
	track    Track
	final    []Cue // sorted by Start
	finalIDs map[string]struct{}
}

func NewHub(retention time.Duration) *Hub {
	if retention <= 0 {
		retention = DefaultRetention
	}
	return &Hub{retention: retention, streams: map[string]*streamCaptions{}}
}

func (h *Hub) stream(streamer string) *streamCaptions {
	s, ok := h.streams[streamer]
	if !ok {
		s = &streamCaptions{tracks: map[string]*trackCaptions{}, subs: map[chan Event]struct{}{}}
		h.streams[streamer] = s
	}
	return s
}

// Publish records a cue on a streamer's track, creating or updating the track,
// and fans the event out to subscribers. Final cues are immutable: republishing
// a final cue's ID is ignored. A slow subscriber misses events rather than
// stalling sources; final cues stay readable through Cues.
func (h *Hub) Publish(streamer string, track Track, cue Cue) {
	h.publish(streamer, track, cue, false)
}

// PublishCanonical joins a MUXL cue clipped across consecutive GoPs. The text
// and start of a final cue never change; only its discovered end can extend.
// This produces one display cue in HLS rather than duplicate clipped pieces.
func (h *Hub) PublishCanonical(streamer string, track Track, cue Cue) {
	h.publish(streamer, track, cue, true)
}

func (h *Hub) publish(streamer string, track Track, cue Cue, continuation bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.stream(streamer)
	t, ok := s.tracks[track.ID]
	if !ok {
		if len(s.tracks) >= maxTracksPerStream {
			return
		}
		t = &trackCaptions{finalIDs: map[string]struct{}{}}
		s.tracks[track.ID] = t
	} else if t.track.Origin != track.Origin {
		return
	}
	t.track = track
	if continuation && len(t.final) > 0 {
		last := &t.final[len(t.final)-1]
		sameCue := last.ID == cue.ID || (strings.HasPrefix(cue.ID, "muxl-") && !cue.Start.After(last.End))
		if strings.HasPrefix(cue.ID, "muxl-") && last.Text == cue.Text && !cue.Start.Before(last.Start) && !cue.End.After(last.End) {
			return
		}
		if last.Text == cue.Text && !cue.Start.Before(last.Start) && cue.End.After(last.End) && sameCue {
			last.End = cue.End
			cue = *last
			h.prune(t)
			for ch := range s.subs {
				select {
				case ch <- Event{Streamer: streamer, Track: track, Cue: cue}:
				default:
				}
			}
			return
		}
	}
	if _, done := t.finalIDs[cue.ID]; done {
		return
	}
	if cue.Final {
		t.finalIDs[cue.ID] = struct{}{}
		i := sort.Search(len(t.final), func(i int) bool { return t.final[i].Start.After(cue.Start) })
		t.final = append(t.final, Cue{})
		copy(t.final[i+1:], t.final[i:])
		t.final[i] = cue
		h.prune(t)
	}
	ev := Event{Streamer: streamer, Track: track, Cue: cue}
	for ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// prune drops final cues that ended more than the retention period before the
// latest final cue ended.
func (h *Hub) prune(t *trackCaptions) {
	var latest time.Time
	for _, c := range t.final {
		if c.End.After(latest) {
			latest = c.End
		}
	}
	cutoff := latest.Add(-h.retention)
	kept := t.final[:0]
	for _, c := range t.final {
		if c.End.Before(cutoff) {
			delete(t.finalIDs, c.ID)
			continue
		}
		kept = append(kept, c)
	}
	t.final = kept
	if over := len(t.final) - maxFinalCuesPerTrack; over > 0 {
		for _, cue := range t.final[:over] {
			delete(t.finalIDs, cue.ID)
		}
		copy(t.final, t.final[over:])
		t.final = t.final[:len(t.final)-over]
	}
}

// Subscribe returns cue events for a streamer's tracks until ctx is done, when
// the channel is closed.
func (h *Hub) Subscribe(ctx context.Context, streamer string) <-chan Event {
	ch := make(chan Event, subscriberBuffer)
	h.mu.Lock()
	h.stream(streamer).subs[ch] = struct{}{}
	h.mu.Unlock()
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		if s, ok := h.streams[streamer]; ok {
			delete(s.subs, ch)
			if len(s.subs) == 0 && len(s.tracks) == 0 {
				delete(h.streams, streamer)
			}
		}
		close(ch)
		h.mu.Unlock()
	}()
	return ch
}

// Tracks lists a streamer's live caption tracks, sorted by ID.
func (h *Hub) Tracks(streamer string) []Track {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamer]
	if !ok {
		return nil
	}
	tracks := make([]Track, 0, len(s.tracks))
	for _, t := range s.tracks {
		tracks = append(tracks, t.track)
	}
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].ID < tracks[j].ID })
	return tracks
}

// Cues returns the final cues of a track that overlap [from, to), sorted by
// start time.
func (h *Hub) Cues(streamer, trackID string, from, to time.Time) []Cue {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamer]
	if !ok {
		return nil
	}
	t, ok := s.tracks[trackID]
	if !ok {
		return nil
	}
	var cues []Cue
	for _, c := range t.final {
		if !c.Start.Before(to) {
			break
		}
		if c.End.After(from) {
			cues = append(cues, c)
		}
	}
	return cues
}

// EndSession forgets a streamer's tracks when their live session ends.
// Subscribers stay registered for the next session.
func (h *Hub) EndSession(streamer string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.streams[streamer]; ok {
		s.tracks = map[string]*trackCaptions{}
		if len(s.subs) == 0 {
			delete(h.streams, streamer)
		}
	}
}

// Retention bounds the final history available for archival reconciliation.
func (h *Hub) Retention() time.Duration { return h.retention }

// RemoveOrigin suppresses competing tracks when canonical captions appear.
func (h *Hub) RemoveOrigin(streamer string, origin Origin) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.streams[streamer]; s != nil {
		for id, t := range s.tracks {
			if t.track.Origin == origin {
				delete(s.tracks, id)
			}
		}
	}
}
