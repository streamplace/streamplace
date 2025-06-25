package media

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestMP4ToMPEGTS(t *testing.T) {
	withNoGSTLeaks(t, func() {
		for _, file := range []string{"sample-segment.mp4", "short-video.mp4"} {
			// Open input file
			inputFile, err := os.Open(getFixture(file))
			require.NoError(t, err)
			defer inputFile.Close()

			// Create a buffer for output
			buf := bytes.Buffer{}

			bs, err := io.ReadAll(inputFile)
			require.NoError(t, err)
			// Create temporary output file

			g, _ := errgroup.WithContext(context.Background())
			for range streamplaceTestCount {
				g.Go(func() error {
					_, err := MP4ToMPEGTS(context.Background(), bytes.NewReader(bs), &buf)
					return err
				})
			}
			err = g.Wait()
			require.NoError(t, err)
			// Convert MPEG-TS to MP4

			// Convert MP4 to MPEG-TS
			dur, err := MP4ToMPEGTS(context.Background(), bytes.NewReader(bs), &buf)
			require.NoError(t, err)
			require.Greater(t, dur, int64(0), "Duration should be greater than 0")

			// Verify buffer has content
			require.Greater(t, buf.Len(), 0, "Output buffer should not be empty")
		}
	})
}

func TestMP4ToMPEGTSInvalid(t *testing.T) {
	withNoGSTLeaks(t, func() {

		inputFile, err := os.Open(getFixture("sample-segment.mpegts"))
		require.NoError(t, err)
		defer inputFile.Close()

		buf := bytes.Buffer{}

		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		// Convert MP4 to MPEG-TS
		_, err = MP4ToMPEGTS(context.Background(), bytes.NewReader(bs), &buf)
		require.ErrorContains(t, err, "This file is invalid and cannot be played")
	})
}

// func TestNoRealtime(t *testing.T) {
// 	gst.Init(nil)

// 	// Open input file
// 	inputFile, err := os.Open(getFixture("5sec.mp4"))
// 	require.NoError(t, err)
// 	defer inputFile.Close()

// 	// Create a buffer for output
// 	tsBuf := bytes.Buffer{}

// 	// Convert MP4 to MPEG-TS
// 	start := time.Now()
// 	dur, err := MP4ToMPEGTS(context.Background(), inputFile, &tsBuf)
// 	require.NoError(t, err)
// 	require.Greater(t, dur, int64(0), "Duration should be greater than 0")
// 	elapsed := time.Since(start)
// 	require.Less(t, elapsed, 4*time.Second, "MP4 to MPEG-TS conversion should take less than 4 seconds")
// 	require.Greater(t, tsBuf.Len(), 0, "MPEG-TS buffer should not be empty")

// 	// Convert back to MP4
// 	mp4Buf := bytes.Buffer{}
// 	start = time.Now()
// 	err = MPEGTSToMP4(context.Background(), bytes.NewReader(tsBuf.Bytes()), &mp4Buf)
// 	require.NoError(t, err)
// 	elapsed = time.Since(start)
// 	require.Less(t, elapsed, 4*time.Second, "MPEG-TS to MP4 conversion should take less than 4 seconds")
// 	require.Greater(t, mp4Buf.Len(), 0, "MP4 buffer should not be empty")
// }

func TestMPEGTSToMP4(t *testing.T) {
	withNoGSTLeaks(t, func() {

		// Open input file
		inputFile, err := os.Open(getFixture("sample-segment.mpegts"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)
		// Create temporary output file
		buf := bytes.Buffer{}

		g, _ := errgroup.WithContext(context.Background())
		for i := 0; i < streamplaceTestCount; i++ {
			g.Go(func() error {
				return MPEGTSToMP4(context.Background(), bytes.NewReader(bs), &buf)
			})
		}
		err = g.Wait()
		// Convert MPEG-TS to MP4

		require.NoError(t, err)
		require.Greater(t, buf.Len(), 0, "Output file should not be empty")
	})
}

func TestMP4ToMPEGTSVideoMP4Audio(t *testing.T) {
	withNoGSTLeaks(t, func() {

		// Open input file
		inputFile, err := os.Open(getFixture("5sec.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()

		// Create buffers for output
		videoBuf := bytes.Buffer{}
		audioBuf := bytes.Buffer{}

		// Split MP4 into MPEG-TS video and MP4 audio
		start := time.Now()
		err = MP4ToMPEGTSVideoMP4Audio(context.Background(), inputFile, &videoBuf, &audioBuf)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.Less(t, elapsed, 4*time.Second, "MP4 to MPEG-TS/MP4 conversion should take less than 4 seconds")

		// Verify outputs
		require.Greater(t, videoBuf.Len(), 0, "Video buffer should not be empty")
		require.Greater(t, audioBuf.Len(), 0, "Audio buffer should not be empty")

		// Join video and audio back together
		buf := bytes.Buffer{}
		start = time.Now()
		err = MPEGTSVideoMP4AudioToMP4(context.Background(), &videoBuf, &audioBuf, &buf)
		require.NoError(t, err)
		require.Greater(t, buf.Len(), 0, "Output buffer should not be empty")
		elapsed = time.Since(start)
		require.Less(t, elapsed, 4*time.Second, "MPEG-TS/MP4 to MP4 conversion should take less than 4 seconds")
	})
}
