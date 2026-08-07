package media

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeClipFile(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		width        int
		height       int
		audioCodec   string // "" = no audio expected
		rate         int
		channels     int
		wantDuration bool // false for fragmented files (duration lives in moof boxes)
	}{
		// The clip path produces flat MP4s (muxl "flat" wrap); the fragmented
		// fixture only checks that sample-entry metadata still extracts.
		{"h264+opus fragmented", "../../test/fixtures/h264-opus-frag.mp4", 320, 240, "x-opus", 48000, 1, false},
		{"h264+opus", "../../test/fixtures/short-video.mp4", 360, 202, "x-opus", 48000, 2, true},
		{"h264+opus frames", "../../test/fixtures/few-video-frames.mp4", 1080, 1920, "x-opus", 48000, 2, true},
		{"h264+aac", "../../test/fixtures/h264-aac.mp4", 320, 240, "mpeg", 44100, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.file)
			require.NoError(t, err)
			defer f.Close()
			res, err := ProbeClipFile(f)
			require.NoError(t, err)
			if tt.wantDuration {
				require.True(t, res.DurationMS > 0, "expected a duration")
			}
			require.NotNil(t, res.Video, "expected a video track")
			require.Equal(t, "h264", res.Video.Codec)
			require.Equal(t, tt.width, res.Video.Width)
			require.Equal(t, tt.height, res.Video.Height)
			if tt.audioCodec == "" {
				require.Nil(t, res.Audio)
				return
			}
			require.NotNil(t, res.Audio, "expected an audio track")
			require.Equal(t, tt.audioCodec, res.Audio.Codec)
			require.Equal(t, tt.rate, res.Audio.Rate)
			require.Equal(t, tt.channels, res.Audio.Channels)
		})
	}
}
