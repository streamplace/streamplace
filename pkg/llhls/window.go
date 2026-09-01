// Package llhls contains the bounded in-memory state used by the LL-HLS
// origin. It deliberately knows nothing about GStreamer or HTTP.
package llhls

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrStalePresentation = errors.New("llhls: stale presentation")
	ErrPartOrder         = errors.New("llhls: invalid part order")
	ErrGeneration        = errors.New("llhls: stale configuration generation")
)

type EventKind uint8

const (
	Init EventKind = iota + 1
	Part
	SegmentComplete
	Discontinuity
	SessionEnd
)

type Event struct {
	Kind            EventKind
	Presentation    string
	Track           string
	Generation      uint64
	MSN             uint64
	Part            uint32
	Start           time.Duration
	Duration        time.Duration
	Independent     bool
	ProgramDateTime time.Time
	Data            []byte
}

type PartSnapshot struct {
	Index           uint32
	Start, Duration time.Duration
	Independent     bool
	Data            []byte
}

type SegmentSnapshot struct {
	MSN             uint64
	Start, Duration time.Duration
	Parts           []PartSnapshot
	Complete        bool
	Data            []byte
}

type Snapshot struct {
	Presentation, Track  string
	Generation           uint64
	Init                 []byte
	Segments             []SegmentSnapshot
	Ended                bool
	ProgramDateTime      time.Time
	ProgramDateTimeStart time.Duration
}

type VideoConfig struct {
	Codec  string
	Width  int
	Height int
}

type Window struct {
	mu                   sync.Mutex
	presentation         string
	tracks               map[string]*track
	maxSegments          int
	maxBytes             int
	bytes                int
	changed              chan struct{}
	videoConfig          VideoConfig
	programDateTime      time.Time
	programDateTimeStart time.Duration
}

type track struct {
	generation uint64
	init       []byte
	segments   []*segment
	ended      bool
}

type segment struct {
	msn             uint64
	start, duration time.Duration
	parts           []*part
	complete        bool
	data            []byte
}

type part struct {
	index           uint32
	start, duration time.Duration
	independent     bool
	data            []byte
}

type Option func(*Window)

func WithMaxSegments(n int) Option { return func(w *Window) { w.maxSegments = n } }
func WithMaxBytes(n int) Option    { return func(w *Window) { w.maxBytes = n } }

func NewWindow(opts ...Option) *Window {
	w := &Window{tracks: make(map[string]*track), changed: make(chan struct{})}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Window) Observe(ev Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ev.Presentation == "" || ev.Track == "" {
		return fmt.Errorf("llhls: presentation and track are required")
	}
	if w.presentation == "" {
		w.presentation = ev.Presentation
	}
	if ev.Presentation != w.presentation {
		// A new init is the explicit reconnect boundary. Drop the old timeline
		// before accepting it so media URLs from the previous presentation stay
		// unusable and memory is released promptly.
		if ev.Kind != Init {
			return ErrStalePresentation
		}
		for _, t := range w.tracks {
			w.removeTrackBytes(t)
		}
		w.tracks = make(map[string]*track)
		w.presentation = ev.Presentation
		w.programDateTime = time.Time{}
		w.programDateTimeStart = 0
	}

	t := w.tracks[ev.Track]
	if t == nil {
		t = &track{}
		w.tracks[ev.Track] = t
	}
	if ev.Kind == Init {
		if t.generation > ev.Generation {
			return ErrGeneration
		}
		if t.generation != ev.Generation {
			w.removeTrackBytes(t)
			t.segments = nil
			t.ended = false
		}
		t.generation, t.init = ev.Generation, append([]byte(nil), ev.Data...)
		w.notify()
		return nil
	}
	if t.generation != ev.Generation {
		return ErrGeneration
	}
	switch ev.Kind {
	case Part:
		s := w.findSegment(t, ev.MSN)
		if s == nil {
			if ev.Part != 0 {
				return ErrPartOrder
			}
			s = w.findOrCreateSegment(t, ev)
		}
		if ev.Part != uint32(len(s.parts)) {
			return ErrPartOrder
		}
		if w.programDateTime.IsZero() && !ev.ProgramDateTime.IsZero() {
			w.programDateTime = ev.ProgramDateTime
			w.programDateTimeStart = ev.Start
		}
		p := &part{index: ev.Part, start: ev.Start, duration: ev.Duration, independent: ev.Independent, data: append([]byte(nil), ev.Data...)}
		s.parts = append(s.parts, p)
		w.bytes += len(p.data)
	case SegmentComplete:
		s := w.findSegment(t, ev.MSN)
		if s == nil {
			return fmt.Errorf("llhls: segment %d has no parts", ev.MSN)
		}
		if s.complete {
			return nil
		}
		s.start, s.duration, s.complete = ev.Start, ev.Duration, true
		s.data = append([]byte(nil), ev.Data...)
		w.bytes += len(s.data)
	case Discontinuity:
		t.segments = nil
		t.ended = false
	case SessionEnd:
		t.ended = true
	default:
		return fmt.Errorf("llhls: unknown event kind %d", ev.Kind)
	}
	w.evict()
	w.notify()
	return nil
}

