// Package livehls is the in-memory "muxl HLS for livestreams" window: it turns
// the muxl event stream (init + canonical segments) into a sliding window of
// recent segments held in memory, plus the HLS playlists that address them.
//
// Nothing is written to the filesystem. The canonical .m4s segments are the
// durable unit (archived elsewhere, by ValidateMP4); this window only keeps
// the most recent few for live playback and serves them straight from RAM:
//
//   - per-track init segment (ftyp+moov) — the EXT-X-MAP target
//   - a windowed ring of recent canonical segments, addressed by a monotonic
//     media-sequence number
//
// The segment bytes are appended verbatim, so any C2PA/S2PA signature over a
// segment is preserved and a player fetches exactly what was signed. Playback
// is plain fMP4 HLS — EXT-X-MAP for the init plus one URI per segment — with
// an advancing EXT-X-MEDIA-SEQUENCE and no EXT-X-ENDLIST until
// [Writer.Finalize] (which turns the live playlist into a VOD playlist).
//
// Typical use, driven off the muxl event stream a stream's segments arrive on
// (locally signed or replicated from another node):
//
//	w := livehls.NewWriter(livehls.WithWindow(6))
//	w.Observe(initEvent)
//	for ev := range segmentEvents {
//	    w.Observe(ev)
//	}
//	// serve w.MasterPlaylist(...), w.MediaPlaylist(...), w.InitSegment(tid),
//	// w.SegmentData(tid, seq) over HTTP.
package livehls

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"stream.place/streamplace/pkg/muxl"
)

// Segment is one canonical segment held in the live window: a track's
// verbatim [c2pa?][muxl][moof][mdat] bytes plus the timing the playlist needs.
type Segment struct {
	// Seq is the monotonic media-sequence number, assigned in arrival order
	// and never reused — the value a player addresses the segment by and the
	// basis for EXT-X-MEDIA-SEQUENCE.
	Seq           uint64
	DurationTicks uint64
	SampleCount   uint32
	data          []byte
}

// Size is the segment's byte length.
func (s Segment) Size() int { return len(s.data) }

// Track is the per-track config plus its (windowed) in-memory segments.
type Track struct {
	Type       string
	Codec      string
	Timescale  uint32
	Width      uint32
	Height     uint32
	Channels   uint32
	SampleRate uint32

	Segments []Segment

	// init is the per-track ftyp+moov (EXT-X-MAP target); not part of the
	// window. nextSeq is the next media-sequence number to hand out.
	init    []byte
	nextSeq uint64
}

// Writer assembles an in-memory live HLS window from the muxl event stream.
// Safe for concurrent Observe and read/serve.
type Writer struct {
	mu       sync.Mutex
	tracks   map[string]*Track
	order    []string // track ids, sorted
	window   int      // max segments retained per track; 0 = keep all
	finished bool
}

// Option configures a Writer.
type Option func(*Writer)

// WithWindow keeps at most n segments per track in memory (a sliding window),
// advancing EXT-X-MEDIA-SEQUENCE as older segments are evicted. n <= 0 keeps
// every segment (an "event"/VOD-style window — unbounded memory, avoid for
// 24/7 streams).
func WithWindow(n int) Option {
	return func(w *Writer) { w.window = n }
}

// NewWriter returns an empty in-memory live HLS window.
func NewWriter(opts ...Option) *Writer {
	w := &Writer{tracks: map[string]*Track{}}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Observe folds one muxl event into the window. An "init" event supplies the
// per-track init segments and catalog; "segment"/"signed-segment" events
// append each track's canonical-segment bytes (copied — the caller may reuse
// its buffer) and evict beyond the window.
func (w *Writer) Observe(ev *muxl.MuxlEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch ev.Type {
	case "init":
		for tid, initBytes := range ev.TrackInits {
			w.track(tid).init = append([]byte(nil), initBytes...)
		}
		if ev.Catalog != nil {
			w.applyCatalog(ev.Catalog)
		}

	case "segment", "signed-segment":
		for _, tid := range sortedKeys(ev.Tracks) {
			t := w.track(tid)
			t.Segments = append(t.Segments, Segment{
				Seq:           t.nextSeq,
				DurationTicks: ev.Durations[tid],
				SampleCount:   ev.SampleCounts[tid],
				data:          append([]byte(nil), ev.Tracks[tid]...),
			})
			t.nextSeq++
			if w.window > 0 && len(t.Segments) > w.window {
				drop := len(t.Segments) - w.window
				t.Segments = append(t.Segments[:0:0], t.Segments[drop:]...)
			}
		}

	default:
		return fmt.Errorf("livehls: unexpected event type %q", ev.Type)
	}
	return nil
}

// Finalize marks the stream complete; subsequent media playlists carry
// EXT-X-ENDLIST and the VOD playlist type.
func (w *Writer) Finalize() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.finished = true
}

// TrackIDs returns the known track ids in sorted order.
func (w *Writer) TrackIDs() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.order...)
}

// Track returns a snapshot of the track's current windowed segments (segment
// byte payloads are shared, not copied), or nil if unknown.
func (w *Writer) Track(trackID string) *Track {
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.tracks[trackID]
	if t == nil {
		return nil
	}
	cp := *t
	cp.Segments = append([]Segment(nil), t.Segments...)
	cp.init = nil
	return &cp
}

// InitSegment returns the per-track init (ftyp+moov) bytes — the EXT-X-MAP
// target — or nil if unknown.
func (w *Writer) InitSegment(trackID string) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t := w.tracks[trackID]; t != nil {
		return t.init
	}
	return nil
}

