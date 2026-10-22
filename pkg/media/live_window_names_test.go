package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/livehls"
)

// Rendition names come from the window's video tracks, highest first, once
// per name, never the source track's name: transcoded renditions and those
// the streamer sent alongside the source alike, but only names with an
// H.264 track, the ones WebRTC plays.
func TestRenditionNames(t *testing.T) {
	const h264, av1 = "avc1.64001f", "av01.0.08M.08"
	names := renditionNames([]livehls.VideoTrack{
		{ID: "1", Codec: h264, Width: 1920, Height: 1080},
		{ID: "102", Codec: h264, Width: 426, Height: 240},
		{ID: "100", Codec: h264, Width: 1280, Height: 720},
		{ID: "101", Codec: h264, Width: 640, Height: 360},
		{ID: "x", Codec: h264, Height: 99},
	})
	require.Equal(t, []string{"720p", "360p", "240p"}, names)
	// An AV1 4K source with a 4K H.264 fallback, and three smaller encodes:
	// 1440p only in AV1, 1080p in H.264, 720p in both.
	names = renditionNames([]livehls.VideoTrack{
		{ID: "1", Codec: av1, Width: 3840, Height: 2160},
		{ID: "2", Codec: av1, Width: 1280, Height: 720},
		{ID: "3", Codec: h264, Width: 3840, Height: 2160},
		{ID: "4", Codec: av1, Width: 2560, Height: 1440},
		{ID: "5", Codec: h264, Width: 1920, Height: 1080},
		{ID: "6", Codec: h264, Width: 1280, Height: 720},
	})
	require.Equal(t, []string{"1080p", "720p"}, names)
	require.Empty(t, renditionNames([]livehls.VideoTrack{{ID: "1", Codec: h264, Height: 1080}}))
	require.Empty(t, renditionNames(nil))
}
