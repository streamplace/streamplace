package media

import (
	"bytes"
	"context"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
)

// audioCodecsOf returns the sorted distinct audio codecs in a bare .m4s segment.
func audioCodecsOf(t *testing.T, ctx context.Context, seg []byte) []string {
	t.Helper()
	events, err := unwrapMuxlEvents(ctx, seg)
	require.NoError(t, err)
	cat, _ := catalogAndTracks(events)
	require.NotNil(t, cat, "segment has a catalog")
	var out []string
	if cat.Audio != nil {
		for _, a := range cat.Audio.Renditions {
			out = append(out, a.Codec)
		}
	}
	sort.Strings(out)
	return out
}

// signedBareSegment signs the given fragmented fixture and returns the first
// GoP's bare canonical .m4s (all tracks) — the live ingest shape.
func signedBareSegment(t *testing.T, ctx context.Context, ms *MediaSignerLocal, fragPath string) []byte {
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
	var seg []byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" && seg == nil {
			seg = concatTracksSorted(ev.Tracks)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, seg, "expected at least one signed GoP")
	return seg
}

// TestCompleteAudioCodecs drives the validate-time codec completion directly:
// sign a single-audio (Opus) segment, then run completeAudioCodecs and confirm
// it transcodes + transcode-signs the missing AAC track so the segment carries
// both codecs and still verifies. This is the live-path machinery (gstreamer
// transcode + muxl canonicalize/remap + SignTranscode), exercised without an
// RTMP source so a hang/error surfaces locally with a stack trace.
func TestCompleteAudioCodecs(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)

	seg := signedBareSegment(t, ctx, ms, getFixture("h264-opus-frag.mp4"))

	srcCodecs := audioCodecsOf(t, ctx, seg)
	require.Len(t, srcCodecs, 1, "source should be single-audio, got %v", srcCodecs)

	// Build a MediaManager with a node transcode signer (reuse the test key).
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "test.example.com"}}
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	mm.nodeCert = ms.Cert
	mm.nodeKeyPEM = keyPEM
	mm.nodeSignerOnce.Do(func() {}) // mark built so transcodeSigner returns the preset cert/key

	completed, err := mm.completeAudioCodecs(ctx, seg)
	require.NoError(t, err)
	require.Greater(t, len(completed), len(seg), "completion should append a signed track")

	codecs := audioCodecsOf(t, ctx, completed)
	require.Len(t, codecs, 2, "expected both AAC and Opus after completion, got %v", codecs)
	hasAAC, hasOpus := false, false
	for _, c := range codecs {
		if isAACCodec(c) {
			hasAAC = true
		}
		if isOpusCodec(c) {
			hasOpus = true
		}
	}
	require.True(t, hasAAC, "expected an AAC track, got %v", codecs)
	require.True(t, hasOpus, "expected the original Opus track, got %v", codecs)

	// The completed segment must still verify end-to-end (every track).
	out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(completed))
	require.NoError(t, err)
	require.Contains(t, out, "track_id", "verify output should describe each track")

	// And it must media-parse without stalling: the extra (audio_1) track is
	// left unlinked in the parse pipeline, which must not block EOS. With the
	// -timeout on the test, a regression here shows up as a 30s watchdog stall.
	res, err := ValidateMP4Media(ctx, completed)
	require.NoError(t, err)
	require.NotEmpty(t, res.MediaData.Video, "3-track segment parses video")
	require.NotEmpty(t, res.MediaData.Audio, "3-track segment parses audio")
}
