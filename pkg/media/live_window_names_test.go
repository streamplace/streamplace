package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/livehls"
)

// Rendition names come from the window's video tracks, highest first, once
// per name, never the source track's name: transcoded renditions and those
// the streamer sent alongside the source alike.
func TestRenditionNames(t *testing.T) {
	names := renditionNames([]livehls.VideoTrack{
		{ID: "1", Width: 1920, Height: 1080},
		{ID: "102", Width: 426, Height: 240},
		{ID: "100", Width: 1280, Height: 720},
		{ID: "101", Width: 640, Height: 360},
		{ID: "x", Height: 99},
	})
	require.Equal(t, []string{"720p", "360p", "240p"}, names)
	// A 4K source with a 4K fallback in another codec, and two smaller
	// encodes, one of them in two codecs.
	names = renditionNames([]livehls.VideoTrack{
		{ID: "1", Width: 3840, Height: 2160},
		{ID: "2", Width: 1280, Height: 720},
		{ID: "3", Width: 3840, Height: 2160},
		{ID: "4", Width: 1920, Height: 1080},
		{ID: "5", Width: 1280, Height: 720},
	})
	require.Equal(t, []string{"1080p", "720p"}, names)
	require.Empty(t, renditionNames([]livehls.VideoTrack{{ID: "1", Height: 1080}}))
	require.Empty(t, renditionNames(nil))
}
