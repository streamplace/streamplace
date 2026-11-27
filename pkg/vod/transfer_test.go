package vod

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
)

// TestRegenerateSidecars is the load-bearing test for VOD transfer: it
// proves that the metafile + per-track init segments a node produces while
// *processing* a VOD can be reproduced byte-for-identically from just the
// finished content blob (which is the only thing TransferVOD pulls over the
// network). This exercises the legacy [init][segments] shape, whose CID is
// the whole-file hash; TestTransferVODFlatBlob covers the flat shape.
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

// TestTransferVODFlatBlob covers the current on-wire blob shape: a
// [flat-header][fragments] MP4 whose CID is the BDASL of the fragments alone,
// so the whole downloaded file hashes to a different CID. The receiving node
// must verify against the canonical fragment stream and regenerate
// fragment-relative sidecars (FlatHeaderSize set) rather than assuming a
// legacy leading-init blob.
func TestTransferVODFlatBlob(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestTransferVODFlatBlob")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)
	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	// Build the source blob exactly as ProcessVOD does: hash the bare
	// fragments, synthesize the flat header, assemble header ++ fragments.
	srcStore, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	fragments := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	mb := newFragmentMetafileBuilder(ctx, srcStore)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), teeWriter{hasher, fragments}, mb, signer.SignerInput)
	require.NoError(t, err)
	contentCID := hasher.CID()

	stagingKey := "stage/frag.m4s"
	require.NoError(t, writeBlob(ctx, srcStore, stagingKey, fragments.Bytes()))
	header, err := synthFlatHeaderForObjects(ctx, srcStore, []string{stagingKey})
	require.NoError(t, err)
	contentKey := BlobsPrefix + contentCID + ".mp4"
	require.NoError(t, assembleContentBlob(ctx, srcStore, header, []string{stagingKey}, contentKey))

	flat := readTestBlob(t, ctx, srcStore, contentKey)
	require.Equal(t, int64(len(header))+int64(fragments.Len()), int64(len(flat)))
	require.NotEqual(t, contentCID, bdasl.CID(flat), "premise: a flat blob's whole-file CID differs from its fragment CID")

	srcMeta := mb.Finalize(contentCID, int64(fragments.Len()))
	srcMeta.FlatHeaderSize = int64(len(header))

	const did = "did:plc:transfertest"
	serve := func(t *testing.T, body []byte) *httptest.Server {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/xrpc/place.stream.playback.getVideoBlob", r.URL.Path)
			assert.Equal(t, contentCID, r.URL.Query().Get("cid"))
			assert.Equal(t, did, r.URL.Query().Get("did"))
			_, _ = w.Write(body)
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("verifies fragments and rebuilds sidecars", func(t *testing.T) {
		srv := serve(t, flat)
		dstStore, err := blob.NewFileStore(t.TempDir())
		require.NoError(t, err)
		base, err := url.Parse(srv.URL)
		require.NoError(t, err)

		n, err := downloadContentBlob(ctx, srv.Client(), dstStore, base, contentKey, contentCID, did)
		require.NoError(t, err)
		require.Equal(t, int64(len(flat)), n)
		require.Equal(t, flat, readTestBlob(t, ctx, dstStore, contentKey))

		meta, initCIDs, err := ensureSidecars(ctx, dstStore, contentCID, n)
		require.NoError(t, err)
		require.Equal(t, srcMeta.BlobCID, meta.BlobCID)
		require.Equal(t, srcMeta.BlobSize, meta.BlobSize)
		require.Equal(t, int64(len(header)), meta.FlatHeaderSize)
		require.Equal(t, srcMeta.Tracks, meta.Tracks, "regenerated sidecars must match the source's")

		require.NotEmpty(t, initCIDs)
		for _, initCID := range initCIDs {
			want := readTestBlob(t, ctx, srcStore, BlobsPrefix+initCID+".mp4")
			got := readTestBlob(t, ctx, dstStore, BlobsPrefix+initCID+".mp4")
			require.Equal(t, want, got, "init blob %s", initCID)
			require.NoError(t, bdasl.Verify(initCID, got))
		}
	})

	t.Run("rejects fragments that do not match the CID", func(t *testing.T) {
		corrupt := append([]byte(nil), flat...)
		corrupt[len(corrupt)-8] ^= 0xFF
		srv := serve(t, corrupt)
		dstStore, err := blob.NewFileStore(t.TempDir())
		require.NoError(t, err)
		base, err := url.Parse(srv.URL)
		require.NoError(t, err)

		_, err = downloadContentBlob(ctx, srv.Client(), dstStore, base, contentKey, contentCID, did)
		require.ErrorContains(t, err, "CID mismatch")
		_, err = dstStore.Open(ctx, contentKey)
		require.ErrorIs(t, err, blob.ErrNotFound, "a rejected blob must not be stored")
	})
}

func readTestBlob(t *testing.T, ctx context.Context, store blob.Store, key string) []byte {
	t.Helper()
	r, err := store.Open(ctx, key)
	require.NoError(t, err)
	defer r.Close()
	b, err := io.ReadAll(io.NewSectionReader(r, 0, r.Size()))
	require.NoError(t, err)
	return b
}
