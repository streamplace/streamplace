package media

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/test/remote"
)

// TestStreamTranscoderNeedsReset covers the rebuild triggers for the per-DID
// continuous transcoder: a newer ingest session (a streamer reconnecting — a
// rapid stop/start that restarts the media timeline), a source-codec swap (RTMP/
// AAC ↔ WHIP/Opus flips the needed target), or a failed pipeline. In each case
// the live encoder is wrong for the incoming segment and must be torn down
// rather than fed (feeding a restarted timeline into it wedges the stream).
func TestStreamTranscoderNeedsReset(t *testing.T) {
	tr := &streamTranscoder{target: "opus", sessionID: 1} // adding Opus to an AAC source

	// Same codec + same session: keep the continuous encoder running.
	require.False(t, tr.needsReset("opus", 1), "same codec + same session keeps the encoder running")

	// Source codec swapped mid-session (AAC→Opus flips the needed target): reset.
	require.True(t, tr.needsReset("aac", 1), "source codec swapped (AAC→Opus) must reset")

	// A newer ingest session took over (streamer reconnected → restarted media
	// timeline): reset even with the same codec — the rapid-restart fix.
	require.True(t, tr.needsReset("opus", 2), "a newer ingest session must rebuild the transcoder")

	// A stale straggler from an OLDER session must NOT reset the newer encoder.
	require.False(t, tr.needsReset("opus", 0), "an older/stale session must not reset the current encoder")

	// A failed pipeline rebuilds on the next segment regardless.
	failed := &streamTranscoder{target: "aac", sessionID: 5, err: errors.New("pipeline died")}
	require.True(t, failed.needsReset("aac", 5), "a failed pipeline rebuilds on the next segment")
}

// A new worker consumer restarts the source timeline. Validation must rebuild
// its continuous AAC encoder, while successive GOPs from one consumer reuse it.
func TestFeedStreamTranscoderRebuildsOnNewSession(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	pub, err := atproto.ParsePubKey(ms.Signer.Public())
	require.NoError(t, err)
	ms.StreamerName = pub.DIDKey()
	ms.PrebuiltManifest = bytes.Replace(ms.PrebuiltManifest, []byte("did:example:rtmp-shadow"), []byte(ms.Streamer()), 1)
	segs := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.GreaterOrEqual(t, len(segs), 2, "fixture should produce multiple segments")

	mm := earlyWebRTCManager(t, ms)
	canonical := make(chan *NewSegmentNotification, len(segs)*2+1)
	mm.newSegmentSubs = []*segmentSubscriber{{queue: canonical}}
	private := mm.bus.SubscribeSegment(ctx, ms.Streamer(), WebRTCSourceRendition)
	defer mm.bus.UnsubscribeSegment(ctx, ms.Streamer(), WebRTCSourceRendition, private)
	readPrivate := func(ch *bus.SegChan) *bus.Seg {
		t.Helper()
		select {
		case segment := <-ch.C:
			return segment
		case <-time.After(5 * time.Second):
			t.Fatal("accepted source did not reach private playback")
			return nil
		}
	}
	firstSession, flushFirst := mm.validateSegment(ctx)
	defer flushFirst()
	require.NoError(t, firstSession(segs[0]))
	firstIDs := []string{readPrivate(private).Filepath}
	t1 := mm.transcoders[ms.Streamer()]
	require.NotNil(t, t1, "session 1 built a transcoder")
	t.Cleanup(func() {
		require.NoError(t, t1.Close())
		select {
		case <-t1.done:
		case <-time.After(20 * time.Second):
			t.Error("previous session's transcoder did not finish")
		}
	})
	for _, seg := range segs[1:] {
		require.NoError(t, firstSession(seg))
		firstIDs = append(firstIDs, readPrivate(private).Filepath)
		require.Same(t, t1, mm.transcoders[ms.Streamer()], "one consumer must keep a continuous encoder")
	}

	secondSession, flushSecond := mm.validateSegment(ctx)
	defer flushSecond()
	require.NoError(t, secondSession(segs[0]))
	newer := readPrivate(private)
	require.NotEqual(t, firstIDs[1], newer.Filepath, "the stale GOP must have a distinct identity")
	t2 := mm.transcoders[ms.Streamer()]
	require.NotNil(t, t2, "session 2 built a transcoder")
	require.NotSame(t, t1, t2,
		"a new ingest session must rebuild the transcoder, not reuse the previous session's continuous encoder")
	require.Positive(t, t1.sessionID)
	require.Greater(t, t2.sessionID, t1.sessionID)
	// A detached older worker may deliver another GOP after its replacement.
	// It must neither replace the newest playback cache nor enter the new AAC timeline.
	require.NoError(t, firstSession(segs[1]))
	require.Same(t, t2, mm.transcoders[ms.Streamer()])
	cached := mm.bus.SubscribeSegmentBuf(ctx, ms.Streamer(), WebRTCSourceRendition, 1)
	defer mm.bus.UnsubscribeSegment(ctx, ms.Streamer(), WebRTCSourceRendition, cached)
	require.Equal(t, newer.Filepath, readPrivate(cached).Filepath,
		"an older worker must not replace the current session's cached GOP")

	// The previous session's transcoder is flushed + torn down (async on reset).
	require.NoError(t, t2.Close(), "the rebuilt transcoder drains cleanly")
	select {
	case <-t1.done:
	case <-time.After(20 * time.Second):
		t.Fatal("the previous session's transcoder did not finish on reconnect")
	}
	counts := map[string]int{}
	for range len(segs) + 1 {
		select {
		case notification := <-canonical:
			counts[notification.Segment.ID]++
			var flat bytes.Buffer
			require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(notification.Muxl), "flat", &flat))
			require.Equal(t, flat.Bytes(), notification.Data, "presentation must include the completed audio track")
		case <-time.After(5 * time.Second):
			t.Fatal("accepted GOP did not complete")
		}
	}
	want := map[string]int{newer.Filepath: 1}
	for _, id := range firstIDs {
		want[id]++
	}
	require.Equal(t, want, counts, "each encoder must drain only its own accepted source GOPs")
	require.Empty(t, canonical, "a superseded worker must not produce another completed GOP")
}

