package livehls

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/webvtt"
	"stream.place/streamplace/pkg/muxl"
)

var t0 = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// timedSegEvent is a one-second segment whose video and audio tfdt are given
// in their own timescales (90000 and 48000).
func timedSegEvent(videoTFDT, audioTFDT uint64) *muxl.MuxlEvent {
	ev := segEvent([]byte("v"), []byte("a"))
	ev.FirstDecodeTimes = map[string]uint64{"1": videoTFDT, "2": audioTFDT}
	return ev
}

// clockWriter is a Writer whose clock the test drives.
func clockWriter(t *testing.T, opts ...Option) (*Writer, *time.Time) {
	now := t0
	w := NewWriter(opts...)
	w.now = func() time.Time { return now }
	require.NoError(t, w.Observe(initEvent()))
	return w, &now
}

// feed observes n one-second segments, segment i starting i seconds after
// start with tfdt = firstTFDT + i seconds, and advances the clock one second
// per segment, as a live stream does.
func feed(t *testing.T, w *Writer, now *time.Time, start time.Time, firstTFDT uint64, n int) {
	for i := range n {
		require.NoError(t, w.ObserveAt(timedSegEvent(firstTFDT+uint64(i)*90000, uint64(i)*48000), start.Add(time.Duration(i)*time.Second)))
		*now = now.Add(time.Second)
	}
}

func mediaSeqOf(t *testing.T, playlist string) uint64 {
	for _, l := range strings.Split(playlist, "\n") {
		if v, ok := strings.CutPrefix(l, "#EXT-X-MEDIA-SEQUENCE:"); ok {
			var n uint64
			_, err := fmt.Sscanf(v, "%d", &n)
			require.NoError(t, err)
			return n
		}
	}
	t.Fatalf("no media sequence in %s", playlist)
	return 0
}

func TestMasterPlaylistSubtitlesGroup(t *testing.T) {
	w, now := clockWriter(t)
	feed(t, w, now, t0, 0, 3)
	url := func(tid string) string { return "t" + tid + ".m3u8" }

	require.Equal(t, w.MasterPlaylist(url), w.MasterPlaylistWithSubtitles(url, nil), "no tracks: the master is unchanged")
	require.NotContains(t, w.MasterPlaylist(url), "SUBTITLES")

	master := w.MasterPlaylistWithSubtitles(url, []SubtitleRendition{
		{Name: `English "auto"`, Language: "en", URI: "/cc?track=canonical-auto-en"},
		{Name: "Deutsch", Language: "de", URI: "/cc?track=sidecar-auto-de"},
	})
	lines := strings.Split(master, "\n")
	var media, inf []string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "#EXT-X-MEDIA:TYPE=SUBTITLES"):
			media = append(media, l)
		case strings.HasPrefix(l, "#EXT-X-STREAM-INF"):
			inf = append(inf, l)
		}
	}
	require.Equal(t, []string{
		`#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="cc",NAME="English 'auto'",LANGUAGE="en",AUTOSELECT=YES,DEFAULT=NO,URI="/cc?track=canonical-auto-en"`,
		`#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="cc",NAME="Deutsch",LANGUAGE="de",AUTOSELECT=YES,DEFAULT=NO,URI="/cc?track=sidecar-auto-de"`,
	}, media)
	require.Len(t, inf, 1)
	require.Contains(t, inf[0], `SUBTITLES="cc"`)
	require.Contains(t, inf[0], `AUDIO="audio"`, "the audio group is kept")
}

func TestSubtitleWindowMirrorsVideoSequence(t *testing.T) {
	w, now := clockWriter(t)
	feed(t, w, now, t0, 0, 5)

	video := w.MediaPlaylist("1", "init", segURI)
	win := w.SubtitleWindow(0, nil)
	require.Len(t, win.Segments, 5)

	subs := win.Playlist(func(seq uint64) string { return fmt.Sprintf("cc%d.vtt", seq) })
	require.Equal(t, mediaSeqOf(t, video), mediaSeqOf(t, subs))
	require.Equal(t, strings.Count(video, "#EXTINF:1.000000,"), strings.Count(subs, "#EXTINF:1.000000,"))
	for seq := range 5 {
		require.Contains(t, video, fmt.Sprintf("seg%d.m4s", seq))
		require.Contains(t, subs, fmt.Sprintf("cc%d.vtt", seq))
	}
	require.NotContains(t, subs, "ENDLIST")
	require.NotContains(t, subs, "EXT-X-MAP", "WebVTT segments have no init")
	require.Contains(t, subs, "#EXT-X-TARGETDURATION:1\n")
}

