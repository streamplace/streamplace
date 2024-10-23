package media

import (
	"context"
	"os"
	"testing"

	_ "aquareum.tv/aquareum/pkg/media/mediatesting"
	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAudio(t *testing.T) {
	gst.Init(nil)
	ifile, err := os.Open(getFixture("sample-stream.mkv"))
	require.NoError(t, err)
	ofile, err := os.CreateTemp("", "*.mkv")
	defer os.Remove(ofile.Name())
	require.NoError(t, err)
	err = AddOpusToMKV(context.Background(), ifile, ofile)
	require.NoError(t, err)
	ofile.Close()
	info, err := os.Stat(ofile.Name())
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}

func TestThumbnail(t *testing.T) {
	mm := MediaManager{}
	gst.Init(nil)
	ifile, err := os.Open(getFixture("sample-segment.mp4"))
	require.NoError(t, err)
	ofile, err := os.CreateTemp("", "*.jpg")
	// defer os.Remove(ofile.Name())
	require.NoError(t, err)
	err = mm.Thumbnail(context.Background(), ifile, ofile)
	require.NoError(t, err)
	ofile.Close()
	info, err := os.Stat(ofile.Name())
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
	panic(ofile.Name())
}