// allSignedBareSegments signs the fragmented fixture per-segment and returns
// every GoP's bare canonical .m4s (the live ingest shape), in order.
func allSignedBareSegments(t testing.TB, ctx context.Context, ms *MediaSignerLocal, fragPath string) [][]byte {
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

// TestStreamTranscoderDegenerateTimestamps feeds a real WHIP-captured segment
// whose source video carries degenerate timestamps — several frames sharing a
// PTS, plus zero/N/A frame durations (a variable-frame-rate capture artifact
// that ffmpeg tolerates). GStreamer's qtdemux collapses those into buffers with
// no PTS, which made mp4mux abort the whole pipeline ("Could not multiplex
// stream"); the per-stream transcoder then produced NOTHING, so the stream
// silently lost its AAC track for its entire life (a prod incident on a 1080p60
// High-profile Opus stream). buildAudioTranscodePipeline now repairs the
// (discarded) passthrough video's timestamps before the muxer, so the segment
// must transcode to a verifiable dual-codec result instead of wedging.
func TestStreamTranscoderDegenerateTimestamps(t *testing.T) {
	ctx := context.Background()
	seg, err := os.ReadFile(remote.RemoteFixture("5b8ddcf569a66d3f8e4a8634d9d422c1848c0becd3c44075afaafdf05f7d731f/h264-opus-degenerate-ts.m4s"))
	require.NoError(t, err)

	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "test.example.com"}}

	var mu sync.Mutex
	var completed [][]byte
	tr := mm.newStreamTranscoder(ctx, "aac", ms.Cert, keyPEM, func(_ any, c []byte) {
		mu.Lock()
		completed = append(completed, c)
		mu.Unlock()
	})
	require.NoError(t, tr.Feed(seg, 0), "degenerate-timestamp segment must not wedge the muxer")
	require.NoError(t, tr.Close(),
		"pipeline must drain cleanly, not abort with 'Could not multiplex stream'")

	mu.Lock()
	defer mu.Unlock()
	// Before the PTS repair the muxer aborted and this segment was dropped whole.
	require.Len(t, completed, 1, "the fed segment must complete to a dual-codec result")

	c := completed[0]
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
	require.True(t, hasOpus, "keeps the source Opus track (got %v)", codecs)
	require.True(t, hasAAC, "gains a transcoded AAC track (got %v)", codecs)

	out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(c))
	require.NoError(t, err)
	require.NotContains(t, out, `"validation_state":"Invalid"`, "completed segment must validate")

	// The transcoded AAC must cover the GoP, not a sliver — the degenerate video
	// timing must not bleed into a truncated audio track.
	o, a := audioDurationsSeconds(t, ctx, c)
	require.Greater(t, a, 0.65*o,
		"transcoded AAC %.3fs far short of source Opus %.3fs", a, o)
}
