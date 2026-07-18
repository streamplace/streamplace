package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/livehls"
)

// makeMultitrackMKV synthesizes an MKV carrying TWO H.264 video tracks (640x360
// and 320x180) plus AAC audio — the shape an OBS eRTMP multitrack push takes
// after MistServer re-muxes it into the MKV exec output that feeds
// `streamplace live`.
func makeMultitrackMKV(t *testing.T, ctx context.Context, seconds int) []byte {
	t.Helper()
	gstinit.InitGST()
	desc := strings.Join([]string{
		fmt.Sprintf("videotestsrc num-buffers=%d ! video/x-raw,width=640,height=360,framerate=30/1 ! x264enc tune=zerolatency key-int-max=30 speed-preset=veryfast ! h264parse ! matroskamux name=mux streamable=true ! appsink name=sink", seconds*30),
		fmt.Sprintf("videotestsrc num-buffers=%d ! video/x-raw,width=320,height=180,framerate=30/1 ! x264enc tune=zerolatency key-int-max=30 speed-preset=veryfast ! h264parse ! mux.", seconds*30),
		fmt.Sprintf("audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.", seconds*47),
	}, "\n")
	pipeline, err := gst.NewPipelineFromString(desc)
	require.NoError(t, err)

	sinkEle, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	var buf bytes.Buffer
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &buf),
	})

	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	require.NoError(t, <-busErr, "synthesize multitrack MKV")
	require.NotEmpty(t, buf.Bytes())
	return buf.Bytes()
}

// TestMKVIngestMultitrackEndToEnd feeds a synthesized 2-video-track MKV (the
// MistServer exec-output shape) through the isolated ingest worker and asserts
// the signed segments carry both video tracks through the real ValidateMP4Media
// path — and that the live HLS master playlist exposes both renditions.
func TestMKVIngestMultitrackEndToEnd(t *testing.T) {
	ctx := context.Background()
	mkv := makeMultitrackMKV(t, ctx, 4)

	segs, err := runMKVThroughIngestWorkerSegments(t, mkv, false)
	require.NoError(t, err, "multitrack MKV ingests cleanly")
	require.NotEmpty(t, segs, "expected at least one signed segment")

	// The first fragment can be a startup runt, so find the first segment that
	// actually carries both video tracks.
	var res *ValidationResult
	var fullSeg []byte
	for _, seg := range segs {
		r, verr := ValidateMP4Media(ctx, seg)
		require.NoError(t, verr)
		require.NotNil(t, r.MediaData)
		if len(r.MediaData.Video) == 2 {
			res, fullSeg = r, seg
			break
		}
	}
	require.NotNil(t, res, "no segment carried both video tracks (got %d segments)", len(segs))
	require.Equal(t, 640, res.MediaData.Video[0].Width, "video_0 should be the first MKV video track (640x360)")
	require.Equal(t, 360, res.MediaData.Video[0].Height)
	require.Equal(t, 320, res.MediaData.Video[1].Width, "video_1 should be the second MKV video track (320x180)")
	require.Equal(t, 180, res.MediaData.Video[1].Height)
	require.NotEmpty(t, res.MediaData.Audio, "segment should carry the audio track")

	// Fold the segment into a live-HLS window: the master playlist must expose
	// one variant per source video track.
	mm := &MediaManager{liveWindows: map[string]*livehls.Writer{}}
	mm.feedLiveWindow(ctx, "did:test:streamer", fullSeg, true)
	w := mm.GetLiveWindow("did:test:streamer")
	require.NotNil(t, w, "live window created on feed")
	master := w.MasterPlaylist(func(tid string) string { return tid + ".m3u8" })
	require.Equal(t, 2, strings.Count(master, "#EXT-X-STREAM-INF"), "one HLS variant per video track:\n%s", master)
	require.Contains(t, master, "RESOLUTION=640x360")
	require.Contains(t, master, "RESOLUTION=320x180")
}
