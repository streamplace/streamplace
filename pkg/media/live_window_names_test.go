package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/livehls"
)

// Rendition names come from the window's rendition tracks (ids from
// renditionTrackBase up), highest first, never the source track.
func TestRenditionNames(t *testing.T) {
	names := renditionNames([]livehls.VideoTrack{
		{ID: "1", Width: 1920, Height: 1080},
		{ID: "102", Width: 426, Height: 240},
		{ID: "100", Width: 1280, Height: 720},
		{ID: "101", Width: 640, Height: 360},
		{ID: "x", Height: 99},
	})
	require.Equal(t, []string{"720p", "360p", "240p"}, names)
	require.Empty(t, renditionNames([]livehls.VideoTrack{{ID: "1", Height: 1080}}))
	require.Empty(t, renditionNames(nil))
}
