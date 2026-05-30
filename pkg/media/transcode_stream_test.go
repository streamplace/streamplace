package media

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
)

// TestStreamTranscoderNeedsReset covers the source-codec-swap detection: a
// streamer dropping RTMP/AAC and picking up WHIP/Opus (or vice versa) flips the
// needed target, which must reset the continuous transcoder rather than feed
// the new codec into a pipeline built for the old one.
func TestStreamTranscoderNeedsReset(t *testing.T) {
	tr := &streamTranscoder{target: "opus"} // adding Opus to an AAC source
	require.False(t, tr.needsReset("opus"), "same source codec keeps the encoder running")
	require.True(t, tr.needsReset("aac"), "source codec swapped (AAC→Opus) must reset")

	failed := &streamTranscoder{target: "aac", err: errors.New("pipeline died")}
	require.True(t, failed.needsReset("aac"), "a failed pipeline rebuilds on the next segment")
}

// allSignedBareSegments signs the fragmented fixture per-segment and returns
// every GoP's bare canonical .m4s (the live ingest shape), in order.
func allSignedBareSegments(t *testing.T, ctx context.Context, ms *MediaSignerLocal, fragPath string) [][]byte {
	t.Helper()
	frag, err := os.ReadFile(fragPath)
	require.NoError(t, err)
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(frag), eventCh)
		close(eventCh)
		errCh <- err
	}()
	var out [][]byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" {
			out = append(out, concatTracksSorted(ev.Tracks))
		}
	}
	require.NoError(t, <-errCh)
	return out
}

// audioDurationsSeconds re-segments a bare .m4s and returns its Opus and AAC
// audio-track durations in seconds (0 if absent).
func audioDurationsSeconds(t *testing.T, ctx context.Context, seg []byte) (opusSec, aacSec float64) {
	t.Helper()
	var fmp4 bytes.Buffer
	require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(seg), "fmp4", &fmp4))
	events, err := segmentMuxlEvents(ctx, fmp4.Bytes())
	require.NoError(t, err)
	cat, _ := catalogAndTracks(events)
	require.NotNil(t, cat)

	type trackInfo struct {
		codec string
		ts    uint32
	}
	info := map[string]trackInfo{}
	if cat.Audio != nil {
		for _, a := range cat.Audio.Renditions {
			info[strconv.FormatUint(uint64(a.TrackID()), 10)] = trackInfo{a.Codec, a.Timescale()}
		}
	}
	for _, ev := range events {
		if ev.Type != "segment" && ev.Type != "signed-segment" {
			continue
		}
		for tid, dur := range ev.Durations {
			x, ok := info[tid]
			if !ok || x.ts == 0 {
				continue
			}
			sec := float64(dur) / float64(x.ts)
			switch {
			case isOpusCodec(x.codec):
				opusSec += sec
			case isAACCodec(x.codec):
				aacSec += sec
			}
		}
	}
	return opusSec, aacSec
}

// TestStreamTranscoderGapless feeds every segment of one stream through the
// continuous transcoder and checks the result: each completed segment carries
// both codecs and verifies, and the transcoded (AAC) track's total duration
// matches the source (Opus) track's — i.e. it is NOT inflated by per-segment
// encoder priming, which is the whole point of running one continuous encoder.
func TestStreamTranscoderGapless(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	segs := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.GreaterOrEqual(t, len(segs), 2, "fixture should produce multiple segments to test continuity")

	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "test.example.com"}}
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)

	var mu sync.Mutex
	var completed [][]byte
	tr := mm.newStreamTranscoder(ctx, "aac", ms.Cert, keyPEM, func(_ any, c []byte) {
		mu.Lock()
		completed = append(completed, c)
		mu.Unlock()
	})

	for i, s := range segs {
		require.NoError(t, tr.Feed(s, i), "feed segment %d", i)
	}
	require.NoError(t, tr.Close())

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, completed, "transcoder emitted completed segments")

	var totalOpus, totalAAC float64
	for i, c := range completed {
		codecs := audioCodecsOf(t, ctx, c)
		hasOpus, hasAAC := false, false
		for _, cc := range codecs {
			if isOpusCodec(cc) {
				hasOpus = true
			}
			if isAACCodec(cc) {
				hasAAC = true
			}
		}
		require.True(t, hasOpus, "segment %d keeps the source Opus track (got %v)", i, codecs)
		require.True(t, hasAAC, "segment %d gains an AAC track (got %v)", i, codecs)

		out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(c))
		require.NoError(t, err, "segment %d verify", i)
		require.NotContains(t, out, `"validation_state":"Invalid"`, "segment %d must validate", i)

		o, a := audioDurationsSeconds(t, ctx, c)
		// Each completed segment must carry a FULL GoP of transcoded audio, not a
		// fraction of it. Relabeling the transcoded track to a free id runs it back
		// through muxl's canonicalize, which re-segments; cut audio-only (no video
		// keyframe to anchor on) it splits each ~2 s GoP into ~1 s pieces, and a bug
		// that extracted only the first piece silently HALVED the track — every
		// segment played ~1 s of AAC against ~2 s of Opus/video, audible as the
		// audio cutting out for the back half of each segment. This proportional
		// guard catches that (and any gross under-fill) independent of segment
		// count; finishTranscodedSegment now carries the video through as the cut
		// clock so the audio re-canonicalizes as one full segment per GoP.
		require.Greater(t, a, 0.65*o,
			"segment %d: transcoded AAC %.3fs far short of source Opus %.3fs (halved track?)", i, a, o)
		totalOpus += o
		totalAAC += a
	}

	t.Logf("completed=%d totalOpus=%.3fs totalAAC=%.3fs delta=%.3fs",
		len(completed), totalOpus, totalAAC, totalAAC-totalOpus)
	// Gapless: the transcoded track tracks the source duration with no per-segment
	// accumulation. The continuous encoder pays its priming ONCE at stream start
	// (the gappy per-segment transcoder this replaced paid 40–80 ms at EVERY
	// boundary), and each segment's audio is cut on the video keyframe — which need
	// not land on an AAC frame — so per-segment durations quantize by ±1 AAC frame
	// (~21 ms) around the source without drifting (measured ~0.15 ms/segment over a
	// 10-min real stream). The fixture is only a few short segments, too few to
	// average the one-time priming, so bound = priming + ~one frame per segment.
	tol := 0.040 + 0.025*float64(len(completed))
	require.InDelta(t, totalOpus, totalAAC, tol,
		"transcoded AAC total drifts from source Opus beyond priming + boundary quantization")
}