// SegmentData returns the bytes of the segment with media-sequence seq for
// trackID, or nil if it's unknown or has already slid out of the window.
func (w *Writer) SegmentData(trackID string, seq uint64) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.tracks[trackID]
	if t == nil {
		return nil
	}
	for _, s := range t.Segments {
		if s.Seq == seq {
			return s.data
		}
	}
	return nil
}

// MediaPlaylist renders the live HLS media playlist for trackID. initURL is
// the EXT-X-MAP target (the per-track init); segURI maps a segment's
// media-sequence number to its URI. Returns "" for an unknown track.
func (w *Writer) MediaPlaylist(trackID, initURL string, segURI func(seq uint64) string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.tracks[trackID]
	if t == nil {
		return ""
	}

	maxDur := 0.0
	for _, s := range t.Segments {
		if d := s.seconds(t.Timescale); d > maxDur {
			maxDur = d
		}
	}
	target := int(math.Ceil(maxDur))
	if target < 1 {
		target = 1
	}
	mediaSeq := uint64(0)
	if len(t.Segments) > 0 {
		mediaSeq = t.Segments[0].Seq
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", target)
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", mediaSeq)
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	if w.finished {
		b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	}
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=%q\n", initURL)
	for _, s := range t.Segments {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n%s\n", s.seconds(t.Timescale), segURI(s.Seq))
	}
	if w.finished {
		b.WriteString("#EXT-X-ENDLIST\n")
	}
	return b.String()
}

// MasterPlaylist renders the HLS master playlist. trackURL maps a track id to
// its media-playlist URL. Audio tracks become EXT-X-MEDIA renditions in the
// "audio" group; video tracks become EXT-X-STREAM-INF variants.
func (w *Writer) MasterPlaylist(trackURL func(trackID string) string) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-INDEPENDENT-SEGMENTS\n")

	// Pick the primary audio rendition: prefer AAC (broadest HLS support —
	// Safari has no Opus) so it's the default; any other codec (Opus) stays a
	// selectable alternate. A segment may carry both after codec completion.
	primaryAudio := ""
	for _, tid := range w.order {
		if t := w.tracks[tid]; t.Type == "audio" {
			if primaryAudio == "" {
				primaryAudio = tid
			}
			if strings.HasPrefix(t.Codec, "mp4a") {
				primaryAudio = tid
				break
			}
		}
	}
	var audioCodec string
	haveAudio := primaryAudio != ""
	for _, tid := range w.order {
		t := w.tracks[tid]
		if t.Type != "audio" {
			continue
		}
		def := "NO"
		if tid == primaryAudio {
			def = "YES"
			audioCodec = t.Codec
		}
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=%q,NAME=%q,DEFAULT=%s,AUTOSELECT=YES,CHANNELS=%q,URI=%q\n",
			"audio", t.Codec, def, strconv.Itoa(int(maxU32(t.Channels, 2))), trackURL(tid))
	}
	for _, tid := range w.order {
		t := w.tracks[tid]
		if t.Type != "video" {
			continue
		}
		codecs := t.Codec
		if audioCodec != "" {
			codecs = t.Codec + "," + audioCodec
		}
		bw := t.peakBitrate()
		if haveAudio {
			fmt.Fprintf(&b, "#EXT-X-STREAM-INF:AUDIO=%q,BANDWIDTH=%d,CODECS=%q,RESOLUTION=%dx%d\n", "audio", bw, codecs, t.Width, t.Height)
		} else {
			fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,CODECS=%q,RESOLUTION=%dx%d\n", bw, codecs, t.Width, t.Height)
		}
		b.WriteString(trackURL(tid) + "\n")
	}
	return b.String()
}

// --- internals ---

func (w *Writer) track(tid string) *Track {
	t := w.tracks[tid]
	if t == nil {
		t = &Track{Type: "unknown"}
		w.tracks[tid] = t
		w.order = append(w.order, tid)
		sort.Strings(w.order)
	}
	return t
}

// applyCatalog fills per-track config from the muxl catalog.
func (w *Writer) applyCatalog(cat *muxl.MuxlCatalog) {
	if cat.Video != nil {
		for _, c := range cat.Video.Renditions {
			t := w.track(strconv.FormatUint(uint64(c.TrackID()), 10))
			t.Type = "video"
			t.Codec = c.Codec
			t.Timescale = c.Timescale()
			t.Width = c.CodedWidth
			t.Height = c.CodedHeight
		}
	}
	if cat.Audio != nil {
		for _, c := range cat.Audio.Renditions {
			t := w.track(strconv.FormatUint(uint64(c.TrackID()), 10))
			t.Type = "audio"
			t.Codec = c.Codec
			t.Timescale = c.Timescale()
			t.Channels = c.NumberOfChannels
			t.SampleRate = c.SampleRate
		}
	}
}

func (s Segment) seconds(timescale uint32) float64 {
	if timescale == 0 {
		return 0
	}
	return float64(s.DurationTicks) / float64(timescale)
}

// peakBitrate estimates BANDWIDTH (bits/sec) from the windowed segments.
func (t *Track) peakBitrate() int {
	var bytes int64
	var ticks uint64
	for _, s := range t.Segments {
		bytes += int64(s.Size())
		ticks += s.DurationTicks
	}
	if ticks == 0 || t.Timescale == 0 {
		return 0
	}
	secs := float64(ticks) / float64(t.Timescale)
	return int(float64(bytes*8) / secs)
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func maxU32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}
