package vod

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
)

// TestFinalizeHeaderConcat is the gating test for live-to-VOD finalize: it
// proves that prepending the init `muxl unwrap` synthesizes for a stream's bare
// signed segments produces a content blob whose regenerated metafile is
// playback-correct — same per-track init CIDs and same per-segment sizes /
// durations as the processing pipeline's, with self-consistent (contiguous,
// header-offset) byte ranges.
//
// This is the assumption FinalizeLivestreamVOD rests on: the live S3 objects
// are bare segments, and finalize prepends one synthesized header so the blob
// is shaped [init][segments…] like an uploaded VOD. If muxl's bare-input init
// synthesis ever diverged from what the metafile builder assumes for a leading
// init, the byte ranges would be wrong and this fails.
//
// Like the other pkg/vod tests it needs gstreamer (warmGST/streamThroughMuxl),
// so it runs in the cgo test containers, not a bare checkout.
func TestFinalizeHeaderConcat(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestFinalizeHeaderConcat")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)

	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	// --- Ground truth: the processing pipeline's blob + metafile. ----------
	storeA, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	dst := teeWriter{hasher, out}
	mbA := newMetafileBuilder(ctx, storeA)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, mbA, signer.SignerInput)
	require.NoError(t, err)
	groundBlob := out.Bytes()
	metaA := mbA.Finalize(hasher.CID(), int64(len(groundBlob)))
	require.NotEmpty(t, metaA.Tracks)

	// The processing blob is [init][segments…]; the live S3 objects would be
	// the bare segments (no leading init). Strip the init to simulate them: it
	// occupies [0, firstSegmentOffset).
	initLen := minFirstOffset(t, metaA)
	require.Greater(t, initLen, int64(0))
	bareSegs := groundBlob[initLen:]

	// --- Finalize path: capture header, prepend, regenerate. ---------------
	storeB, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	// Write the bare segments as the single recorded "object".
	objKey := "live/obj0.m4s"
	wObj, err := storeB.NewWriter(ctx, objKey, "video/iso.segment")
	require.NoError(t, err)
	_, err = wObj.Write(bareSegs)
	require.NoError(t, err)
	require.NoError(t, wObj.Complete())

	// captureInitHeader is the real finalize helper: unwrap the bare object and
	// take the init event it emits.
	header, err := captureInitHeader(ctx, storeB, objKey)
	require.NoError(t, err)
	require.NotEmpty(t, header)

	newBlob := append(append([]byte(nil), header...), bareSegs...)
	cidB := func() string {
		h := bdasl.NewWriter()
		_, _ = h.Write(newBlob)
		return h.CID()
	}()
	contentKey := BlobsPrefix + cidB + ".mp4"
	wBlob, err := storeB.NewWriter(ctx, contentKey, "video/mp4")
	require.NoError(t, err)
	_, err = wBlob.Write(newBlob)
	require.NoError(t, err)
	require.NoError(t, wBlob.Complete())

	metaB, initCIDs, err := regenerateSidecars(ctx, storeB, cidB, int64(len(newBlob)))
	require.NoError(t, err)
	require.NotEmpty(t, initCIDs)

	// --- Correctness assertions. -------------------------------------------
	require.Equal(t, len(metaA.Tracks), len(metaB.Tracks), "track count")
	for tid, ta := range metaA.Tracks {
		tb, ok := metaB.Tracks[tid]
		require.Truef(t, ok, "track %s missing from regenerated metafile", tid)

		// Per-track identity must match: same init blob, codec, geometry.
		require.Equalf(t, ta.InitCID, tb.InitCID, "track %s initCID", tid)
		require.Equalf(t, ta.Codec, tb.Codec, "track %s codec", tid)
		require.Equalf(t, ta.Type, tb.Type, "track %s type", tid)
		require.Equalf(t, ta.Timescale, tb.Timescale, "track %s timescale", tid)
		require.Equalf(t, ta.Width, tb.Width, "track %s width", tid)
		require.Equalf(t, ta.Height, tb.Height, "track %s height", tid)
		require.Equalf(t, ta.Channels, tb.Channels, "track %s channels", tid)
		require.Equalf(t, ta.SampleRate, tb.SampleRate, "track %s sampleRate", tid)

		// Per-segment sizes/durations must match exactly; offsets may differ
		// only if the synthesized header length differs from the pipeline init.
		require.Equalf(t, len(ta.Segments), len(tb.Segments), "track %s segment count", tid)
		for i := range ta.Segments {
			require.Equalf(t, ta.Segments[i].Size, tb.Segments[i].Size, "track %s seg %d size", tid, i)
			require.Equalf(t, ta.Segments[i].DurationTicks, tb.Segments[i].DurationTicks, "track %s seg %d duration", tid, i)
			require.Equalf(t, ta.Segments[i].SampleCount, tb.Segments[i].SampleCount, "track %s seg %d sampleCount", tid, i)
		}
	}

	// The regenerated metafile must be self-consistent with the actual blob:
	// the earliest segment starts exactly after the header, and each track's
	// segments tile contiguously.
	require.Equal(t, int64(len(header)), minFirstOffset(t, metaB), "first segment must start right after the header")
	for tid, tb := range metaB.Tracks {
		for i := 1; i < len(tb.Segments); i++ {
			prev := tb.Segments[i-1]
			require.Equalf(t, prev.Offset+prev.Size, tb.Segments[i].Offset,
				"track %s seg %d not contiguous", tid, i)
		}
	}

	// Sanity: the prepended-header blob round-trips to its own CID, and the
	// header length matches the pipeline's init length (so a live VOD is
	// byte-structurally identical to an uploaded one).
	require.Equal(t, initLen, int64(len(header)), "synthesized header length should match the pipeline init length")
}

// minFirstOffset returns the smallest first-segment offset across all tracks —
// i.e. where the first segment byte begins, which equals the leading init's
// length in a [init][segments…] blob.
func minFirstOffset(t *testing.T, m *Metafile) int64 {
	t.Helper()
	var min int64 = -1
	for _, tr := range m.Tracks {
		require.NotEmpty(t, tr.Segments)
		off := tr.Segments[0].Offset
		if min < 0 || off < min {
			min = off
		}
	}
	return min
}
