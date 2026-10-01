package fmp4

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

func box(typ string, payload ...[]byte) []byte {
	var body []byte
	for _, p := range payload {
		body = append(body, p...)
	}
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out, uint32(8+len(body)))
	copy(out[4:], typ)
	return append(out, body...)
}

func u32(v uint32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return b[:]
}

func u64(v uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	return b[:]
}

// fragment builds [moof][mdat] for one track with per-sample sizes,
// durations and composition offsets, default-base-is-moof addressing.
func fragment(trackID uint32, tfdt uint64, samples [][]byte, durations []uint32, ctos []int32, defaultDur uint32) []byte {
	var mdat []byte
	for _, s := range samples {
		mdat = append(mdat, s...)
	}
	tfhdFlags := uint32(tfhdDefaultBaseIsMoof)
	var tfhdBody []byte
	if defaultDur != 0 {
		tfhdFlags |= tfhdDefaultSampleDuration
		tfhdBody = u32(defaultDur)
	}
	tfhd := box("tfhd", u32(tfhdFlags), u32(trackID), tfhdBody)
	tfdtBox := box("tfdt", []byte{1, 0, 0, 0}, u64(tfdt))
	trunFlags := uint32(trunDataOffset | trunSampleSize | trunSampleCTO)
	if durations != nil {
		trunFlags |= trunSampleDuration
	}
	var entries []byte
	for i, s := range samples {
		if durations != nil {
			entries = append(entries, u32(durations[i])...)
		}
		entries = append(entries, u32(uint32(len(s)))...)
		entries = append(entries, u32(uint32(ctos[i]))...)
	}
	// Data offset is filled in once the moof size is known.
	trun := func(off uint32) []byte {
		return box("trun", []byte{1, byte(trunFlags >> 16), byte(trunFlags >> 8), byte(trunFlags)}, u32(uint32(len(samples))), u32(off), entries)
	}
	moofLen := len(box("moof", box("mfhd", u32(0), u32(1)), box("traf", tfhd, tfdtBox, trun(0))))
	moof := box("moof", box("mfhd", u32(0), u32(1)), box("traf", tfhd, tfdtBox, trun(uint32(moofLen+8))))
	return append(moof, box("mdat", mdat)...)
}

func TestFragmentsSynthesized(t *testing.T) {
	samples := [][]byte{[]byte("IIII"), []byte("PP"), []byte("B")}
	// Decode order I P B with B-frame reordering: PTS = DTS + cto.
	b := fragment(1, 9000, samples, []uint32{3000, 3000, 3000}, []int32{3000, 6000, 0}, 0)
	// A leading uuid box, like a canonical MUXL track carries.
	b = append(box("uuid", make([]byte, 16), []byte("sig")), b...)
	// A second fragment relying on the default duration, with a version 1
	// negative composition offset.
	b = append(b, fragment(1, 18000, [][]byte{[]byte("next")}, nil, []int32{-1000}, 3000)...)

	frags, err := Fragments(b)
	require.NoError(t, err)
	require.Len(t, frags, 2)
	require.Equal(t, uint32(1), frags[0].TrackID)
	require.Equal(t, uint64(9000), frags[0].BaseDecodeTime)
	require.Len(t, frags[0].Samples, 3)
	require.Equal(t, "IIII", string(frags[0].Samples[0].Data))
	require.Equal(t, "PP", string(frags[0].Samples[1].Data))
	require.Equal(t, "B", string(frags[0].Samples[2].Data))
	require.Equal(t, []uint64{9000, 12000, 15000}, []uint64{frags[0].Samples[0].DTS, frags[0].Samples[1].DTS, frags[0].Samples[2].DTS})
	require.Equal(t, []uint64{12000, 18000, 15000}, []uint64{frags[0].Samples[0].PTS, frags[0].Samples[1].PTS, frags[0].Samples[2].PTS})
	require.Equal(t, "next", string(frags[1].Samples[0].Data))
	require.Equal(t, uint64(18000), frags[1].Samples[0].DTS)
	require.Equal(t, uint64(17000), frags[1].Samples[0].PTS, "a negative offset pulls presentation earlier")
}

