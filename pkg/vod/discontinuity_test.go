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

// TestMetafileFlagsReconnectDiscontinuity proves the metafile builder flags a
// concatenated reconnect. It synthesizes a "reconnect" recording by processing
// the fixture twice and concatenating session 2's bare segments after session
// 1's full [init][segments]; session 2's segments restart their tfdt at ~0,
// exactly like a streamer who disconnected/reconnected. Regenerating the
// sidecars from that blob must flag exactly one discontinuity per track — the
// first segment of session 2 — and never the very first segment.
//
// Needs gstreamer (warmGST/streamThroughMuxl), so it runs in the cgo test
// containers, not a bare checkout.
func TestMetafileFlagsReconnectDiscontinuity(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestMetafileFlagsReconnectDiscontinuity")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)
	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	buildSession := func() (b []byte, initLen int64) {
		out := &bytes.Buffer{}
		hasher := bdasl.NewWriter()
		dst := teeWriter{hasher, out}
		st, err := blob.NewFileStore(t.TempDir())
		require.NoError(t, err)
		mb := newMetafileBuilder(ctx, st)
		_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, mb, signer.SignerInput)
		require.NoError(t, err)
		blob := out.Bytes()
		meta := mb.Finalize(hasher.CID(), int64(len(blob)))
		return blob, minFirstOffset(t, meta)
	}

	s1, _ := buildSession()
	s2, s2InitLen := buildSession()
	reconnect := append(append([]byte(nil), s1...), s2[s2InitLen:]...)

	// Regenerate the sidecars from the concatenated blob, exactly as finalize
	// (and VOD transfer) do.
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	h := bdasl.NewWriter()
	_, _ = h.Write(reconnect)
	cid := h.CID()
	w, err := store.NewWriter(ctx, BlobsPrefix+cid+".mp4", "video/mp4")
	require.NoError(t, err)
	_, err = w.Write(reconnect)
	require.NoError(t, err)
	require.NoError(t, w.Complete())

	meta, _, err := regenerateSidecars(ctx, store, cid, int64(len(reconnect)))
	require.NoError(t, err)
	require.NotEmpty(t, meta.Tracks)

	for tid, tr := range meta.Tracks {
		var discIdx []int
		for i, s := range tr.Segments {
			if s.Discontinuity {
				discIdx = append(discIdx, i)
			}
		}
		t.Logf("track %s: %d segments, discontinuities at %v", tid, len(tr.Segments), discIdx)
		require.Lenf(t, discIdx, 1, "track %s should have exactly one discontinuity (the reconnect boundary)", tid)
		require.Greaterf(t, discIdx[0], 0, "track %s discontinuity must not be the first segment", tid)
		require.Falsef(t, tr.Segments[0].Discontinuity, "track %s first segment must not be flagged", tid)
	}
}
