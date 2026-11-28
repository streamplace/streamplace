package vod

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"testing"
	"time"

	"github.com/abema/go-mp4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
)

const (
	syncSampleFlags    = 0x02000000
	nonSyncSampleFlags = 0x01010000
)

type splitSample struct {
	DTS      uint64
	Duration uint32
	Flags    uint32
	CTO      int64
	Data     string
}

// decodeFragment demuxes every moof+mdat pair in a single-track GOP.
func decodeFragment(t *testing.T, frag []byte) []splitSample {
	t.Helper()
	var out []splitSample
	start := -1
	for b := frag; len(b) > 0; {
		box, tail, err := nextBox(b)
		require.NoError(t, err)
		switch string(box[4:8]) {
		case "moof":
			require.Equal(t, -1, start)
			start = len(frag) - len(b)
		case "mdat":
			require.NotEqual(t, -1, start)
			out = append(out, decodeOneFragment(t, frag[start:len(frag)-len(tail)])...)
			start = -1
		}
		b = tail
	}
	require.Equal(t, -1, start)
	return out
}

func decodeOneFragment(t *testing.T, frag []byte) []splitSample {
	t.Helper()
	r := bytes.NewReader(frag)
	one := func(path ...mp4.BoxType) *mp4.BoxInfoWithPayload {
		boxes, err := mp4.ExtractBoxWithPayload(r, nil, path)
		require.NoError(t, err)
		require.Len(t, boxes, 1)
		return boxes[0]
	}
	moof, mdat := one(mp4.BoxTypeMoof()), one(mp4.BoxTypeMdat())
	tfhd := one(mp4.BoxTypeMoof(), mp4.BoxTypeTraf(), mp4.BoxTypeTfhd()).Payload.(*mp4.Tfhd)
	tfdt := one(mp4.BoxTypeMoof(), mp4.BoxTypeTraf(), mp4.BoxTypeTfdt()).Payload.(*mp4.Tfdt)
	trun := one(mp4.BoxTypeMoof(), mp4.BoxTypeTraf(), mp4.BoxTypeTrun()).Payload.(*mp4.Trun)

	pos := mdat.Info.Offset + mdat.Info.HeaderSize
	if trun.CheckFlag(trunDataOffset) {
		pos = moof.Info.Offset + uint64(trun.DataOffset)
	}
	dts := tfdt.GetBaseMediaDecodeTime()
	var out []splitSample
	for i := 0; i < int(trun.SampleCount); i++ {
		s := splitSample{DTS: dts, Duration: tfhd.DefaultSampleDuration, Flags: tfhd.DefaultSampleFlags}
		size := tfhd.DefaultSampleSize
		if trun.CheckFlag(trunSampleDuration) {
			s.Duration = trun.Entries[i].SampleDuration
		}
		if trun.CheckFlag(trunSampleSize) {
			size = trun.Entries[i].SampleSize
		}
		if trun.CheckFlag(0x400) {
			s.Flags = trun.Entries[i].SampleFlags
		}
		if trun.CheckFlag(0x800) {
			s.CTO = trun.GetSampleCompositionTimeOffset(i)
		}
		if i == 0 && trun.CheckFlag(trunFirstSampleFlags) {
			s.Flags = trun.FirstSampleFlags
		}
		require.LessOrEqual(t, pos+uint64(size), mdat.Info.Offset+mdat.Info.Size, "sample inside the mdat")
		s.Data = string(frag[pos : pos+uint64(size)])
		out = append(out, s)
		pos += uint64(size)
		dts += uint64(s.Duration)
	}
	return out
}

// boxTree lists a fragment's boxes in order, descending into moof and traf.
func boxTree(t *testing.T, frag []byte) []string {
	t.Helper()
	var types []string
	_, err := mp4.ReadBoxStructure(bytes.NewReader(frag), func(h *mp4.ReadHandle) (interface{}, error) {
		types = append(types, h.BoxInfo.Type.String())
		if h.BoxInfo.Type == mp4.BoxTypeMoof() || h.BoxInfo.Type == mp4.BoxTypeTraf() {
			return h.Expand()
		}
		return nil, nil
	})
	require.NoError(t, err)
	return types
}

