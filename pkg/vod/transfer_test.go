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

// TestRegenerateSidecars is the load-bearing test for VOD transfer: it
// proves that the metafile + per-track init segments a node produces while
// *processing* a VOD can be reproduced byte-for-identically from just the
// finished content blob (which is the only thing TransferVOD pulls over the
// network).
//
// It runs the real processing pipeline to get a ground-truth metafile +
// init blobs in store A, then feeds only the resulting content blob through
// regenerateSidecars into a fresh store B and asserts the regenerated
// metafile matches exactly — same track set, same init CIDs, same segment
// byte offsets/sizes/durations — and that every init blob + the metafile
// landed in store B. If `muxl unwrap` ever stopped emitting per-track init
// bytes, or the offset accounting drifted, this fails.
func TestRegenerateSidecars(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestRegenerateSidecars")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)

	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	// --- store A: ground truth from the real processing pipeline ---------
	storeA, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	dst := teeWriter{hasher, out}

	mbA := newMetafileBuilder(ctx, storeA)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, mbA, signer.SignerInput)
	require.NoError(t, err)

	cid := hasher.CID()
	blobBytes := out.Bytes()
	metaA := mbA.Finalize(cid, int64(len(blobBytes)))
	require.NotEmpty(t, metaA.Tracks)

	// --- store B: regenerate from only the content blob ------------------
	storeB, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	// Place the content blob exactly where a download would have left it.
	key := BlobsPrefix + cid + ".mp4"
	w, err := storeB.NewWriter(ctx, key, transferMimeType)
	require.NoError(t, err)
	_, err = w.Write(blobBytes)
	require.NoError(t, err)
	require.NoError(t, w.Complete())

	metaB, initCIDs, err := regenerateSidecars(ctx, storeB, cid, int64(len(blobBytes)))
	require.NoError(t, err)

	// The regenerated metafile must equal the one the processing pipeline
	// produced. BlobCID/BlobSize are derived inputs; the meat is the track
	// table (codecs, init CIDs, and especially segment byte ranges).
	require.Equal(t, metaA.BlobCID, metaB.BlobCID)
	require.Equal(t, metaA.BlobSize, metaB.BlobSize)
	require.Equal(t, metaA.Tracks, metaB.Tracks, "regenerated metafile tracks differ from processing-time metafile")

	// Every init CID reported back was actually written into store B, and
	// matches the init the pipeline wrote into store A.
	require.NotEmpty(t, initCIDs)
	for _, tr := range metaB.Tracks {
		require.NotEmpty(t, tr.InitCID, "track missing initCid")
		require.Contains(t, initCIDs, tr.InitCID)

		rb, err := storeB.Open(ctx, BlobsPrefix+tr.InitCID+".mp4")
		require.NoError(t, err, "init blob %s not written to store B", tr.InitCID)
		bBytes := make([]byte, rb.Size())
		_, _ = rb.ReadAt(bBytes, 0)
		require.NoError(t, rb.Close())

		ra, err := storeA.Open(ctx, BlobsPrefix+tr.InitCID+".mp4")
		require.NoError(t, err, "init blob %s missing from store A", tr.InitCID)
		aBytes := make([]byte, ra.Size())
		_, _ = ra.ReadAt(aBytes, 0)
		require.NoError(t, ra.Close())

		require.Equal(t, aBytes, bBytes, "init blob %s bytes differ between stores", tr.InitCID)
		require.NoError(t, bdasl.Verify(tr.InitCID, bBytes), "init blob %s fails its own CID", tr.InitCID)
	}

	// The metafile JSON sidecar landed at blobs/<cid>.json in store B.
	mr, err := storeB.Open(ctx, BlobsPrefix+cid+".json")
	require.NoError(t, err, "metafile not written to store B")
	require.Positive(t, mr.Size())
	require.NoError(t, mr.Close())
}
