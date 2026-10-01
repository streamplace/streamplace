package livehls

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/webvtt"
	"stream.place/streamplace/pkg/muxl"
)

// The live subtitle rendition.
//
// Caption tracks are WebVTT renditions of the master playlist's SubtitlesGroup.
// A subtitle media playlist mirrors a reference video track of the window: the
// same media-sequence numbers and durations, one WebVTT document per video
// segment holding every final cue that overlaps the segment's time range.
//
// Timeline. Cue times are wall-clock (the clock of the segment startTime), and
// a video segment i starts at wall time Start_i with its first sample at media
// time tfdt_i. Cue text in segment documents counts seconds from the window's
// epoch E (the Start of the first segment it observed), so one cue has the same
// timestamps in every segment document it appears in (players deduplicate such
// repeats), and every document carries
//
//	X-TIMESTAMP-MAP=MPEGTS:<m_i>,LOCAL:00:00:00.000
//	m_i = 90000 * tfdt_i/timescale - 90000 * (Start_i - E)      (mod 2^33)
//
// so cue time c plays at media time m_i + 90000*c, which for a cue at wall time
// T is 90000 * (tfdt_i/timescale + (T - Start_i)): exactly where the video
// frame at that wall time sits. m_i is the same for every segment as long as
// tfdt and the wall clock advance together; computing it per segment keeps
// captions on the video across wall-clock jitter, gaps, and reconnects (a
// reconnect restarts tfdt near zero, and m_i follows).

// SubtitlesGroup is the GROUP-ID of the subtitle renditions.
const SubtitlesGroup = "cc"

// SubtitleRendition is one caption track in the master playlist.
type SubtitleRendition struct {
	Name     string
	Language string
	URI      string
}

// MediaLine is the rendition's EXT-X-MEDIA line.
func (r SubtitleRendition) MediaLine() string {
	return fmt.Sprintf("#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=%q,NAME=%q,LANGUAGE=%q,AUTOSELECT=YES,DEFAULT=NO,URI=%q",
		SubtitlesGroup, attrString(r.Name), attrString(r.Language), r.URI)
}

// attrString makes s safe inside an HLS quoted-string: they cannot hold a
// double quote or a line break.
func attrString(s string) string {
	return strings.NewReplacer("\"", "'", "\r", " ", "\n", " ").Replace(s)
}

// SubtitleSegment is one video segment as the subtitle rendition sees it.
type SubtitleSegment struct {
	// Seq is the video segment's media-sequence number, also the subtitle
	// segment's.
	Seq uint64
	// Start and End are the segment's wall-clock time range, [Start, End).
	Start, End time.Time
	Duration   time.Duration
	// MPEGTS is the X-TIMESTAMP-MAP value of the segment's document.
	MPEGTS uint64
	// Discontinuity marks a segment that starts a new media timeline (VOD
	// only; see VODSubtitleWindow).
	Discontinuity bool
}

// SubtitleWindow is the ready part of the subtitle rendition: the segments a
// subtitle media playlist lists.
type SubtitleWindow struct {
	// Epoch is the wall-clock time cue offsets count from.
	Epoch time.Time
	// Segments are in sequence order; their sequence numbers are the video
	// track's, and contiguous.
	Segments []SubtitleSegment
	// Ended is set once the window is finalized: the playlist is complete.
	Ended bool
	// DiscontinuitySequence is EXT-X-DISCONTINUITY-SEQUENCE (VOD only).
	DiscontinuitySequence int
}

// subtitleReference picks the track the subtitle rendition mirrors: the first
// video track that has wall-clock timing, else the first audio track.
// Caller holds w.mu.
func (w *Writer) subtitleReference() *Track {
	var audio *Track
	for _, tid := range w.order {
		t := w.tracks[tid]
		if t == nil || t.Timescale == 0 || !t.timed() {
			continue
		}
		switch t.Type {
		case "video":
			return t
		case "audio":
			if audio == nil {
				audio = t
			}
		}
	}
	return audio
}