func rawBox(typ string, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	return append(append(binary.BigEndian.AppendUint32(nil, uint32(8+len(body))), typ...), body...)
}

func marshalBox(t *testing.T, b mp4.IBox) []byte {
	t.Helper()
	var buf bytes.Buffer
	_, err := mp4.Marshal(&buf, b, mp4.Context{})
	require.NoError(t, err)
	return rawBox(b.GetType().String(), buf.Bytes())
}

// fragmentSpec describes a fragment to build with go-mp4's marshaller.
type fragmentSpec struct {
	tfhdFlags   uint32
	tfdtVersion uint8
	decodeTime  uint64
	trunVersion uint8
	trunFlags   uint32
	samples     []string
}

func (spec fragmentSpec) build(t *testing.T) []byte {
	t.Helper()
	tfhd := &mp4.Tfhd{TrackID: 1, DefaultSampleDuration: 3000, DefaultSampleSize: uint32(len(spec.samples[0])), DefaultSampleFlags: nonSyncSampleFlags}
	tfhd.SetFlags(spec.tfhdFlags)
	tfdt := &mp4.Tfdt{BaseMediaDecodeTimeV0: uint32(spec.decodeTime), BaseMediaDecodeTimeV1: spec.decodeTime}
	tfdt.SetVersion(spec.tfdtVersion)
	trun := &mp4.Trun{SampleCount: uint32(len(spec.samples)), FirstSampleFlags: syncSampleFlags}
	trun.SetVersion(spec.trunVersion)
	trun.SetFlags(spec.trunFlags)
	var mdat [][]byte
	for i, s := range spec.samples {
		e := mp4.TrunEntry{
			SampleDuration:                uint32(1000 + i),
			SampleSize:                    uint32(len(s)),
			SampleFlags:                   nonSyncSampleFlags,
			SampleCompositionTimeOffsetV0: uint32(500 * i),
			SampleCompositionTimeOffsetV1: int32(-500 * i),
		}
		if i == 0 {
			e.SampleFlags = syncSampleFlags
		}
		trun.Entries = append(trun.Entries, e)
		mdat = append(mdat, []byte(s))
	}
	moof := func() []byte {
		return rawBox("moof",
			marshalBox(t, &mp4.Mfhd{SequenceNumber: 7}),
			rawBox("traf", marshalBox(t, tfhd), marshalBox(t, tfdt), rawBox("sbgp", []byte("not carried")), marshalBox(t, trun)))
	}
	trun.DataOffset = int32(len(moof()) + 8)
	return bytes.Join([][]byte{rawBox("uuid", make([]byte, 16), []byte("c2pa")), rawBox("uuid", make([]byte, 16)), moof(), rawBox("mdat", mdat...)}, nil)
}

func TestSplitFirstSample(t *testing.T) {
	varied := []string{"keyframe-bytes", "p1", "p-two", "p3!"}
	uniform := []string{"KKKK", "aaaa", "bbbb"}
	for name, spec := range map[string]fragmentSpec{
		"per-sample duration, size, flags and signed cto; 64-bit tfdt": {
			tfhdFlags: 0x020000, tfdtVersion: 1, decodeTime: 1 << 40,
			trunVersion: 1, trunFlags: 0x000f01, samples: varied,
		},
		"tfhd defaults with first-sample-flags; 32-bit tfdt that overflows": {
			tfhdFlags: 0x020038, decodeTime: math.MaxUint32 - 5,
			trunFlags: 0x000005, samples: uniform,
		},
		"no data offset, sample description index, per-sample size and unsigned cto": {
			tfhdFlags: 0x02002a, decodeTime: 9000,
			trunFlags: 0x000a00, samples: varied,
		},
		"one sample": {
			tfhdFlags: 0x020000, decodeTime: 9000,
			trunFlags: 0x000701, samples: varied[:1],
		},
	} {
		t.Run(name, func(t *testing.T) {
			frag := spec.build(t)
			want := decodeFragment(t, frag)
			require.Len(t, want, len(spec.samples))

			key, rest, err := SplitFirstSample(frag)
			require.NoError(t, err)
			tree := []string{"moof", "mfhd", "traf", "tfhd", "tfdt", "trun", "mdat"}
			require.Equal(t, tree, boxTree(t, key))
			got := decodeFragment(t, key)
			require.Len(t, got, 1, "key carries only the first sample")
			if len(spec.samples) == 1 {
				require.Nil(t, rest)
			} else {
				require.Equal(t, tree, boxTree(t, rest))
				got = append(got, decodeFragment(t, rest)...)
			}
			require.Equal(t, want, got, "key+rest decode to the original samples and timestamps")
		})
	}
}

