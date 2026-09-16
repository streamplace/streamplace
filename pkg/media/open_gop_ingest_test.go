package media

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

// makeOpenGOPFMP4 synthesizes what a broadcast encoder sends at scene cuts:
// open-GOP I-frames, I-slices that are not IDRs (x264 open-gop makes every
// keyframe after the first one of those), between IDRs, alongside AAC
// audio, in a fragmented MP4. h264parse flags them as keyframes.
func makeOpenGOPFMP4(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	desc := "videotestsrc num-buffers=90 pattern=ball ! video/x-raw,width=320,height=240,framerate=30/1 ! " +
		"x264enc key-int-max=10 bframes=2 b-adapt=false speed-preset=ultrafast option-string=open-gop=1:scenecut=0 ! h264parse ! mp4mux name=mux fragment-duration=500 ! appsink name=sink " +
		"audiotestsrc num-buffers=130 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! fdkaacenc ! aacparse ! mux."
	return runSynthPipeline(t, ctx, desc)
}

// videoFirstSampleNALs returns the NAL types of the first video sample of
// each signed segment's first event.
func videoFirstSampleNALs(t *testing.T, ctx context.Context, seg []byte) []int {
	t.Helper()
	evs, err := unwrapMuxlEvents(ctx, seg)
	require.NoError(t, err)
	for _, ev := range evs {
		if ev.Type != "segment" && ev.Type != "signed-segment" {
			continue
		}
		vb := ev.Tracks["1"]
		if len(vb) == 0 {
			return nil
		}
		return firstSampleNALTypes(vb)
	}
	return nil
}

// firstSampleNALTypes walks a canonical track fragment to its first sample
// and lists that sample's NAL types (length-prefixed).
func firstSampleNALTypes(b []byte) []int {
	var size int
	var mdat []byte
	walkBoxes(b, func(typ string, body []byte) bool {
		switch typ {
		case "moof":
			walkBoxes(body, func(t2 string, traf []byte) bool {
				if t2 != "traf" {
					return true
				}
				walkBoxes(traf, func(t3 string, trun []byte) bool {
					if t3 != "trun" || size != 0 || len(trun) < 8 {
						return true
					}
					flags := uint32(trun[1])<<16 | uint32(trun[2])<<8 | uint32(trun[3])
					p := 8
					if flags&1 != 0 {
						p += 4
					}
					if flags&4 != 0 {
						p += 4
					}
					if flags&0x100 != 0 {
						p += 4
					}
					if flags&0x200 != 0 && p+4 <= len(trun) {
						size = int(uint32(trun[p])<<24 | uint32(trun[p+1])<<16 | uint32(trun[p+2])<<8 | uint32(trun[p+3]))
					}
					return true
				})
				return true
			})
		case "mdat":
			if mdat == nil {
				mdat = body
			}
		}
		return true
	})
	if size == 0 || len(mdat) < size {
		return nil
	}
	return h264NALTypes(mdat[:size])
}

var _ = muxl.RunMuxlUnwrapEvents

// Every signed segment starts on an IDR, even when the encoder's keyframes
// include open-GOP I-frames: those stay inside the GoP of the IDR before
// them rather than starting a segment no decoder could begin on.
func TestIngestOpenGOPSegmentsStartOnIDR(t *testing.T) {
	ctx := context.Background()
	mp4 := makeOpenGOPFMP4(t, ctx)
	segs, err := runMP4ThroughIngestWorkerSegments(t, mp4, false)
	require.NoError(t, err)
	require.NotEmpty(t, segs)
	var nonIDR int
	for i, seg := range segs {
		nals := videoFirstSampleNALs(t, ctx, seg)
		require.NotEmpty(t, nals, "segment %d has a first video sample", i)
		hasIDR := false
		for _, n := range nals {
			if n == 5 {
				hasIDR = true
			}
		}
		if !hasIDR {
			nonIDR++
			t.Logf("segment %d starts with NALs %v", i, nals)
		}
	}
	require.Zero(t, nonIDR, "segments starting without an IDR")
	require.LessOrEqual(t, len(segs), 2, "one IDR in 90 frames (the other 8 keyframes are open-GOP I-frames): one GoP, not a segment per I-frame")
}

// The dual-codec completion re-segments its transcoded audio on the
// passed-through video's keyframes; with open-GOP I-frames in the source
// it must still cut exactly where the source segments were cut, or the
// audio chunks pair with the wrong segments and the completed track
// drifts from the video (HLS audio then lands outside the video's
// timeline). Every completed segment's audio must match its video.
func TestIngestOpenGOPCompletionStaysAligned(t *testing.T) {
	ctx := context.Background()
	mp4 := makeOpenGOPFMP4(t, ctx)
	segs, err := runMP4ThroughIngestWorkerSegments(t, mp4, true)
	require.NoError(t, err)
	require.NotEmpty(t, segs)
	for i, seg := range segs {
		evs, err := unwrapMuxlEvents(ctx, seg)
		require.NoError(t, err)
		cat, _ := catalogAndTracks(evs)
		require.NotNil(t, cat)
		require.NotNil(t, cat.Audio)
		require.Len(t, cat.Audio.Renditions, 2, "segment %d is dual-codec", i)
		scales := map[string]float64{}
		for _, v := range cat.Video.Renditions {
			scales[fmt.Sprint(v.TrackID())] = float64(v.Timescale())
		}
		for _, a := range cat.Audio.Renditions {
			scales[fmt.Sprint(a.TrackID())] = float64(a.Timescale())
		}
		for _, ev := range evs {
			if ev.Type != "segment" && ev.Type != "signed-segment" {
				continue
			}
			// The two audio tracks (the source's and the completed one)
			// must cover the same span: the completion cut where the
			// source did.
			var audios []float64
			for tid, d := range ev.Durations {
				if tid == "1" {
					continue
				}
				audios = append(audios, float64(d)/scales[tid])
			}
			require.Len(t, audios, 2, "segment %d has both audio tracks", i)
			require.InDelta(t, audios[0], audios[1], 0.1, "segment %d: audio tracks carry %.3fs and %.3fs", i, audios[0], audios[1])
			break
		}
	}
	require.LessOrEqual(t, len(segs), 2, "as many completed segments as source segments: one GoP")
}