func TestSubtitleWindowSlidesWithVideoWindow(t *testing.T) {
	w, now := clockWriter(t, WithWindow(3), WithRetention(time.Minute))
	feed(t, w, now, t0, 0, 8)

	win := w.SubtitleWindow(0, nil)
	require.Equal(t, mediaSeqOf(t, w.MediaPlaylist("1", "init", segURI)), win.Segments[0].Seq)
	require.Len(t, win.Segments, 3)
	require.Equal(t, uint64(5), win.Segments[0].Seq)
	require.Equal(t, uint64(7), win.Segments[2].Seq)
}

// The captions run behind the video: a segment is listed once it is older than
// the latency budget or its captions are provably complete, and the playlist
// ends at the last such segment.
func TestSubtitleWindowLagHandling(t *testing.T) {
	w, now := clockWriter(t)
	feed(t, w, now, t0, 0, 6) // segments 0..5 added at t0+0..t0+5; now = t0+6
	budget := 3 * time.Second

	seqs := func(win SubtitleWindow) []uint64 {
		var out []uint64
		for _, s := range win.Segments {
			out = append(out, s.Seq)
		}
		return out
	}

	// Segment i is 6-i seconds old: those at least 3s old are ready (0..3).
	require.Equal(t, []uint64{0, 1, 2, 3}, seqs(w.SubtitleWindow(budget, nil)))

	// Final cues published past the end of segment 4 make it, and what
	// precedes it, ready early; segment 5 is still waiting.
	published := t0.Add(5 * time.Second)
	final := func(start, end time.Time) bool { return !end.After(published) }
	require.Equal(t, []uint64{0, 1, 2, 3, 4}, seqs(w.SubtitleWindow(budget, final)))

	// Time passes with no new cues (silence): everything becomes ready.
	*now = now.Add(3 * time.Second)
	require.Equal(t, []uint64{0, 1, 2, 3, 4, 5}, seqs(w.SubtitleWindow(budget, nil)))

	// Nothing is ready for a stream younger than the budget.
	young, ynow := clockWriter(t)
	feed(t, young, ynow, t0, 0, 1)
	require.Empty(t, young.SubtitleWindow(budget, nil).Segments)

	// A finalized window is complete and lists everything.
	young.Finalize()
	ended := young.SubtitleWindow(budget, nil)
	require.Len(t, ended.Segments, 1)
	pl := ended.Playlist(func(seq uint64) string { return "s" })
	require.Contains(t, pl, "#EXT-X-ENDLIST")
	require.Contains(t, pl, "#EXT-X-PLAYLIST-TYPE:VOD")
}

func TestSubtitleSegmentLookup(t *testing.T) {
	w, now := clockWriter(t)
	feed(t, w, now, t0, 0, 4)
	budget := 2 * time.Second

	seg, epoch, ok := w.SubtitleSegment(1, budget, nil)
	require.True(t, ok)
	require.Equal(t, t0, epoch)
	require.Equal(t, t0.Add(time.Second), seg.Start)
	require.Equal(t, t0.Add(2*time.Second), seg.End)

	_, _, ok = w.SubtitleSegment(3, budget, nil)
	require.False(t, ok, "the newest segment is not ready yet")
	_, _, ok = w.SubtitleSegment(99, budget, nil)
	require.False(t, ok, "unknown sequence")
}

func TestSubtitleWindowNeedsSegmentStartTimes(t *testing.T) {
	w, now := clockWriter(t)
	for range 3 {
		require.NoError(t, w.Observe(segEvent([]byte("v"), []byte("a"))))
		*now = now.Add(time.Second)
	}
	require.Empty(t, w.SubtitleWindow(0, nil).Segments)
	_, _, ok := w.SubtitleSegment(0, 0, nil)
	require.False(t, ok)
}

func TestSubtitleReferenceIsFirstVideoTrack(t *testing.T) {
	w, now := clockWriter(t)
	feed(t, w, now, t0, 90000, 2)
	seg, _, ok := w.SubtitleSegment(0, 0, nil)
	require.True(t, ok)
	// Video timescale 90000: tfdt 90000 is 1s, not the audio track's 0.
	require.Equal(t, uint64(90000), seg.MPEGTS)
}

