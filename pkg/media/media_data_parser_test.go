package media

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/log"
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
		require.Greater(t, mediaData.Duration, int64(0), "Video duration should not be empty")
	})
}
