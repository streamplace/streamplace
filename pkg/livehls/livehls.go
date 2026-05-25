// Package livehls is the "muxl hls for livestreams" write side: it turns the
// muxl event stream (init + canonical segments) into a growing, appendable
// fMP4 plus a per-track byte-range index, and serves live HLS playlists.
//
// The model follows MUXL's presentation layering: the canonical segments are
// appended verbatim (so any C2PA/S2PA signature over a segment is preserved),
// and the index records each segment's absolute byte range within the fMP4.
// Playback is plain HLS — EXT-X-MAP for the init plus EXT-X-BYTERANGE per
// segment — with an increasing EXT-X-MEDIA-SEQUENCE and no EXT-X-ENDLIST until
// [Writer.Finalize] (which turns the live playlist into a VOD playlist).
//
// fMP4 is the appendable format (the flat MP4 used for finalized VOD cannot be
// written incrementally), so this is the format a live stream uses. The index
// JSON shape matches pkg/vod.Metafile so the same read side can serve it.
//
// Typical use, driven off muxl.RunMuxlSegmenter / RunMuxlSignSegment events:
//
//	w := livehls.NewWriter(fmp4File, livehls.WithWindow(6))
//	for ev := range eventCh {
//	    if err := w.Observe(ev); err != nil { ... }
//	    publish(w.MediaPlaylist(trackID, initURL, blobURL))
//	}
//	w.Finalize()
package livehls

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"stream.place/streamplace/pkg/muxl"
)

// Segment is one canonical segment's byte range within the growing fMP4.
// JSON tags match pkg/vod.MetafileSegment.
type Segment struct {
	Offset        int64  `json:"offset"`
	Size          int64  `json:"size"`
	DurationTicks uint64 `json:"durationTicks"`
	SampleCount   uint32 `json:"sampleCount"`
}

// Track is the per-track config plus its (windowed) segment index. JSON tags
// match pkg/vod.MetafileTrack so the existing playback read side can consume
// the index unchanged.
type Track struct {
	Type       string    `json:"type"`
	Codec      string    `json:"codec"`
	Timescale  uint32    `json:"timescale"`
	Segments   []Segment `json:"segments"`
	Width      uint32    `json:"width,omitempty"`
	Height     uint32    `json:"height,omitempty"`
	Channels   uint32    `json:"channels,omitempty"`
	SampleRate uint32    `json:"sampleRate,omitempty"`

	// init is the per-track ftyp+moov (EXT-X-MAP target); not serialized.
	init []byte
	// mediaSeq is the number of segments evicted from the head of the
	// playlist window — i.e. EXT-X-MEDIA-SEQUENCE.
	mediaSeq uint64
}

// InitSegment returns the per-track init (ftyp+moov) bytes for trackID, the
// EXT-X-MAP target, or nil if unknown.
func (w *Writer) InitSegment(trackID string) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t := w.tracks[trackID]; t != nil {
		return t.init
	}
	return nil
}

// Writer assembles a live MUXL fMP4 and its HLS index incrementally. It is
// safe for concurrent Observe and playlist reads.
type Writer struct {
	mu       sync.Mutex
	out      io.Writer
	running  int64 // bytes appended to out so far (= next segment offset)
	tracks   map[string]*Track
	order    []string // track ids, first-seen order
	window   int      // max segments per media playlist; 0 = keep all
	finished bool
}

// Option configures a Writer.
type Option func(*Writer)

// WithWindow keeps at most n segments per track in the live playlist (a
// sliding window), advancing EXT-X-MEDIA-SEQUENCE as older segments fall off.
// The underlying fMP4 bytes are not truncated; only the playlist window
// slides. n <= 0 keeps every segment (an "event" playlist).
func WithWindow(n int) Option {
	return func(w *Writer) { w.window = n }
}

// NewWriter returns a live HLS writer that appends the init and segments to
// out (a file, blob multipart writer, etc.).
func NewWriter(out io.Writer, opts ...Option) *Writer {
	w := &Writer{out: out, tracks: map[string]*Track{}}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Observe appends one muxl event to the fMP4 and updates the index. Events
// must arrive in stream order (init first, then segments in emission order),
// since byte offsets are computed from the cumulative bytes written.
func (w *Writer) Observe(ev *muxl.MuxlEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch ev.Type {
	case "init":
		// The combined ftyp+moov is written once at the head of the fMP4 so
		// the file is a valid, self-contained MUXL fMP4; players use the
		// per-track EXT-X-MAP init and byte-range into the segments, skipping
		// this header.
		if _, err := w.out.Write(ev.Data); err != nil {
			return fmt.Errorf("livehls: write init: %w", err)
		}
		w.running += int64(len(ev.Data))
		for tid, initBytes := range ev.TrackInits {
			t := w.track(tid)
			t.init = append([]byte(nil), initBytes...)
		}
		if ev.Catalog != nil {
			w.applyCatalog(ev.Catalog)
		}

	case "segment", "signed-segment":
		// Per-track chunks are concatenated in sorted track-id order, matching
		// the byte layout muxl emits; track that order so offsets line up. A
		// signed-segment chunk carries a leading c2pa-uuid box, which only
		// changes its length — the offset math is unchanged.
		for _, tid := range sortedKeys(ev.Tracks) {
			chunk := ev.Tracks[tid]
			if _, err := w.out.Write(chunk); err != nil {
				return fmt.Errorf("livehls: write segment (track %s): %w", tid, err)
			}
			t := w.track(tid)
			t.Segments = append(t.Segments, Segment{
				Offset:        w.running,
				Size:          int64(len(chunk)),
				DurationTicks: ev.Durations[tid],
				SampleCount:   ev.SampleCounts[tid],
			})
			w.running += int64(len(chunk))
			if w.window > 0 && len(t.Segments) > w.window {
				drop := len(t.Segments) - w.window
				t.Segments = append(t.Segments[:0:0], t.Segments[drop:]...)
				t.mediaSeq += uint64(drop)
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

// TrackIDs returns the known track ids in first-seen order.
func (w *Writer) TrackIDs() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.order...)
}

// Track returns a snapshot of the track's current index, or nil if unknown.
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

// MediaPlaylist renders the live HLS media playlist for trackID. initURL is
// the EXT-X-MAP target (the per-track init); blobURL is the URL of the growing
// fMP4 that the EXT-X-BYTERANGE entries address. Returns "" for an unknown
// track.
func (w *Writer) MediaPlaylist(trackID, initURL, blobURL string) string {
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

	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", target)
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", t.mediaSeq)
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	if w.finished {
		b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	}
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=%q\n", initURL)
	for _, s := range t.Segments {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n#EXT-X-BYTERANGE:%d@%d\n%s\n", s.seconds(t.Timescale), s.Size, s.Offset, blobURL)
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

	var audioCodec string
	haveAudio := false
	for _, tid := range w.order {
		t := w.tracks[tid]
		if t.Type == "audio" {
			haveAudio = true
			if audioCodec == "" {
				audioCodec = t.Codec
			}
			fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=%q,NAME=%q,DEFAULT=YES,AUTOSELECT=YES,CHANNELS=%q,URI=%q\n",
				"audio", t.Codec, strconv.Itoa(int(maxU32(t.Channels, 2))), trackURL(tid))
		}
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

// applyCatalog fills per-track config from the muxl catalog, mirroring
// vod.metafileBuilder.Finalize.
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
		bytes += s.Size
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