// Worked example. The window's epoch E is the first segment's start. A segment
// starting d after E whose first sample is at media time tfdt gets
// MPEGTS = tfdt*90000/timescale - d*90000.
func TestTimestampMapWorkedExample(t *testing.T) {
	w, now := clockWriter(t)
	// First segment at E=12:00:00 with tfdt = 10s (900000 @ 90kHz).
	feed(t, w, now, t0, 900000, 4)
	win := w.SubtitleWindow(0, nil)
	require.Equal(t, t0, win.Epoch)

	// tfdt and wall clock advance together: MPEGTS is the same everywhere.
	for _, s := range win.Segments {
		require.Equal(t, uint64(900000), s.MPEGTS, "segment %d", s.Seq)
	}

	// A cue at E+2.5s sits in segment 2 (starts E+2s, tfdt 12s): cue time 2.5
	// plays at 900000 + 2.5*90000 = 1125000 = tfdt(12s)+0.5s, which is the
	// video frame at that wall time.
	seg := win.Segments[2]
	cueMedia := seg.MPEGTS + uint64(2.5*90000)
	videoMedia := uint64(900000+2*90000) + uint64(0.5*90000)
	require.Equal(t, videoMedia, cueMedia)

	vtt := SegmentVTT(seg, win.Epoch, []captions.Cue{{ID: "c", Start: t0.Add(2500 * time.Millisecond), End: t0.Add(3 * time.Second), Text: "hi"}})
	require.Equal(t, "WEBVTT\nX-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:00:00:00.000\n\nc\n00:00:02.500 --> 00:00:03.000\nhi\n\n", string(vtt))
}

func TestTimestampMapFollowsTheVideoNotTheWallClock(t *testing.T) {
	// Wall clock gap: segment 1 starts 0.5s later than its predecessor ends,
	// but tfdt is contiguous. The cue timeline stays on the wall clock, so
	// MPEGTS shifts by the gap to keep cue times on the video.
	got := TimestampMapMPEGTS(90000, 90000, t0.Add(1500*time.Millisecond), t0)
	require.Equal(t, uint64(90000-135000+webvtt.MPEGTSModulus), got)

	// Reconnect: tfdt restarts near zero 20s into the window. The map wraps
	// to a positive 33-bit value that equals -1800000 mod 2^33.
	got = TimestampMapMPEGTS(0, 90000, t0.Add(20*time.Second), t0)
	require.Equal(t, uint64(webvtt.MPEGTSModulus-1800000), got)

	// Timescales other than 90kHz are converted exactly.
	require.Equal(t, uint64(90000), TimestampMapMPEGTS(48000, 48000, t0, t0))
	require.Equal(t, uint64(45000), TimestampMapMPEGTS(24000, 48000, t0, t0))

	// Beyond 2^33 ticks (about 26.5 hours) the timestamp wraps like MPEG-TS.
	require.Equal(t, uint64(5), TimestampMapMPEGTS(webvtt.MPEGTSModulus+5, 90000, t0, t0))
}

// A cue that overlaps two segments is in both documents, with the same id and
// the same timestamps, so players recognise the repeat.
func TestSegmentVTTCueSpanningSegmentBoundary(t *testing.T) {
	w, now := clockWriter(t)
	feed(t, w, now, t0, 0, 3)
	win := w.SubtitleWindow(0, nil)

	spanning := captions.Cue{ID: "span", Start: t0.Add(900 * time.Millisecond), End: t0.Add(1600 * time.Millisecond), Text: "across the cut"}
	inFirst := captions.Cue{ID: "one", Start: t0.Add(100 * time.Millisecond), End: t0.Add(500 * time.Millisecond), Text: "first"}
	inSecond := captions.Cue{ID: "two", Start: t0.Add(1700 * time.Millisecond), End: t0.Add(1900 * time.Millisecond), Text: "second"}
	all := []captions.Cue{inFirst, spanning, inSecond}

	// What the hub returns for each segment's range.
	overlap := func(s SubtitleSegment) []captions.Cue {
		var out []captions.Cue
		for _, c := range all {
			if c.Start.Before(s.End) && c.End.After(s.Start) {
				out = append(out, c)
			}
		}
		return out
	}
	doc0 := string(SegmentVTT(win.Segments[0], win.Epoch, overlap(win.Segments[0])))
	doc1 := string(SegmentVTT(win.Segments[1], win.Epoch, overlap(win.Segments[1])))
	doc2 := string(SegmentVTT(win.Segments[2], win.Epoch, overlap(win.Segments[2])))

	const spanBlock = "span\n00:00:00.900 --> 00:00:01.600\nacross the cut\n"
	require.Contains(t, doc0, spanBlock)
	require.Contains(t, doc1, spanBlock)
	require.NotContains(t, doc2, "span")
	require.Contains(t, doc0, "first")
	require.NotContains(t, doc1, "first")
	require.Contains(t, doc1, "second")
}