func TestFragmentsRejectsSamplesPastTheEnd(t *testing.T) {
	b := fragment(1, 0, [][]byte{[]byte("abcd")}, nil, []int32{0}, 1)
	_, err := Fragments(b[:len(b)-2])
	require.Error(t, err)
}

func TestTracksAndTextTrackDetection(t *testing.T) {
	trak := func(id uint32, timescale uint32, handler string) []byte {
		tkhd := box("tkhd", []byte{0, 0, 0, 0}, u32(0), u32(0), u32(id))
		mdhd := box("mdhd", []byte{0, 0, 0, 0}, u32(0), u32(0), u32(timescale), u32(0), u32(0))
		hdlr := box("hdlr", []byte{0, 0, 0, 0}, u32(0), []byte(handler), make([]byte, 12), []byte{0})
		return box("trak", tkhd, box("mdia", mdhd, hdlr))
	}
	moov := box("moov", box("mvhd", make([]byte, 100)), trak(1, 90000, "vide"), trak(2, 48000, "soun"))
	file := append(box("ftyp", []byte("isom"), u32(0)), moov...)
	tracks, err := Tracks(file)
	require.NoError(t, err)
	require.Equal(t, []TrackInfo{{ID: 1, Timescale: 90000, Handler: "vide"}, {ID: 2, Timescale: 48000, Handler: "soun"}}, tracks)
	require.False(t, HasTextTrack(file))

	withText := append(box("ftyp", []byte("isom"), u32(0)), box("moov", trak(1, 90000, "vide"), trak(3, 1000, "text"))...)
	require.True(t, HasTextTrack(withText))
	require.False(t, HasTextTrack(nil))
}

// TestFragmentsReadsCanonicalMuxlTrack walks a real canonical video track
// (muxl's canonicalization of a fixture) and checks the samples are whole
// length-prefixed access units in decode order.
func TestFragmentsReadsCanonicalMuxlTrack(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	fixture := filepath.Join(filepath.Dir(filename), "..", "..", "..", "test", "fixtures", "sample-segment.mp4")
	mp4, err := os.ReadFile(fixture)
	require.NoError(t, err)
	ctx := context.Background()
	canon, err := muxl.RunMuxlCanonicalize(ctx, mp4, nil)
	require.NoError(t, err)

	events := make(chan *muxl.MuxlEvent, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(canon), events)
		close(events)
	}()
	var cat *muxl.MuxlCatalog
	var video []byte
	var videoID string
	for ev := range events {
		switch ev.Type {
		case "init":
			cat = ev.Catalog
		case "segment", "signed-segment":
			if videoID == "" && cat != nil && cat.Video != nil {
				for _, v := range cat.Video.Renditions {
					videoID = itoa(v.TrackID())
				}
			}
			video = append(video, ev.Tracks[videoID]...)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, video)

	frags, err := Fragments(video)
	require.NoError(t, err)
	require.NotEmpty(t, frags)
	total := 0
	var lastDTS uint64
	require.True(t, frags[0].Samples[0].Sync, "a canonical segment starts at a keyframe")
	for _, f := range frags {
		require.NotEmpty(t, f.Samples)
		for _, s := range f.Samples {
			total++
			require.GreaterOrEqual(t, s.DTS, lastDTS)
			lastDTS = s.DTS
			// Length-prefixed NALs fill the sample exactly.
			rest := s.Data
			for len(rest) > 0 {
				require.GreaterOrEqual(t, len(rest), 4)
				n := int(binary.BigEndian.Uint32(rest[:4]))
				require.LessOrEqual(t, n, len(rest)-4)
				rest = rest[4+n:]
			}
		}
	}
	require.Greater(t, total, 1)
	pts := make([]uint64, 0, total)
	for _, f := range frags {
		for _, s := range f.Samples {
			pts = append(pts, s.PTS)
		}
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i] < pts[j] })
	require.Equal(t, frags[0].BaseDecodeTime, frags[0].Samples[0].DTS)
}

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
