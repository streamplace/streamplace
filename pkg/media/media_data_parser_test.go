package media

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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

// The two real demuxed tracks finish on native threads. Repeated EOS processing
// must preserve both tracks and stay race-free under the race detector.
func TestMediaDataParserConcurrentEOS(t *testing.T) {
	var logs logCapture
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)
	previousVerbosity := flag.Lookup("v").Value.String()
	require.NoError(t, flag.Set("v", "3"))
	t.Cleanup(func() { require.NoError(t, flag.Set("v", previousVerbosity)) })
	withNoGSTLeaks(t, func() {
		bs, err := os.ReadFile(getFixture("sample-segment.mp4"))
		require.NoError(t, err)
		g, ctx := errgroup.WithContext(context.Background())
		g.SetLimit(4)
		for range 16 {
			g.Go(func() error {
				meta, err := ParseSegmentMediaData(ctx, bs)
				if err != nil {
					return err
				}
				if len(meta.Video) != 1 || len(meta.Audio) != 1 || meta.Duration <= 0 {
					return fmt.Errorf("EOS lost media tracks or duration: %+v", meta)
				}
				if meta.Video[0].Width <= 0 || meta.Video[0].Height <= 0 || meta.Audio[0].Rate <= 0 || meta.Audio[0].Channels <= 0 {
					return fmt.Errorf("EOS lost video dimensions or audio configuration: %+v", meta)
				}
				return nil
			})
		}
		require.NoError(t, g.Wait())
	})
	// The fixture's initial zero-duration picture warns once per parse.
	require.Equal(t, 16, strings.Count(logs.String(), "no duration found for track"))
	require.NotContains(t, logs.String(), "level=ERROR")
}

func BenchmarkParseSegmentMediaData(b *testing.B) {
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.Cleanup(func() { slog.SetDefault(previousLogger) })
	previousVerbosity := flag.Lookup("v").Value.String()
	require.NoError(b, flag.Set("v", "0"))
	b.Cleanup(func() { require.NoError(b, flag.Set("v", previousVerbosity)) })
	bs, err := os.ReadFile(getFixture("sample-segment.mp4"))
	require.NoError(b, err)
	b.ReportAllocs()
	b.SetBytes(int64(len(bs)))
	for b.Loop() {
		meta, err := ParseSegmentMediaData(context.Background(), bs)
		if err != nil {
			b.Fatal(err)
		}
		if meta.Duration <= 0 {
			b.Fatal("missing media duration")
		}
	}
}

func TestMediaDataParserMissingVideo(t *testing.T) {
	var logs logCapture
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)
	previousVerbosity := flag.Lookup("v").Value.String()
	require.NoError(t, flag.Set("v", "3"))
	t.Cleanup(func() { require.NoError(t, flag.Set("v", previousVerbosity)) })
	withNoGSTLeaks(t, func() {
		synthCtx, stopSynth := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopSynth()
		flat := runSynthPipeline(t, synthCtx,
			"audiotestsrc num-buffers=48 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! opusenc ! mp4mux fragment-duration=500 ! appsink name=sink")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		meta, err := ParseSegmentMediaData(ctx, flat)
		require.ErrorContains(t, err, "no video in segment")
		require.Nil(t, meta)
		// The demux EOS error must stop parsing before the missing sink stalls.
		require.NoError(t, ctx.Err())
	})
	require.Contains(t, logs.String(), "expected at least 2 tracks (video + audio), got 1")
}
