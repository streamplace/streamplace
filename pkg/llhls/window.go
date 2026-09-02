// Package llhls contains the bounded in-memory state used by the LL-HLS
// origin. It deliberately knows nothing about GStreamer or HTTP.
package llhls

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrStalePresentation = errors.New("llhls: stale presentation")
	ErrPartOrder         = errors.New("llhls: invalid part order")
	ErrGeneration        = errors.New("llhls: stale configuration generation")
	ErrPartUnavailable   = errors.New("llhls: part unavailable")
	ErrWindowCapacity    = errors.New("llhls: window capacity exceeded")
)

const (
	// The muxer durations are scheduling goals. AAC frame boundaries and GOP
	// drain can make emitted media slightly longer, while playlist duration
	// tags must remain fixed for the presentation and cover every emitted item.
	defaultTargetDuration = 6 * time.Second
	defaultPartTarget     = 1100 * time.Millisecond
	partHoldBackMargin    = time.Millisecond
	// Keep enough completed history for hold-back playback when a sibling track
	// drives the shared byte budget over its limit.
	minRetainedSegments = 12
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
	Identity        PartIdentity
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
	Independent     bool
	ProgramDateTime time.Time
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

type AudioConfig struct {
	Channels int
}

// PartIdentity is the stable logical identity used by a part URI. Parent
// timing metadata may be corrected when a parent closes, but this identity is
// never reassigned or aliased to another parent's bytes.
type PartIdentity struct {
	MSN   uint64
	Index uint32
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
	audioConfig          AudioConfig
	programDateTime      time.Time
	programDateTimeStart time.Duration
	targetDuration       int64
	partTarget           time.Duration
	configuredTarget     int64
	configuredPartTarget time.Duration
	completionHold       time.Duration
}

type track struct {
	generation uint64
	init       []byte
	segments   []*segment
	ended      bool
	bytes      int
}

type segment struct {
	msn             uint64
	start, duration time.Duration
	parts           []*part
	complete        bool
	closing         bool
	independent     bool
	programDateTime time.Time
	data            []byte
}

type part struct {
	identity        PartIdentity
	start, duration time.Duration
	independent     bool
	data            []byte
}

type Option func(*Window)

func WithMaxSegments(n int) Option { return func(w *Window) { w.maxSegments = n } }
func WithMaxBytes(n int) Option    { return func(w *Window) { w.maxBytes = n } }

// WithSegmentCompletionDelay delays parent completion visibility by d. This
// gives blocking reloads time to observe the final part of an open segment.
func WithSegmentCompletionDelay(d time.Duration) Option {
	return func(w *Window) { w.completionHold = d }
}

// WithPlaylistDurations sets fixed upper bounds for parent and part durations.
// The parent bound is rounded to the nearest whole second for TARGETDURATION.
// Both values remain fixed for each presentation observed by the Window.
func WithPlaylistDurations(parent, part time.Duration) Option {
	return func(w *Window) {
		if parent > 0 {
			w.targetDuration = roundedDurationSeconds(parent)
		}
		if part > 0 {
			w.partTarget = part
		}
	}
}

func NewWindow(opts ...Option) *Window {
	w := &Window{
		tracks:         make(map[string]*track),
		changed:        make(chan struct{}),
		targetDuration: roundedDurationSeconds(defaultTargetDuration),
		partTarget:     defaultPartTarget,
	}
	for _, opt := range opts {
		opt(w)
	}
	w.configuredTarget = w.targetDuration
	w.configuredPartTarget = w.partTarget
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
		w.audioConfig = AudioConfig{}
		w.programDateTime = time.Time{}
		w.programDateTimeStart = 0
		w.targetDuration = w.configuredTarget
		w.partTarget = w.configuredPartTarget
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
			w.completeClosingSegments(t, ev.MSN)
			s = w.findOrCreateSegment(t, ev)
		}
		if ev.Part != uint32(len(s.parts)) {
			return ErrPartOrder
		}
		if ev.Part == 0 {
			s.independent = ev.Independent
			if !ev.ProgramDateTime.IsZero() {
				s.programDateTime = ev.ProgramDateTime
			}
		}
		if w.programDateTime.IsZero() && !ev.ProgramDateTime.IsZero() {
			w.programDateTime = ev.ProgramDateTime
			w.programDateTimeStart = ev.Start
		}
		p := &part{identity: PartIdentity{MSN: ev.MSN, Index: ev.Part}, start: ev.Start, duration: ev.Duration, independent: ev.Independent, data: append([]byte(nil), ev.Data...)}
		s.parts = append(s.parts, p)
		w.bytes += len(p.data)
		t.bytes += len(p.data)
	case SegmentComplete:
		s := w.findSegment(t, ev.MSN)
		if s == nil {
			return fmt.Errorf("llhls: segment %d has no parts", ev.MSN)
		}
		if s.complete || s.closing {
			return nil
		}
		s.start, s.duration = ev.Start, ev.Duration
		s.data = append([]byte(nil), ev.Data...)
		w.bytes += len(s.data)
		t.bytes += len(s.data)
		if w.completionHold <= 0 {
			s.complete = true
		} else {
			s.closing = true
			w.scheduleCompletion(t, ev.Track, s)
		}
	case Discontinuity:
		w.removeTrackBytes(t)
		t.segments = nil
		t.bytes = 0
		t.ended = false
	case SessionEnd:
		t.ended = true
	default:
		return fmt.Errorf("llhls: unknown event kind %d", ev.Kind)
	}
	w.evict()
	w.notify()
	if w.maxBytes > 0 && w.hasOverCapacityOpenSegment() {
		return ErrWindowCapacity
	}
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

