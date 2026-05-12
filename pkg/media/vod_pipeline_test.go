package media

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
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
