package media

import (
	"context"
	"os"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/require"
)

func TestMP4ToMPEGTS(t *testing.T) {
	gst.Init(nil)

	// Open input file
	inputFile, err := os.Open(getFixture("sample-segment.mp4"))
	require.NoError(t, err)
	defer inputFile.Close()

	// Create temporary output file
	outputFile, err := os.CreateTemp("", "*.ts")
	require.NoError(t, err)
	defer os.Remove(outputFile.Name())
	defer outputFile.Close()

	// Convert MP4 to MPEG-TS
	dur, err := MP4ToMPEGTS(context.Background(), inputFile, outputFile)
	require.NoError(t, err)
	require.Greater(t, dur, int64(0), "Duration should be greater than 0")

	// Verify output file has content
	info, err := os.Stat(outputFile.Name())
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0), "Output file should not be empty")
}