func TestSegmentVTTClampsCuesBeforeEpoch(t *testing.T) {
	seg := SubtitleSegment{Start: t0, End: t0.Add(time.Second)}
	vtt := string(SegmentVTT(seg, t0, []captions.Cue{{ID: "early", Start: t0.Add(-2 * time.Second), End: t0.Add(500 * time.Millisecond), Text: "x"}}))
	require.Contains(t, vtt, "00:00:00.000 --> 00:00:00.500")
}

func TestSegmentVTTEmptyIsValid(t *testing.T) {
	vtt := string(SegmentVTT(SubtitleSegment{MPEGTS: 42}, t0, nil))
	require.Equal(t, "WEBVTT\nX-TIMESTAMP-MAP=MPEGTS:42,LOCAL:00:00:00.000\n\n", vtt)
}

func box(typ string, payload ...[]byte) []byte {
	var body []byte
	for _, p := range payload {
		body = append(body, p...)
	}
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], typ)
	return append(b, body...)
}

func tfdtBox(version byte, v uint64) []byte {
	if version == 1 {
		p := make([]byte, 12)
		p[0] = 1
		binary.BigEndian.PutUint64(p[4:], v)
		return box("tfdt", p)
	}
	p := make([]byte, 8)
	binary.BigEndian.PutUint32(p[4:], uint32(v))
	return box("tfdt", p)
}

func TestFirstTFDT(t *testing.T) {
	uuid := box("uuid", make([]byte, 40))
	mfhd := box("mfhd", make([]byte, 8))
	seg := func(v byte, tfdt uint64) []byte {
		return append(append([]byte(nil), uuid...), box("moof", mfhd, box("traf", box("tfhd", make([]byte, 8)), tfdtBox(v, tfdt)))...)
	}

	for _, v := range []byte{0, 1} {
		got, ok := FirstTFDT(append(seg(v, 123456), box("mdat", []byte("media"))...))
		require.True(t, ok)
		require.Equal(t, uint64(123456), got)
	}
	big, ok := FirstTFDT(seg(1, 1<<40))
	require.True(t, ok)
	require.Equal(t, uint64(1<<40), big)

	// Only the start of a segment is available (a ranged read): the moof is
	// cut off after its tfdt.
	full := append(append([]byte(nil), uuid...), box("moof", mfhd, box("traf", box("tfhd", make([]byte, 8)), tfdtBox(1, 777), box("trun", make([]byte, 400))))...)
	cut := full[:len(full)-200]
	got, ok := FirstTFDT(cut)
	require.True(t, ok)
	require.Equal(t, uint64(777), got)

	for name, in := range map[string][]byte{
		"empty":     nil,
		"short":     {0, 0, 0},
		"no tfdt":   box("moof", mfhd),
		"zero size": append(make([]byte, 4), []byte("moof....")...),
		"truncated": box("uuid", make([]byte, 40))[:20],
	} {
		_, ok := FirstTFDT(in)
		require.False(t, ok, name)
	}
}

// Without an explicit tfdt in the event the window reads it from the segment.
func TestObserveFallsBackToSegmentTFDT(t *testing.T) {
	w, _ := clockWriter(t)
	ev := segEvent(box("moof", box("traf", tfdtBox(0, 4500))), []byte("a"))
	require.NoError(t, w.ObserveAt(ev, t0))
	require.Equal(t, uint64(4500), w.Track("1").Segments[0].FirstDecode)
}

// A fragment joined out of short segments is timed from its first piece.
func TestSubtitleSegmentOfJoinedFragment(t *testing.T) {
	w, now := clockWriter(t, WithMinFragment(time.Second))
	short := func(tfdt uint64) *muxl.MuxlEvent {
		ev := segEvent([]byte("v"), []byte("a"))
		ev.Durations = map[string]uint64{"1": 22500, "2": 12000} // 250ms
		ev.FirstDecodeTimes = map[string]uint64{"1": tfdt, "2": 0}
		return ev
	}
	for i := range 4 {
		require.NoError(t, w.ObserveAt(short(uint64(i)*22500), t0.Add(time.Duration(i)*250*time.Millisecond)))
	}
	*now = now.Add(time.Second)
	win := w.SubtitleWindow(0, nil)
	require.Len(t, win.Segments, 1)
	require.Equal(t, t0, win.Segments[0].Start)
	require.Equal(t, time.Second, win.Segments[0].Duration)
}
