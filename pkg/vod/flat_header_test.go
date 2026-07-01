package vod

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// TestFlatHeaderServing is the end-to-end proof of the flat-MP4 VOD path on the
// streamplace side: from a finalize-shaped canonical blob ([init][signed
// segments]), emit metafiles, synthesize a faststart header, and show that
// serving header ++ <a byte-range of the canonical blob> via blob.PrefixReader
// reproduces a real flat MP4 byte-for-byte. It also pins down where the flat
// body sits relative to the canonical blob (the bodyOffset the endpoint feeds
// PrefixReader).
//
// Needs gstreamer (warmGST) and the new muxl (Metafiles/SynthesizeFlatHeader),
// so it runs in the cgo container with the local muxl replace.
func TestFlatHeaderServing(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestFlatHeaderServing")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)
	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	// Build a canonical VOD blob exactly like finalize: [init][signed segments].
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	dst := teeWriter{hasher, out}
	mb := newMetafileBuilder(ctx, store)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, mb, signer.SignerInput)
	require.NoError(t, err)
	canon := out.Bytes()
	meta := mb.Finalize(hasher.CID(), int64(len(canon)))
	initLen := minFirstOffset(t, meta)

	// Emit metafiles, then synthesize the flat header from them.
	var metas bytes.Buffer
	require.NoError(t, muxl.RunMuxlMetafiles(ctx, bytes.NewReader(canon), &metas))
	require.NotZero(t, metas.Len(), "Metafiles produced no output")
	var hdr bytes.Buffer
	require.NoError(t, muxl.RunMuxlSynthesizeFlatHeader(ctx, bytes.NewReader(metas.Bytes()), &hdr))
	require.NotZero(t, hdr.Len(), "SynthesizeFlatHeader produced no output")

	// Oracle: the real flat MP4 muxl writes from the same canonical blob.
	var oracle bytes.Buffer
	require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(canon), "flat", &oracle))

	// The synthesized header is the flat MP4's prefix; the remainder is the body.
	require.True(t, bytes.HasPrefix(oracle.Bytes(), hdr.Bytes()),
		"synthesized header (%d) must be a prefix of the flat MP4 (%d)", hdr.Len(), oracle.Len())
	body := oracle.Bytes()[hdr.Len():]

	// The body must be a contiguous tail range of the canonical blob.
	idx := bytes.Index(canon, body)
	require.GreaterOrEqual(t, idx, 0, "flat body must be a contiguous range of the canonical blob")
	require.Equal(t, len(canon), idx+len(body), "flat body must run to the end of the canonical blob")
	t.Logf("flat body offset in canonical blob = %d (initLen=%d, blobSize=%d, headerLen=%d)",
		idx, initLen, len(canon), hdr.Len())

	// Serve header ++ canonical-blob[idx:] via PrefixReader over the STORED blob
	// (exactly what the playback endpoint will do) and prove it's byte-identical
	// to the real flat MP4.
	require.NoError(t, writeBlob(ctx, store, "canon.mp4", canon))
	r, err := store.Open(ctx, "canon.mp4")
	require.NoError(t, err)
	defer r.Close()
	pr := blob.NewPrefixReader(hdr.Bytes(), r, int64(idx), int64(len(body)))
	served, err := io.ReadAll(io.NewSectionReader(pr, 0, pr.Size()))
	require.NoError(t, err)
	require.Equal(t, oracle.Bytes(), served, "PrefixReader must serve the byte-exact flat MP4")
}

func writeBlob(ctx context.Context, store blob.Store, key string, b []byte) error {
	w, err := store.NewWriter(ctx, key, "video/mp4")
	if err != nil {
		return err
	}
	defer w.Close()
	if _, err := w.Write(b); err != nil {
		return err
	}
	return w.Complete()
}