// timed reports whether the track's segments carry wall-clock start times.
func (t *Track) timed() bool {
	for _, s := range t.Segments {
		if !s.Start.IsZero() {
			return true
		}
	}
	return false
}

// SubtitleEpoch is the wall-clock time subtitle cue offsets count from, or
// the zero time when the window has no timing yet.
func (w *Writer) SubtitleEpoch() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.epoch
}

// SubtitleWindow returns the subtitle rendition's playlist window for one
// caption track.
//
// The caption track runs behind the video by the recognition latency, so a
// video segment is listed only once its captions can be trusted: it has been
// in the window for at least budget (the caption latency budget), or the
// final callback reports that the track has already published final cues past
// the segment's end (it gets the segment's start and end). Listing stops at the last such segment, so the playlist
// ends at the last complete segment and a player's subtitle buffer never
// waits on cues that will not arrive: it only ever gets segments it can
// parse in full. A finalized window lists everything. final may be nil.
func (w *Writer) SubtitleWindow(budget time.Duration, final func(start, end time.Time) bool) SubtitleWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.evictExpired(w.now())
	win := SubtitleWindow{Epoch: w.epoch, Ended: w.finished}
	t := w.subtitleReference()
	if t == nil {
		return win
	}
	segs := w.advertised(t)
	last := -1
	for i := len(segs) - 1; i >= 0; i-- {
		if w.subtitleReady(t, segs[i], budget, final) {
			last = i
			break
		}
	}
	for _, s := range segs[:last+1] {
		if s.Start.IsZero() {
			continue
		}
		win.Segments = append(win.Segments, w.subtitleSegment(t, s))
	}
	return win
}

// SubtitleSegment looks up one subtitle segment by sequence number. ok is
// false when the sequence is unknown, aged out, or not ready (see
// SubtitleWindow). Like video segments, one that has left the playlist stays
// available until it ages out.
func (w *Writer) SubtitleSegment(seq uint64, budget time.Duration, final func(start, end time.Time) bool) (seg SubtitleSegment, epoch time.Time, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.evictExpired(w.now())
	t := w.subtitleReference()
	if t == nil {
		return SubtitleSegment{}, time.Time{}, false
	}
	for _, s := range t.Segments {
		if s.Seq != seq || s.open || s.Start.IsZero() {
			continue
		}
		if !w.subtitleReady(t, s, budget, final) {
			return SubtitleSegment{}, time.Time{}, false
		}
		return w.subtitleSegment(t, s), w.epoch, true
	}
	return SubtitleSegment{}, time.Time{}, false
}

// subtitleReady is the readiness rule of SubtitleWindow. Caller holds w.mu.
func (w *Writer) subtitleReady(t *Track, s Segment, budget time.Duration, final func(start, end time.Time) bool) bool {
	if w.finished || w.now().Sub(s.addedAt) >= budget {
		return true
	}
	if final == nil || s.Start.IsZero() {
		return false
	}
	d := ticksDuration(s.DurationTicks, t.Timescale)
	return final(s.Start, s.Start.Add(d))
}

// subtitleSegment derives a segment's subtitle timing. Caller holds w.mu.
func (w *Writer) subtitleSegment(t *Track, s Segment) SubtitleSegment {
	d := ticksDuration(s.DurationTicks, t.Timescale)
	return SubtitleSegment{
		Seq:      s.Seq,
		Start:    s.Start,
		End:      s.Start.Add(d),
		Duration: d,
		MPEGTS:   TimestampMapMPEGTS(s.FirstDecode, t.Timescale, s.Start, w.epoch),
	}
}

