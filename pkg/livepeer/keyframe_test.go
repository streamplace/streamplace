package livepeer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTSStartsDecodable(t *testing.T) {
	nal := func(types ...byte) []byte {
		var b []byte
		for _, ty := range types {
			b = append(b, 0x47, 0x41, 0x00, 0x10, 0, 0, 0, 1, ty, 0xaa)
		}
		return b
	}
	require.True(t, tsStartsDecodable(nal(9, 7, 8, 5, 9, 1)))
	require.False(t, tsStartsDecodable(nal(9, 1, 9, 1)), "P-slices only")
	require.False(t, tsStartsDecodable(nal(9, 5)), "IDR without its parameter sets")
	require.False(t, tsStartsDecodable(nil))
}
