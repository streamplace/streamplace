package renditions

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func TestBuildSourceRenditions(t *testing.T) {
	vid := func(w, h int64) placestream.Segment_Video {
		return placestream.Segment_Video{Codec: "h264", Width: w, Height: h}
	}

	t.Run("single track keeps plain source name", func(t *testing.T) {
		spseg := &placestream.Segment{Video: []placestream.Segment_Video{vid(1920, 1080)}}
		rs := BuildSourceRenditions(spseg, 5_000_000)
		require.Len(t, rs, 1)
		require.Equal(t, "source", rs[0].Name)
		require.Equal(t, int64(1920), rs[0].Width)
		require.Equal(t, 5_000_000, rs[0].Bitrate)
	})

	t.Run("multitrack: top track by pixels is source regardless of wire order", func(t *testing.T) {
		spseg := &placestream.Segment{Video: []placestream.Segment_Video{vid(1280, 720), vid(1920, 1080), vid(854, 480)}}
		rs := BuildSourceRenditions(spseg, 8_000_000)
		require.Len(t, rs, 3)
		require.Equal(t, "source", rs[0].Name)
		require.Equal(t, int64(1080), rs[0].Height)
		require.Equal(t, "source-720p", rs[1].Name)
		require.Equal(t, "source-480p", rs[2].Name)
		// only the top track carries the (whole-segment) bitrate
		require.Equal(t, 8_000_000, rs[0].Bitrate)
		require.Zero(t, rs[1].Bitrate)
		require.Zero(t, rs[2].Bitrate)
	})

	t.Run("same-height tracks disambiguate with per-height counter", func(t *testing.T) {
		spseg := &placestream.Segment{Video: []placestream.Segment_Video{vid(1920, 1080), vid(1280, 720), vid(720, 720)}}
		rs := BuildSourceRenditions(spseg, 0)
		require.Len(t, rs, 3)
		require.Equal(t, "source", rs[0].Name)
		names := []string{rs[1].Name, rs[2].Name}
		require.Contains(t, names, "source-720p")
		// the second 720p track gets the counter suffix
		require.Contains(t, names, "source-720p-2")
	})

	t.Run("third same-height track keeps incrementing", func(t *testing.T) {
		spseg := &placestream.Segment{Video: []placestream.Segment_Video{vid(1920, 1080), vid(1280, 720), vid(720, 720), vid(640, 720)}}
		rs := BuildSourceRenditions(spseg, 0)
		require.Len(t, rs, 4)
		require.Equal(t, "source", rs[0].Name)
		names := []string{rs[1].Name, rs[2].Name, rs[3].Name}
		require.Contains(t, names, "source-720p")
		require.Contains(t, names, "source-720p-2")
		// regression: the counter previously restarted, colliding with -2
		require.Contains(t, names, "source-720p-3")
	})
}
