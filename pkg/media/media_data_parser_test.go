package media

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/test/remote"
)

func TestMediaDataParser(t *testing.T) {
	withNoGSTLeaks(t, func() {
		// Open input file
		inputFile, err := os.Open(getFixture("sample-segment.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"GStreamerFunc": {"ParseSegmentMediaData": 9}})
		mediaData, err := ParseSegmentMediaData(ctx, bs)
		require.NoError(t, err)
		require.NotNil(t, mediaData)
		require.False(t, mediaData.Video[0].BFrames, "Video should not have BFrames")
		require.Greater(t, mediaData.Duration, int64(0), "Video duration should not be empty")
	})
}

var bframeTestCasts = []struct {
	asset      string
	hasBFrames bool
}{
	{
		asset: "4c7f24d6a054aeee30dccc32f812184bd06abed2fd02fbebdfd2f24195adf1ce/no-bframes.mp4",
		hasBFrames: false,
	},
	{
		asset: "5ea6c4491bade0cdcad3770aa0b63b2cd7a580e233ee320d5bc2282503b26491/segment-with-bframes.mp4",
		hasBFrames: true,
	},
	{
		asset: "77c5a702fac8256bdeeb6c803326f8a80fe767d056792ab03d230e7c6e722230/2025-07-19T19-42-49-575Z.mp4",
		hasBFrames: false,
	},
}

func TestMediaDataParserBFrames(t *testing.T) {
	withNoGSTLeaks(t, func() {
		for _, test := range bframeTestCasts {
			inputFile, err := os.Open(remote.RemoteFixture(test.asset))
			require.NoError(t, err)
			defer inputFile.Close()
			bs, err := io.ReadAll(inputFile)
			require.NoError(t, err)

			ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"GStreamerFunc": {"ParseSegmentMediaData": 9}})
			mediaData, err := ParseSegmentMediaData(ctx, bs)
			require.NoError(t, err)
			require.NotNil(t, mediaData)
			require.Equal(t, test.hasBFrames, mediaData.Video[0].BFrames, "Video should have BFrames")
			require.Greater(t, mediaData.Duration, int64(0), "Video duration should not be empty")
		}
	})
}
