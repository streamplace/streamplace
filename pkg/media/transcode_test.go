package media

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/gstinit"
)

func TestAudioTranscodePipelineReleasesQueues(t *testing.T) {
	withNoGSTLeaks(t, func() {
		for _, target := range []string{"aac", "opus"} {
			pipeline, err := buildAudioTranscodePipeline(target)
			require.NoError(t, err)
			require.NoError(t, pipeline.BlockSetState(gst.StateNull))
		}
	})
}

func TestStreamTranscoderAACSource(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx := context.Background()
		input := makeH264AACFMP4(t, ctx, getFixture("5sec.mp4"))
		inputPath := filepath.Join(t.TempDir(), "aac.mp4")
		require.NoError(t, os.WriteFile(inputPath, input, 0o600))
		ms := newBareSegmentSigner(t)
		segs := allSignedBareSegments(t, ctx, ms, inputPath)
		require.NotEmpty(t, segs)
		keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
		require.NoError(t, err)
		mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "test.example.com"}}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		var completed [][]byte
		tr := mm.newStreamTranscoder(ctx, "opus", ms.Cert, keyPEM, func(_ any, seg []byte) {
			completed = append(completed, seg)
		})
		for i, seg := range segs {
			require.NoError(t, tr.Feed(seg, i))
		}
		require.NoError(t, tr.Close())
		require.NoError(t, ctx.Err(), "both demux branches finish before the watchdog")
		require.NotEmpty(t, completed)
		for _, seg := range completed {
			codecs := audioCodecsOf(t, context.Background(), seg)
			require.Len(t, codecs, 2)
			require.True(t, slices.ContainsFunc(codecs, isAACCodec))
			require.True(t, slices.ContainsFunc(codecs, isOpusCodec))
		}
	})
}

func BenchmarkAudioTranscodePipeline(b *testing.B) {
	gstinit.InitGST()
	for _, target := range []string{"aac", "opus"} {
		b.Run(target, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				pipeline, err := buildAudioTranscodePipeline(target)
				if err != nil {
					b.Fatal(err)
				}
				if err := pipeline.BlockSetState(gst.StateNull); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

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
