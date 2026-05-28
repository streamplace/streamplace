package media

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// audioCodecsOf returns the sorted distinct audio codecs in a bare .m4s segment.
func audioCodecsOf(t *testing.T, ctx context.Context, seg []byte) []string {
	t.Helper()
	events, err := unwrapMuxlEvents(ctx, seg)
	require.NoError(t, err)
	cat, _ := catalogAndTracks(events)
	require.NotNil(t, cat, "segment has a catalog")
	var out []string
	if cat.Audio != nil {
		for _, a := range cat.Audio.Renditions {
			out = append(out, a.Codec)
		}
	}
	sort.Strings(out)
	return out
}