// scheduleCompletion flips a closing segment to complete after the hold. The
// timer re-validates under the lock: a presentation reset or discontinuity
// may have dropped the segment (or the whole track) in the meantime.
func (w *Window) scheduleCompletion(t *track, trackID string, s *segment) {
	time.AfterFunc(w.completionHold, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.tracks[trackID] != t {
			return
		}
		if w.findSegment(t, s.msn) != s || s.complete {
			return
		}
		s.closing = false
		s.complete = true
		w.evict()
		w.notify()
	})
}

func (w *Window) completeClosingSegments(t *track, nextMSN uint64) {
	for _, s := range t.segments {
		if s.closing && s.msn < nextMSN {
			s.closing = false
			s.complete = true
		}
	}
}

func (w *Window) evict() {
	for _, t := range w.tracks {
		for len(t.segments) > 0 {
			// Incomplete segments are still receiving part events; evicting
			// one strands those events and fails the ingest stream. Small
			// tracks would otherwise be emptied wholesale while a large
			// track keeps the byte budget exceeded.
			if !t.segments[0].complete {
				break
			}
			overCount := w.maxSegments > 0 && len(t.segments) > w.maxSegments
			overBytes := w.maxBytes > 0 && w.bytes > w.maxBytes
			if !overCount && !overBytes {
				break
			}
			// Retained history for hold-back playback is only shed when this
			// track alone exceeds the byte budget.
			if !overCount && len(t.segments) <= minRetainedSegments && t.bytes <= w.maxBytes {
				break
			}
			w.removeSegmentBytes(t, t.segments[0])
			t.segments = t.segments[1:]
		}
	}
}
func (w *Window) removeTrackBytes(t *track) {
	for _, s := range t.segments {
		w.removeSegmentBytes(t, s)
	}
	t.bytes = 0
}
func (w *Window) removeSegmentBytes(t *track, s *segment) {
	bytes := segmentBytes(s)
	w.bytes -= bytes
	t.bytes -= bytes
}

func segmentBytes(s *segment) int {
	bytes := len(s.data)
	for _, p := range s.parts {
		bytes += len(p.data)
	}
	return bytes
}