func TestSplitFirstSampleMultipleFragments(t *testing.T) {
	first := fragmentSpec{tfhdFlags: 0x020000, trunFlags: 0x000701, samples: []string{"key", "p1"}}
	next := first
	next.decodeTime = 2001
	// Each pair after the first has no segment-level uuid prefix.
	second := next.build(t)
	for string(second[4:8]) == "uuid" {
		_, tail, err := nextBox(second)
		require.NoError(t, err)
		second = tail
	}
	fragment := append(first.build(t), second...)
	want := decodeFragment(t, fragment)
	key, rest, err := SplitFirstSample(fragment)
	require.NoError(t, err)
	require.Equal(t, want, append(decodeFragment(t, key), decodeFragment(t, rest)...))
}

func TestSplitFirstSampleRefusesWhatItCannotSplit(t *testing.T) {
	good := fragmentSpec{tfhdFlags: 0x020000, trunFlags: 0x000701, samples: []string{"key", "p1", "p2"}}
	frag := good.build(t)
	_, _, err := SplitFirstSample(frag)
	require.NoError(t, err)

	// Every truncation is an error, never a panic or a short fragment.
	for n := range len(frag) {
		_, _, err := SplitFirstSample(frag[:n])
		require.Error(t, err, "truncated to %d bytes", n)
	}

	traf := rawBox("traf")
	_, _, err = SplitFirstSample(append(rawBox("moof", traf, traf), rawBox("mdat")...))
	require.ErrorContains(t, err, "more than one traf")

	// Durations and sizes left to the init segment's trex can't be split here.
	trexOnly := fragmentSpec{tfhdFlags: 0x020000, trunFlags: 0x000001, samples: []string{"key", "p1"}}
	_, _, err = SplitFirstSample(trexOnly.build(t))
	require.Error(t, err)

	// A sample size that runs past the mdat.
	bad := bytes.Clone(frag)
	sizeAt := bytes.Index(bad, []byte("trun")) + 4 + 4 + 4 + 4 + 4
	binary.BigEndian.PutUint32(bad[sizeAt:], 1<<20)
	_, _, err = SplitFirstSample(bad)
	require.ErrorContains(t, err, "outside the mdat")

	overflow := good
	overflow.tfdtVersion = 1
	overflow.decodeTime = math.MaxUint64
	_, _, err = SplitFirstSample(overflow.build(t))
	require.ErrorContains(t, err, "decode time overflow")
}

// A real MUXL video segment splits: guards the trun/tfhd layout muxl writes.
func TestSplitFirstSampleMuxlSegment(t *testing.T) {
	warmGST()
	ctx := context.Background()
	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)
	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	mb := newMetafileBuilder(ctx, store)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), teeWriter{hasher, out}, mb, signer.SignerInput)
	require.NoError(t, err)
	meta := mb.Finalize(hasher.CID(), int64(out.Len()))

	split := 0
	for _, track := range meta.Tracks {
		if track.Type != "video" {
			continue
		}
		seg := track.Segments[0]
		frag := out.Bytes()[seg.Offset : seg.Offset+seg.Size]
		want := decodeFragment(t, frag)
		require.Len(t, want, int(seg.SampleCount))
		key, rest, err := SplitFirstSample(frag)
		require.NoError(t, err)
		require.Equal(t, want, append(decodeFragment(t, key), decodeFragment(t, rest)...))
		split++
	}
	require.NotZero(t, split, "fixture has a video track")
}
