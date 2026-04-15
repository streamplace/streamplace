package media

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/test/remote"
)

func TestMP4ToMPEGTS(t *testing.T) {
	withNoGSTLeaks(t, func() {
		for _, file := range []string{
			getFixture("sample-segment.mp4"),
			getFixture("short-video.mp4"),
			remote.RemoteFixture("82d20ee62b02f1c3a727b3001f1fa939afb757f9f205fa438d7b5753e1253eef/2026-04-11T22-39-41-861Z-packetize-input-019d7eb3-6f24-776c-ba1b-2f909a2379d7.mp4")} {
			// Open input file
			inputFile, err := os.Open(file)
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

func TestMPEGTSVideoMP4AudioToMP4Invalid(t *testing.T) {
	withNoGSTLeaks(t, func() {
		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				return innerTestMPEGTSVideoMP4AudioToMP4Invalid(t)
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

func innerTestMPEGTSVideoMP4AudioToMP4Invalid(t *testing.T) error {
	// Join video and audio back together
	videoBuf := bytes.Buffer{}
	audioBuf := bytes.Buffer{}
	// Fill buffers with 1MB of random data

	rng := rand.New(rand.NewSource(42))
	randomData := make([]byte, 1024*1024) // 1MB
	_, err := rng.Read(randomData)
	require.NoError(t, err)
	_, err = videoBuf.Write(randomData)
	require.NoError(t, err)

	randomData = make([]byte, 1024*1024) // 1MB
	_, err = rng.Read(randomData)
	require.NoError(t, err)
	_, err = audioBuf.Write(randomData)
	require.NoError(t, err)

	buf := bytes.Buffer{}

	err = MPEGTSVideoMP4AudioToMP4(context.Background(), &videoBuf, &audioBuf, &buf)
	require.Empty(t, buf.Bytes())
	require.Error(t, err)
	return nil
}