func (w *Window) hasOverCapacityOpenSegment() bool {
	for _, t := range w.tracks {
		for _, s := range t.segments {
			if !s.complete && !s.closing && segmentBytes(s) > w.maxBytes {
				return true
			}
		}
	}
	return false
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

func (w *Window) SetAudioConfig(config AudioConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.audioConfig = config
}

func (w *Window) AudioConfig() AudioConfig {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.audioConfig
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
		ss := SegmentSnapshot{MSN: seg.msn, Start: seg.start, Duration: seg.duration, Complete: seg.complete, Independent: seg.independent, ProgramDateTime: seg.programDateTime, Data: append([]byte(nil), seg.data...)}
		for _, p := range seg.parts {
			ss.Parts = append(ss.Parts, PartSnapshot{Identity: p.identity, Index: p.identity.Index, Start: p.start, Duration: p.duration, Independent: p.independent, Data: append([]byte(nil), p.data...)})
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
			if uint64(len(s.parts)) > uint64(partIndex) && s.parts[partIndex].identity == (PartIdentity{MSN: msn, Index: partIndex}) {
				return append([]byte(nil), s.parts[partIndex].data...)
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

// Wait blocks a playlist reload until the requested reload point is reflected
// in a newer playlist. For a completed parent, a part index beyond the final
// part rolls over to part zero of the following parent, per HLS blocking
// reload semantics. This rule intentionally does not apply to media lookups.
func (w *Window) Wait(ctx context.Context, presentation, trackID string, msn uint64, partIndex uint32) error {
	for {
		w.mu.Lock()
		ready := w.playlistReloadReadyLocked(presentation, trackID, msn, partIndex)
		changed := w.changed
		w.mu.Unlock()
		if ready {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitForPart blocks until the exact (MSN, part index) is available. A part
// that belongs to a completed or evicted parent is permanently unavailable;
// callers should serve ErrPartUnavailable as a no-store 404. Part identity is
// the immutable tuple of presentation, track, MSN, and local part index.
func (w *Window) WaitForPart(ctx context.Context, presentation, trackID string, msn uint64, partIndex uint32) error {
	for {
		w.mu.Lock()
		state := w.partStateLocked(presentation, trackID, msn, partIndex)
		changed := w.changed
		w.mu.Unlock()
		switch state {
		case partReady:
			return nil
		case partUnavailable:
			return ErrPartUnavailable
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type partState uint8

const (
	partPending partState = iota
	partReady
	partUnavailable
)

func (w *Window) playlistReloadReadyLocked(presentation, trackID string, msn uint64, partIndex uint32) bool {
	if presentation != w.presentation {
		return false
	}
	t := w.tracks[trackID]
	if t == nil {
		return false
	}
	if t.ended {
		return true
	}
	if s := w.findSegment(t, msn); s != nil {
		if uint64(len(s.parts)) > uint64(partIndex) {
			return true
		}
		if !s.complete {
			return false
		}
		// A part past the end of a completed parent is a blocking reload for
		// part zero of the following parent. Do not resolve until that part
		// exists, so the response can advertise the same URI it will serve.
		if partIndex >= uint32(len(s.parts)) {
			next := w.findSegment(t, msn+1)
			return next != nil && len(next.parts) > 0
		}
	}
	if len(t.segments) == 0 {
		return false
	}
	first := t.segments[0].msn
	if first > msn+1 {
		return true
	}
	if first == msn+1 && partIndex > 0 {
		return len(t.segments[0].parts) > 0
	}
	for _, seg := range t.segments {
		if seg.msn > msn {
			return true
		}
	}
	return false
}

func (w *Window) partStateLocked(presentation, trackID string, msn uint64, partIndex uint32) partState {
	if presentation != w.presentation {
		return partUnavailable
	}
	t := w.tracks[trackID]
	if t == nil || t.ended {
		return partUnavailable
	}
	s := w.findSegment(t, msn)
	if s == nil {
		if len(t.segments) == 0 {
			return partPending
		}
		if t.segments[0].msn > msn {
			return partUnavailable
		}
		for _, candidate := range t.segments {
			if candidate.msn > msn {
				return partUnavailable
			}
		}
		return partPending
	}
	if uint64(len(s.parts)) > uint64(partIndex) {
		return partReady
	}
	if s.complete {
		return partUnavailable
	}
	return partPending
}

// Playlist renders the media playlist for one track. URIs are supplied by the
// caller so routing and presentation identifiers remain outside this package.
// When renditionURI is non-nil, an EXT-X-RENDITION-REPORT is emitted for
// every other track that has published media (required for LL-HLS).
func (w *Window) Playlist(presentation, trackID string, partURI func(uint64, uint32) string, segmentURI func(uint64) string, initURI string, renditionURI func(string) string) string {
	return w.playlist(presentation, trackID, partURI, segmentURI, initURI, renditionURI, playlistWithParts)
}

// PlaylistSegmentsOnly renders a media playlist using only completed parents.
func (w *Window) PlaylistSegmentsOnly(presentation, trackID string, partURI func(uint64, uint32) string, segmentURI func(uint64) string, initURI string, renditionURI func(string) string) string {
	return w.playlist(presentation, trackID, partURI, segmentURI, initURI, renditionURI, playlistSegmentsOnly)
}

type playlistMode uint8

const (
	playlistWithParts playlistMode = iota
	playlistSegmentsOnly
)

func (w *Window) playlist(presentation, trackID string, partURI func(uint64, uint32) string, segmentURI func(uint64) string, initURI string, renditionURI func(string) string, mode playlistMode) string {
	s := w.Snapshot(presentation, trackID)
	if s.Track == "" {
		return ""
	}
	includeParts := mode == playlistWithParts
	targetSeconds, partTarget := w.playlistDurations()
	partHoldBack := 3*partTarget + partHoldBackMargin
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:10\n#EXT-X-TARGETDURATION:%d\n", targetSeconds)
	if includeParts {
		fmt.Fprintf(&b, "#EXT-X-PART-INF:PART-TARGET=%.6f\n", partTarget.Seconds())
	}
	if trackID == "video" && allSegmentsIndependent(s.Segments) {
		b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	}
	if includeParts {
		fmt.Fprintf(&b, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=%.6f,HOLD-BACK=%.6f\n", partHoldBack.Seconds(), 3*float64(targetSeconds))
	}
	if includeParts && renditionURI != nil {
		for _, rep := range w.renditionReports(presentation, trackID) {
			fmt.Fprintf(&b, "#EXT-X-RENDITION-REPORT:URI=%q,LAST-MSN=%d", renditionURI(rep.trackID), rep.lastMSN)
			if rep.lastPart >= 0 {
				fmt.Fprintf(&b, ",LAST-PART=%d", rep.lastPart)
			}
			b.WriteByte('\n')
		}
	}
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n#EXT-X-MAP:URI=%q\n", firstMSN(s), initURI)
	openMSN := uint64(0)
	if len(s.Segments) > 0 {
		if last := s.Segments[len(s.Segments)-1]; !last.Complete {
			openMSN = last.MSN
		}
	}
	for _, seg := range s.Segments {
		if includeParts && !seg.Complete && seg.MSN == openMSN {
			for _, p := range seg.Parts {
				fmt.Fprintf(&b, "#EXT-X-PART:DURATION=%.6f,URI=%q", p.Duration.Seconds(), partURI(p.Identity.MSN, p.Identity.Index))
				if p.Independent {
					b.WriteString(",INDEPENDENT=YES")
				}
				b.WriteByte('\n')
			}
		}
		if seg.Complete {
			programDateTime := seg.ProgramDateTime
			if programDateTime.IsZero() && !s.ProgramDateTime.IsZero() {
				programDateTime = s.ProgramDateTime.Add(seg.Start - s.ProgramDateTimeStart)
			}
			if !programDateTime.IsZero() {
				fmt.Fprintf(&b, "#EXT-X-PROGRAM-DATE-TIME:%s\n", programDateTime.Format(time.RFC3339Nano))
			}
			fmt.Fprintf(&b, "#EXTINF:%.6f,\n%s\n", seg.Duration.Seconds(), segmentURI(seg.MSN))
		}
	}
	if includeParts && !s.Ended && len(s.Segments) > 0 {
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

func allSegmentsIndependent(segments []SegmentSnapshot) bool {
	if len(segments) == 0 {
		return false
	}
	for _, segment := range segments {
		if !segment.Independent {
			return false
		}
	}
	return true
}

func (w *Window) playlistDurations() (targetSeconds int64, partTarget time.Duration) {
	w.mu.Lock()
	targetSeconds, partTarget = w.targetDuration, w.partTarget
	w.mu.Unlock()
	return targetSeconds, partTarget
}

type renditionReport struct {
	trackID  string
	lastMSN  uint64
	lastPart int32
}

// renditionReports describes the latest published state of every track other
// than exclude. A track with an open segment reports that segment's MSN and
// its last published part; a fully completed tail reports its last MSN
// without a part (completed parents carry no listed partial segments).
func (w *Window) renditionReports(presentation, exclude string) []renditionReport {
	w.mu.Lock()
	defer w.mu.Unlock()
	if presentation != w.presentation {
		return nil
	}
	var reports []renditionReport
	for trackID, t := range w.tracks {
		if trackID == exclude || len(t.segments) == 0 {
			continue
		}
		last := t.segments[len(t.segments)-1]
		if !last.complete {
			if len(last.parts) == 0 {
				if len(t.segments) < 2 {
					continue
				}
				last = t.segments[len(t.segments)-2]
			} else {
				reports = append(reports, renditionReport{trackID: trackID, lastMSN: last.msn, lastPart: int32(len(last.parts) - 1)})
				continue
			}
		}
		reports = append(reports, renditionReport{trackID: trackID, lastMSN: last.msn, lastPart: -1})
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].trackID < reports[j].trackID })
	return reports
}

func roundedDurationSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 1
	}
	seconds := int64(math.Round(d.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func firstMSN(s Snapshot) uint64 {
	if len(s.Segments) == 0 {
		return 0
	}
	return s.Segments[0].MSN
}
