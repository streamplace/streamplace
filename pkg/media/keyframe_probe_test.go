package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAUHasIDR(t *testing.T) {
	annexB := func(types ...byte) []byte {
		var b []byte
		for _, ty := range types {
			b = append(b, 0, 0, 0, 1, ty, 0xaa, 0xbb)
		}
		return b
	}
	avc := func(types ...byte) []byte {
		var b []byte
		for _, ty := range types {
			b = append(b, 0, 0, 0, 3, ty, 0xaa, 0xbb)
		}
		return b
	}
	require.True(t, auHasIDR(annexB(9, 7, 8, 5)), "IDR with headers, Annex B")
	require.True(t, auHasIDR(avc(7, 8, 5)), "IDR with headers, length-prefixed")
	require.False(t, auHasIDR(annexB(9, 1)), "a non-IDR I- or P-slice")
	require.False(t, auHasIDR(avc(1)), "length-prefixed non-IDR")
	require.False(t, auHasIDR(avc(6, 1)), "SEI then non-IDR slice")
	require.False(t, auHasIDR(nil))
	require.False(t, auHasIDR([]byte{1, 2, 3, 4, 5, 6}), "garbage is not an IDR")
	require.Equal(t, []int{7, 8, 5, 1}, h264NALTypes(avc(7, 8, 5, 1)))
}

func TestTUHasShownKeyFrame(t *testing.T) {
	// Sized OBUs: header (type<<3 | has_size), size, payload.
	obu := func(typ, payload byte) []byte { return []byte{typ<<3 | 0x02, 1, payload} }
	td := []byte{2<<3 | 0x02, 0}
	seq := obu(obuSequenceHeader, 0x00) // profile 0, not a reduced still picture
	tu := func(obus ...[]byte) []byte {
		b := append([]byte{}, td...)
		for _, o := range obus {
			b = append(b, o...)
		}
		return b
	}
	// Frame header byte: show_existing_frame(1) frame_type(2) show_frame(1).
	require.True(t, tuHasShownKeyFrame(tu(seq, obu(obuFrame, 0x10))), "a shown key frame")
	require.True(t, tuHasShownKeyFrame(tu(seq, obu(obuFrameHeader, 0x10))), "a shown key frame header")
	require.False(t, tuHasShownKeyFrame(tu(seq, obu(obuFrame, 0x00))), "a hidden (forward) key frame")
	require.False(t, tuHasShownKeyFrame(tu(obu(obuFrame, 0x30))), "a shown inter frame")
	require.False(t, tuHasShownKeyFrame(tu(obu(obuFrameHeader, 0x80))), "show_existing_frame")
	require.False(t, tuHasShownKeyFrame(nil))
}
