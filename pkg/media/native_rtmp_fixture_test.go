package media

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/muxl"
)

// The only difference from native RTMP ingest is filesrc in place of rtmp2src.
func nativeRTMPSignedFixture(t *testing.T, ctx context.Context, ms *MediaSignerLocal) [][]byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.flv")
	gstinit.InitGST()
	source, err := gst.NewPipelineFromString(fmt.Sprintf(
		"videotestsrc num-buffers=120 ! video/x-raw,format=I420,width=160,height=90,framerate=30/1 ! x264enc tune=zerolatency speed-preset=ultrafast key-int-max=30 bframes=0 ! h264parse name=sourcevideo ! queue ! flvmux name=mux streamable=true ! filesink location=%s "+
			"audiotestsrc num-buffers=187 samplesperbuffer=1024 freq=440 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc bitrate=128000 ! aacparse name=sourceaudio ! queue ! mux.", path))
	require.NoError(t, err)
	defer func() { require.NoError(t, source.SetState(gst.StateNull)) }()
	// Repeated encoder tags make flvmux rewrite metadata and reset demux codec data.
	for _, name := range []string{"sourcevideo", "sourceaudio"} {
		ele, err := source.GetElementByName(name)
		require.NoError(t, err)
		ele.GetStaticPad("src").AddProbe(gst.PadProbeTypeEventDownstream, func(_ *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
			if info.GetEvent().Type() == gst.EventTypeTag {
				return gst.PadProbeDrop
			}
			return gst.PadProbeOK
		})
	}
	sourceDone := make(chan error, 1)
	go func() { sourceDone <- HandleBusMessages(ctx, source) }()
	require.NoError(t, source.SetState(gst.StatePlaying))
	require.NoError(t, <-sourceDone)
	require.NoError(t, source.SetState(gst.StateNull))
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	p, err := gst.NewPipelineFromString(fmt.Sprintf("filesrc location=%s ! flvdemux name=demux demux.audio ! queue ! aacparse name=audioenc demux.video ! queue ! h264parse name=parse", path))
	require.NoError(t, err)
	defer func() { require.NoError(t, p.SetState(gst.StateNull)) }()
	var mu sync.Mutex
	var segs [][]byte
	signer, done, err := muxlSignSegmentElem(ingestCtx, &config.CLI{}, ms.SignSegmentStream, func(_ context.Context, seg []byte) error {
		mu.Lock()
		segs = append(segs, seg)
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, p.Add(signer))
	for _, name := range []string{"parse", "audioenc"} {
		ele, err := p.GetElementByName(name)
		require.NoError(t, err)
		require.NoError(t, ele.Link(signer))
	}
	bus := make(chan error, 1)
	go func() { bus <- HandleBusMessages(ingestCtx, p) }()
	require.NoError(t, p.SetState(gst.StatePlaying))
	select {
	case err := <-bus:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Native sink EOS must drain the signer without caller cancellation.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("native signer did not drain")
	}
	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(segs), 4)
	ends := map[string]uint64{}
	for i, seg := range segs {
		verified, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(seg))
		require.NoError(t, err)
		require.NotContains(t, verified, `"validation_state":"Invalid"`)
		events, err := unwrapMuxlEvents(ctx, seg)
		require.NoError(t, err)
		cat, _ := catalogAndTracks(events)
		require.NotNil(t, cat)
		scales := map[string]uint32{}
		if cat.Audio != nil {
			for _, a := range cat.Audio.Renditions {
				scales[fmt.Sprint(a.TrackID())] = a.Timescale()
			}
		}
		if cat.Video != nil {
			for _, v := range cat.Video.Renditions {
				scales[fmt.Sprint(v.TrackID())] = v.Timescale()
			}
		}
		for _, ev := range events {
			for tid, start := range ev.FirstDecodeTimes {
				require.NotZero(t, scales[tid])
				require.Positive(t, ev.Durations[tid])
				if i > 0 {
					require.EqualValues(t, ends[tid], start, "source sample intervals must be contiguous")
				}
				ends[tid] = start + ev.Durations[tid]
				if tid == "1" && i < len(segs)-1 {
					require.InDelta(t, 1.0, float64(ev.Durations[tid])/float64(scales[tid]), .001)
				}
				t.Logf("native source segment=%d track=%s timescale=%d start=%.9fs end=%.9fs duration=%.9fs", i, tid, scales[tid], float64(start)/float64(scales[tid]), float64(start+ev.Durations[tid])/float64(scales[tid]), float64(ev.Durations[tid])/float64(scales[tid]))
			}
		}
	}
	return segs
}