func (w *Window) findSegment(t *track, msn uint64) *segment {
	for _, s := range t.segments {
		if s.msn == msn {
			return s
		}
	}
	return nil
}

func (w *Window) findOrCreateSegment(t *track, ev Event) *segment {
	if s := w.findSegment(t, ev.MSN); s != nil {
		return s
	}
	s := &segment{msn: ev.MSN, start: ev.Start, duration: ev.Duration}
	t.segments = append(t.segments, s)
	sort.Slice(t.segments, func(i, j int) bool { return t.segments[i].msn < t.segments[j].msn })
	return s
}

func (w *Window) evict() {
	for _, t := range w.tracks {
		for (w.maxSegments > 0 && len(t.segments) > w.maxSegments) || (w.maxBytes > 0 && w.bytes > w.maxBytes) {
			if len(t.segments) == 0 {
				break
			}
			w.removeSegmentBytes(t.segments[0])
			t.segments = t.segments[1:]
		}
	}
}
func (w *Window) removeTrackBytes(t *track) {
	for _, s := range t.segments {
		w.removeSegmentBytes(s)
	}
}
func (w *Window) removeSegmentBytes(s *segment) {
	for _, p := range s.parts {
		w.bytes -= len(p.data)
	}
	w.bytes -= len(s.data)
}
func (w *Window) notify() { close(w.changed); w.changed = make(chan struct{}) }

func (w *Window) Changed() <-chan struct{} { w.mu.Lock(); defer w.mu.Unlock(); return w.changed }
func (w *Window) Bytes() int               { w.mu.Lock(); defer w.mu.Unlock(); return w.bytes }

func (w *Window) Presentation() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.presentation
}

func (w *Window) SetVideoConfig(config VideoConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.videoConfig = config
}

func (w *Window) VideoConfig() VideoConfig {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.videoConfig
}

func (w *Window) Snapshot(presentation, trackID string) Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	if presentation != w.presentation {
		return Snapshot{}
	}
	t := w.tracks[trackID]
	if t == nil {
		return Snapshot{Presentation: w.presentation, Track: trackID}
	}
	s := Snapshot{
		Presentation:         w.presentation,
		Track:                trackID,
		Generation:           t.generation,
		Init:                 append([]byte(nil), t.init...),
		Ended:                t.ended,
		ProgramDateTime:      w.programDateTime,
		ProgramDateTimeStart: w.programDateTimeStart,
	}
	for _, seg := range t.segments {
		ss := SegmentSnapshot{MSN: seg.msn, Start: seg.start, Duration: seg.duration, Complete: seg.complete, Data: append([]byte(nil), seg.data...)}
		for _, p := range seg.parts {
			ss.Parts = append(ss.Parts, PartSnapshot{Index: p.index, Start: p.start, Duration: p.duration, Independent: p.independent, Data: append([]byte(nil), p.data...)})
		}
		s.Segments = append(s.Segments, ss)
	}
	return s
}

func (w *Window) Data(presentation, trackID string, msn uint64, partIndex uint32) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if presentation != w.presentation {
		return nil
	}
	if t := w.tracks[trackID]; t != nil {
		if s := w.findSegment(t, msn); s != nil {
			if int(partIndex) < len(s.parts) {
				return append([]byte(nil), s.parts[partIndex].data...)
			}
			// A preload request can race the parent boundary. If the requested
			// part is beyond a completed parent, serve the first part of the
			// following parent instead of returning a spurious 404.
			if s.complete {
				if next := w.findSegment(t, msn+1); next != nil && len(next.parts) > 0 {
					return append([]byte(nil), next.parts[0].data...)
				}
			}
		}
	}
	return nil
}

