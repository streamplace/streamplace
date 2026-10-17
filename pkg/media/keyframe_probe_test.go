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
