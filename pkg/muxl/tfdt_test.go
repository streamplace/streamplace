package muxl

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func box(typ string, payload ...[]byte) []byte {
	var body []byte
	for _, p := range payload {
		body = append(body, p...)
	}
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out[:4], uint32(8+len(body)))
	copy(out[4:], typ)
	return append(out, body...)
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