func (w *Window) SegmentData(presentation, trackID string, msn uint64) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if presentation != w.presentation {
		return nil
	}
	if t := w.tracks[trackID]; t != nil {
		if s := w.findSegment(t, msn); s != nil && s.complete {
			return append([]byte(nil), s.data...)
		}
	}
	return nil
}

func (w *Window) Wait(ctx context.Context, presentation, trackID string, msn uint64, partIndex uint32) error {
	for {
		s := w.Snapshot(presentation, trackID)
		for _, seg := range s.Segments {
			if seg.MSN > msn || (seg.MSN == msn && len(seg.Parts) > int(partIndex)) {
				return nil
			}
		}
		select {
		case <-w.Changed():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Playlist renders the media playlist for one track. URIs are supplied by the
// caller so routing and presentation identifiers remain outside this package.
func (w *Window) Playlist(presentation, trackID string, partURI func(uint64, uint32) string, segmentURI func(uint64) string, initURI string) string {
	s := w.Snapshot(presentation, trackID)
	if s.Track == "" {
		return ""
	}
	targetSeconds, partTarget := w.playlistDurations(s)
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:10\n#EXT-X-TARGETDURATION:%d\n#EXT-X-PART-INF:PART-TARGET=%.6f\n#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=%.6f,HOLD-BACK=%.6f\n", targetSeconds, partTarget.Seconds(), 3*partTarget.Seconds(), 3*float64(targetSeconds))
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n#EXT-X-MAP:URI=%q\n", firstMSN(s), initURI)
	openMSN := uint64(0)
	if len(s.Segments) > 0 {
		if last := s.Segments[len(s.Segments)-1]; !last.Complete {
			openMSN = last.MSN
		}
	}
	for _, seg := range s.Segments {
		if !seg.Complete && seg.MSN == openMSN {
			for _, p := range seg.Parts {
				fmt.Fprintf(&b, "#EXT-X-PART:DURATION=%.6f,URI=%q", p.Duration.Seconds(), partURI(seg.MSN, p.Index))
				if p.Independent {
					b.WriteString(",INDEPENDENT=YES")
				}
				b.WriteByte('\n')
			}
		}
		if seg.Complete {
			if !s.ProgramDateTime.IsZero() {
				programDateTime := s.ProgramDateTime.Add(seg.Start - s.ProgramDateTimeStart)
				fmt.Fprintf(&b, "#EXT-X-PROGRAM-DATE-TIME:%s\n", programDateTime.Format(time.RFC3339Nano))
			}
			fmt.Fprintf(&b, "#EXTINF:%.6f,\n%s\n", seg.Duration.Seconds(), segmentURI(seg.MSN))
		}
	}
	if !s.Ended && len(s.Segments) > 0 {
		last := s.Segments[len(s.Segments)-1]
		nextMSN, nextPart := last.MSN, uint32(len(last.Parts))
		if last.Complete {
			nextMSN++
			nextPart = 0
		}
		fmt.Fprintf(&b, "#EXT-X-PRELOAD-HINT:TYPE=PART,URI=%q\n", partURI(nextMSN, nextPart))
	}
	if s.Ended {
		b.WriteString("#EXT-X-ENDLIST\n")
	}
	return b.String()
}

func (w *Window) playlistDurations(s Snapshot) (targetSeconds int64, partTarget time.Duration) {
	targetSeconds = 1
	partTarget = 0
	w.mu.Lock()
	for _, track := range w.tracks {
		for _, seg := range track.segments {
			if seg.complete {
				if seconds := ceilDurationSeconds(seg.duration); seconds > targetSeconds {
					targetSeconds = seconds
				}
			}
		}
	}
	w.mu.Unlock()
	for _, seg := range s.Segments {
		for _, p := range seg.Parts {
			if p.Duration > partTarget {
				partTarget = p.Duration
			}
		}
	}
	if partTarget <= 0 {
		partTarget = time.Second
	}
	return targetSeconds, partTarget
}

func ceilDurationSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 1
	}
	seconds := int64(d / time.Second)
	if d%time.Second != 0 {
		seconds++
	}
	return seconds
}

func firstMSN(s Snapshot) uint64 {
	if len(s.Segments) == 0 {
		return 0
	}
	return s.Segments[0].MSN
}