// TimestampMapMPEGTS is the MPEGTS value of a segment's X-TIMESTAMP-MAP
// (LOCAL:00:00:00.000) when cue times count from epoch: the 90kHz media time
// of the segment's first sample (tfdt in the track's timescale), less the 90kHz
// distance of the segment's start from epoch, wrapped to 33 bits.
func TimestampMapMPEGTS(tfdt uint64, timescale uint32, start, epoch time.Time) uint64 {
	if timescale == 0 {
		return 0
	}
	ts := uint64(timescale)
	media := int64(tfdt/ts*90000 + tfdt%ts*90000/ts)
	d := start.Sub(epoch)
	offset := int64(d/time.Second)*90000 + int64(d%time.Second)*9/100000
	m := (media - offset) % webvtt.MPEGTSModulus
	if m < 0 {
		m += webvtt.MPEGTSModulus
	}
	return uint64(m)
}

func ticksDuration(ticks uint64, timescale uint32) time.Duration {
	if timescale == 0 {
		return 0
	}
	ts := uint64(timescale)
	return time.Duration(ticks/ts)*time.Second + time.Duration(ticks%ts*uint64(time.Second)/ts)
}

// Playlist renders the subtitle media playlist for the window. segURI maps a
// segment's media-sequence number to its URI.
func (win SubtitleWindow) Playlist(segURI func(seq uint64) string) string {
	maxDur := 0.0
	for _, s := range win.Segments {
		maxDur = math.Max(maxDur, s.Duration.Seconds())
	}
	target := int(math.Ceil(maxDur))
	if target < 1 {
		target = 1
	}
	mediaSeq := uint64(0)
	if len(win.Segments) > 0 {
		mediaSeq = win.Segments[0].Seq
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:6\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", target)
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", mediaSeq)
	if win.DiscontinuitySequence > 0 {
		fmt.Fprintf(&b, "#EXT-X-DISCONTINUITY-SEQUENCE:%d\n", win.DiscontinuitySequence)
	}
	if win.Ended {
		b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	}
	for i, s := range win.Segments {
		if s.Discontinuity && i > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n%s\n", s.Duration.Seconds(), segURI(s.Seq))
	}
	if win.Ended {
		b.WriteString("#EXT-X-ENDLIST\n")
	}
	return b.String()
}

// SegmentVTT renders a subtitle segment's WebVTT document from the final cues
// that overlap it (captions.Hub.Cues over [seg.Start, seg.End)). Cues keep
// their identity and full extent in every document they overlap, so a cue
// spanning two segments appears, identically, in both; offsets count from
// epoch and are clamped at zero for a cue that began before it.
func SegmentVTT(seg SubtitleSegment, epoch time.Time, cues []captions.Cue) []byte {
	out := make([]webvtt.Cue, 0, len(cues))
	for _, c := range cues {
		start := max(c.Start.Sub(epoch), 0)
		end := max(c.End.Sub(epoch), start)
		out = append(out, webvtt.Cue{ID: c.ID, Start: start, End: end, Text: c.Text})
	}
	return webvtt.EncodeVTT(out, &webvtt.TimestampMap{MPEGTS: seg.MPEGTS})
}

func firstDecode(ev *muxl.MuxlEvent, tid string) uint64 {
	if v, ok := ev.FirstDecodeTimes[tid]; ok {
		return v
	}
	v, _ := muxl.FirstTFDT(ev.Tracks[tid])
	return v
}

// ReadFirstTFDT reads the baseMediaDecodeTime of the segment stored at
// [off, off+size) of r: it skips the signature boxes ahead of the moof by
// their headers and reads only the moof.
func ReadFirstTFDT(r io.ReaderAt, off, size int64) (uint64, bool) {
	const maxMoof = 64 << 10
	end := off + size
	for pos := off; pos+8 <= end; {
		var hdr [8]byte
		if _, err := r.ReadAt(hdr[:], pos); err != nil {
			return 0, false
		}
		boxSize := int64(binary.BigEndian.Uint32(hdr[:4]))
		if boxSize < 8 {
			return 0, false
		}
		if string(hdr[4:]) == "moof" {
			buf := make([]byte, min(boxSize, maxMoof, end-pos))
			if _, err := r.ReadAt(buf, pos); err != nil && err != io.EOF {
				return 0, false
			}
			return muxl.FirstTFDT(buf)
		}
		pos += boxSize
	}
	return 0, false
}
