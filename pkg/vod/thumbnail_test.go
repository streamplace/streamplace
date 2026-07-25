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

// TestGenerateThumbnail runs the VOD pipeline end-to-end against a real
// file-backed store, then exercises generateThumbnail: it should pull
// the midpoint video segment + its init segment out of the store,
// flatten the signed m4s, and render a valid JPEG.
func TestGenerateThumbnail(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestGenerateThumbnail")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)

	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	dst := teeWriter{hasher, out}

	mb := newMetafileBuilder(ctx, store)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, mb, signer.SignerInput)
	require.NoError(t, err)

	cid := hasher.CID()
	meta := mb.Finalize(cid, int64(out.Len()))

	// generateThumbnail reads the content blob from the store. The real
	// ProcessVOD writes it via the staging writer + content assembly; this
	// test drives streamThroughMuxl directly, so place it by hand.
	w, err := store.NewWriter(ctx, BlobsPrefix+cid+".mp4", "video/mp4")
	require.NoError(t, err)
	_, err = w.Write(out.Bytes())
	require.NoError(t, err)
	require.NoError(t, w.Complete())

	thumb, err := generateThumbnail(ctx, store, cid, meta)
	require.NoError(t, err)
	require.NotEmpty(t, thumb)
	require.True(t, bytes.HasPrefix(thumb, []byte{0xFF, 0xD8, 0xFF}), "expected JPEG SOI marker")
}
