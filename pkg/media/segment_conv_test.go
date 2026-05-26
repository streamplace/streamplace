package media

import (
	"bytes"
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

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
