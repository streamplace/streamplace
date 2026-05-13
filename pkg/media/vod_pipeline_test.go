package media

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/test/remote"
)

// TestRunVODPipeline_h264Opus exercises the full VOD pipeline on the
// h264 + opus test fixture. The pipeline should:
//
//  1. Read the file via RandomAccessSrcBin
//  2. Parse via parsebin (yielding video/x-h264 + audio/x-opus pads)
//  3. Transcode opus -> AAC via fdkaacenc
//  4. Pass h264 through h264parse
//  5. Mux to fragmented MP4
//  6. Hand off through appsink to our writer
//
// We don't assert byte-level structure here; just that we got non-trivial
// fMP4 output that starts with the ftyp box.
//
// We skip withNoGSTLeaks for this test: parsebin loads ~160 typefind
// factories on first use that stay alive in gstreamer's global registry
// for the rest of the process. They register independently of the leak
// tracer's baseline, so the leak check inevitably reports them as leaks
// even after the pipeline is fully torn down. That's registry overhead,
// not a real leak.
func TestRunVODPipeline_h264Opus(t *testing.T) {
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestRunVODPipeline_h264Opus")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = RunVODPipeline(ctx, bytes.NewReader(fixture), int64(len(fixture)), out)
	require.NoError(t, err)
	require.Greater(t, out.Len(), 1024, "expected non-trivial fMP4 output")

	// ftyp box is at offset 4 in the standard MP4 layout: [size(4)] [type(4)].
	require.GreaterOrEqual(t, out.Len(), 8)
	require.Equal(t, "ftyp", string(out.Bytes()[4:8]), "expected output to start with ftyp box")
}

// TestRunVODPipeline_h264AAC_RocketLeague exercises a real-world
// recording (Rocket League gameplay, 30 s, 1080p60, h264 + AAC, no
// B-frames). This is the kind of upload an actual user would push,
// as opposed to the short fixture used by the smoke test above —
// real-world parsebin behavior around stream collections is what we
// want to lock down here.
func TestRunVODPipeline_h264AAC_RocketLeague(t *testing.T) {
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestRunVODPipeline_h264AAC_RocketLeague")

	fixture, err := os.ReadFile(remote.RemoteFixture("eb34493396ec3606e3f6d5543c7f695b6ec4b8959c083ffbd04623ea16ed9598/RocketLeague_30s_1sGOP-1080p60_NoBframes.mp4"))
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = RunVODPipeline(ctx, bytes.NewReader(fixture), int64(len(fixture)), out)
	require.NoError(t, err)
	require.Greater(t, out.Len(), 1024, "expected non-trivial fMP4 output")
	require.GreaterOrEqual(t, out.Len(), 8)
	require.Equal(t, "ftyp", string(out.Bytes()[4:8]), "expected output to start with ftyp box")
}
