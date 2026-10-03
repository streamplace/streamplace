package livehls

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions/webvtt"
)

// Four two-second segments of a 6000-tick/s video; segment 2 starts a new
// recording session whose tfdt restarted near zero.
func vodFixture() (durations []uint64, disc []bool, tfdt func(int) (uint64, bool)) {
	durations = []uint64{12000, 12000, 12000, 12000}
	disc = []bool{false, false, true, false}
	tfdts := map[int]uint64{0: 60000, 2: 3000} // first session starts at 10s
	return durations, disc, func(i int) (uint64, bool) {
		v, ok := tfdts[i]
		return v, ok
	}
}

func TestVODSubtitleWindowAlignsToSegments(t *testing.T) {
	durations, disc, tfdt := vodFixture()
	win := VODSubtitleWindow(6000, durations, disc, 0, 3, 0, 0, tfdt)

	require.Equal(t, VODEpoch, win.Epoch)
	require.True(t, win.Ended)
	require.Len(t, win.Segments, 4)
	for i, s := range win.Segments {
		require.Equal(t, uint64(i), s.Seq)
		require.Equal(t, 2*time.Second, s.Duration)
		require.Equal(t, time.Duration(i)*2*time.Second, s.Start.Sub(VODEpoch))
		require.Equal(t, s.Start.Add(2*time.Second), s.End)
	}
	// Session one: tfdt 10s at video 0s, contiguous, so one constant map.
	require.Equal(t, uint64(900000), win.Segments[0].MPEGTS)
	require.Equal(t, uint64(900000), win.Segments[1].MPEGTS)
	// Session two restarts tfdt at 0.5s and starts at video 4s: MPEGTS is
	// 0.5s - 4s = -3.5s, wrapped to 33 bits.
	require.Equal(t, uint64(webvtt.MPEGTSModulus-315000), win.Segments[2].MPEGTS)
	require.Equal(t, uint64(webvtt.MPEGTSModulus-315000), win.Segments[3].MPEGTS)
	require.False(t, win.Segments[1].Discontinuity)
	require.True(t, win.Segments[2].Discontinuity)

	pl := win.Playlist(func(seq uint64) string { return "s" + string(rune('0'+seq)) + ".vtt" })
	require.Equal(t, 1, strings.Count(pl, "#EXT-X-DISCONTINUITY\n"))
	require.Less(t, strings.Index(pl, "s1.vtt"), strings.Index(pl, "#EXT-X-DISCONTINUITY\n"))
	require.Less(t, strings.Index(pl, "#EXT-X-DISCONTINUITY\n"), strings.Index(pl, "s2.vtt"))
	require.Contains(t, pl, "#EXT-X-MEDIA-SEQUENCE:0\n")
	require.Contains(t, pl, "#EXT-X-PLAYLIST-TYPE:VOD\n")
	require.Contains(t, pl, "#EXT-X-ENDLIST\n")
	require.Equal(t, 4, strings.Count(pl, "#EXTINF:2.000000,"))
	require.NotContains(t, pl, "DISCONTINUITY-SEQUENCE")
}

// A clipped playlist starts mid-recording: sequence numbers restart at zero,
// offsets count from the clip start, the first segment is timed from its own
// tfdt, and a boundary trimmed off the front is the discontinuity sequence.
func TestVODSubtitleWindowOfClip(t *testing.T) {
	durations, disc, tfdt := vodFixture()
	// The clip begins 1s into segment 2 (video 5s): segment 2 is the first
	// listed, and the boundary at segment 2 itself is in discSeq.
	win := VODSubtitleWindow(6000, durations, disc, 2, 3, 5*time.Second, 1, func(i int) (uint64, bool) {
		require.Equal(t, 2, i, "only the first listed segment's tfdt is read")
		return tfdt(i)
	})
	require.Len(t, win.Segments, 2)
	require.Equal(t, uint64(0), win.Segments[0].Seq)
	require.Equal(t, -time.Second, win.Segments[0].Start.Sub(VODEpoch), "the first segment began 1s before the clip")
	require.Equal(t, time.Second, win.Segments[1].Start.Sub(VODEpoch))
	// tfdt 0.5s at clip time -1s: MPEGTS = 0.5s - (-1s) = 1.5s.
	require.Equal(t, uint64(135000), win.Segments[0].MPEGTS)
	require.Equal(t, uint64(135000), win.Segments[1].MPEGTS)
	pl := win.Playlist(func(uint64) string { return "x" })
	require.Contains(t, pl, "#EXT-X-DISCONTINUITY-SEQUENCE:1\n")
}

func TestVODSubtitleWindowWithoutTFDTAssumesZeroStart(t *testing.T) {
	durations, disc, _ := vodFixture()
	win := VODSubtitleWindow(6000, durations, disc, 0, 1, 0, 0, func(int) (uint64, bool) { return 0, false })
	require.Equal(t, uint64(0), win.Segments[0].MPEGTS)
	require.Equal(t, uint64(0), win.Segments[1].MPEGTS)
}

func TestVODSubtitleWindowBadRange(t *testing.T) {
	durations, disc, tfdt := vodFixture()
	for _, r := range [][2]int{{-1, 1}, {2, 9}, {3, 1}} {
		require.Empty(t, VODSubtitleWindow(6000, durations, disc, r[0], r[1], 0, 0, tfdt).Segments, r)
	}
}

func TestReadFirstTFDTSkipsSignatureBoxes(t *testing.T) {
	seg := append(append(box("uuid", make([]byte, 5000)), box("uuid", make([]byte, 100))...),
		append(box("moof", box("traf", tfdtBox(1, 4242))), box("mdat", make([]byte, 10))...)...)
	// A blob: some other bytes, then the segment, then another segment.
	blob := append(append(bytes.Repeat([]byte{0xAA}, 321), seg...), seg...)

	got, ok := ReadFirstTFDT(bytes.NewReader(blob), 321, int64(len(seg)))
	require.True(t, ok)
	require.Equal(t, uint64(4242), got)

	_, ok = ReadFirstTFDT(bytes.NewReader(blob), 0, 100)
	require.False(t, ok, "garbage")
	_, ok = ReadFirstTFDT(bytes.NewReader(blob), 321, 100)
	require.False(t, ok, "segment cut off before its moof")
}
