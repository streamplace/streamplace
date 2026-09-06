package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConcatTracksSorted(t *testing.T) {
	tracks := map[string][]byte{
		"1":  []byte("a"),
		"2":  []byte("b"),
		"10": []byte("c"),
		"3":  []byte("d"),
	}
	// numeric track-id order, not lexical ("10" must not sort before "2")
	require.Equal(t, "abdc", string(concatTracksSorted(tracks)))
}
