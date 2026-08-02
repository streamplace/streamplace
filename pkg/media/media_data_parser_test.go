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
	segmentsWithoutBFrames := []string{
		remote.RemoteFixture("d63d26050db9a60c0944b4c2e2b1d052c4350a2a8a877324c7b0b7e7a0c1ae27/bframe-false-positive.mp4"),
		getFixture("sample-segment.mp4"),
		remote.RemoteFixture("604bebf51c97f27aa07a8952462ac9885dd963f7a88375154217f59db32e1573/2025-11-18T01-10-56-292Z-signed-segment.mp4"),
		remote.RemoteFixture("1083078df7b88bf2b658a9895f54a733879a665e8b8748706ab1a0f7ab15efdb/2026-04-11T22-15-30-041Z-muxl_segment_input.fmp4"),
		remote.RemoteFixture("82d20ee62b02f1c3a727b3001f1fa939afb757f9f205fa438d7b5753e1253eef/2026-04-11T22-39-41-861Z-packetize-input-019d7eb3-6f24-776c-ba1b-2f909a2379d7.mp4"),
		remote.RemoteFixture("45c56c6e95c5babfaa86f1d07e94e0c54faefc8e4d334995d2fd5fcc9dbb4de2/segment-with-edit-list-bug.mp4"),
	}
	withNoGSTLeaks(t, func() {
		for _, segment := range segmentsWithoutBFrames {
			// Open input file
			inputFile, err := os.Open(segment)
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
		}
	})
}

func TestMediaDataParserBFrames(t *testing.T) {
	withNoGSTLeaks(t, func() {
		inputFile, err := os.Open(remote.RemoteFixture("5ea6c4491bade0cdcad3770aa0b63b2cd7a580e233ee320d5bc2282503b26491/segment-with-bframes.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"GStreamerFunc": {"ParseSegmentMediaData": 9}})
		mediaData, err := ParseSegmentMediaData(ctx, bs)
		require.NoError(t, err)
		require.NotNil(t, mediaData)
		require.True(t, mediaData.Video[0].BFrames, "Video should have BFrames")
		require.Greater(t, mediaData.Duration, int64(0), "Video duration should not be empty")
	})
}

// sample-segment.mp4 is 59.75fps (239/4); its video track's last PTS − first
// PTS is ~785.93ms, and the full span including the final sample's own duration
// is ~802.67ms. The parser must report the full span — dropping the last frame
// undercounts every segment's duration and inflates bitrate calculations.
func TestMediaDataParserDurationIncludesLastSample(t *testing.T) {
	withNoGSTLeaks(t, func() {
		inputFile, err := os.Open(getFixture("sample-segment.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"GStreamerFunc": {"ParseSegmentMediaData": 9}})
		mediaData, err := ParseSegmentMediaData(ctx, bs)
		require.NoError(t, err)
		require.NotNil(t, mediaData)
		require.InDelta(t, 802_666_666, mediaData.Duration, 2_000_000, "duration must include the final sample")
	})
}

func TestMediaDataParserVideoHeaderWithNoVideo(t *testing.T) {
	withNoGSTLeaks(t, func() {
		inputFile, err := os.Open(remote.RemoteFixture("0aa38ed08bb6b6b0ae5f4891a97244717e2c952d5ca878e34450729770f7ca53/2025-11-16T23-05-04-512Z-converge-segment-did-key-zQ3shkzEYN8UrJoRAGS6pgPodXjdg8kF2fXQNGfJhpg3x4KJT.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"GStreamerFunc": {"ParseSegmentMediaData": 9}})
		mediaData, err := ParseSegmentMediaData(ctx, bs)
		require.ErrorContains(t, err, "no video in segment")
		require.Nil(t, mediaData)
	})
}
