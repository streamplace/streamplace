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

// TestFinalizeFlatVODAssembly exercises the new finalize model end to end (minus
// statedb/publish): from the bare canonical fragments, it confirms the MUXL CID
// is the fragments' own BDASL hash, the HLS metafile is fragment-relative
// (offsets from 0), and the assembled [flat-header][fragments] blob is a real
// flat MP4 byte-for-byte. Needs gstreamer + the new muxl (local replace).
func TestFinalizeFlatVODAssembly(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestFinalizeFlatVODAssembly")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)
	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	// Produce [init][segments] then strip the init to get the bare fragments
	// (what the live recorder stores as .m4s objects).
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	out := &bytes.Buffer{}
	h0 := bdasl.NewWriter()
	dst := teeWriter{h0, out}
	mb0 := newMetafileBuilder(ctx, store)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, mb0, signer.SignerInput)
	require.NoError(t, err)
	canon := out.Bytes()
	meta0 := mb0.Finalize(h0.CID(), int64(len(canon)))
	initLen := minFirstOffset(t, meta0)
	fragments := canon[initLen:]

	require.NoError(t, writeBlob(ctx, store, "live/frag.m4s", fragments))
	keys := []string{"live/frag.m4s"}

	// --- the finalize steps ---
	flatHeader, err := synthFlatHeaderForObjects(ctx, store, keys)
	require.NoError(t, err)
	require.NotEmpty(t, flatHeader)

	muxlCID, fragSize, metafile, err := hashAndBuildFragmentMetafile(ctx, store, keys)
	require.NoError(t, err)
	require.Equal(t, int64(len(fragments)), fragSize, "fragments size")

	// MUXL CID is the BDASL of the bare fragments alone — stable across header
	// re-synthesis.
	h := bdasl.NewWriter()
	_, _ = h.Write(fragments)
	require.Equal(t, h.CID(), muxlCID, "MUXL CID must be the BDASL of the bare fragments")

	// HLS metafile is fragment-relative: the earliest segment starts at 0.
	require.Equal(t, int64(0), minFirstOffset(t, metafile), "segment offsets must be fragment-relative")
	metafile.FlatHeaderSize = int64(len(flatHeader))

	// Assemble [flat-header][fragments] at blobs/<muxlCID>.mp4.
	contentKey := BlobsPrefix + muxlCID + ".mp4"
	require.NoError(t, assembleContentBlob(ctx, store, flatHeader, keys, contentKey))

	// The assembled blob is a real flat MP4 (== muxl's own flat wrap), and the
	// flat header is its prefix.
	var oracle bytes.Buffer
	require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(fragments), "flat", &oracle))
	cr, err := store.Open(ctx, contentKey)
	require.NoError(t, err)
	defer cr.Close()
	assembled, err := io.ReadAll(io.NewSectionReader(cr, 0, cr.Size()))
	require.NoError(t, err)
	require.Equal(t, oracle.Bytes(), assembled, "assembled [flat-header][fragments] must be a byte-exact flat MP4")
	require.True(t, bytes.HasPrefix(assembled, flatHeader), "flat header must be the blob prefix")

	// HLS reads the same blob: header size + fragment-relative offset lands on
	// the segment bytes (== the canonical fragment chunk).
	for tid, tr := range metafile.Tracks {
		s := tr.Segments[0]
		blobOff := metafile.FlatHeaderSize + s.Offset
		got := make([]byte, s.Size)
		_, err := cr.ReadAt(got, blobOff)
		require.NoError(t, err)
		require.Equal(t, fragments[s.Offset:s.Offset+s.Size], got, "track %s seg0: HLS byte range must hit the fragment bytes", tid)
	}
}

// TestProcessVODFragmentBuilderPath proves the ProcessVOD path: streaming
// through muxl with a *fragment* metafile builder writes the bare canonical
// fragments to dst (no leading init) — so the same dst hashes to the MUXL CID
// and the synth-header + assemble produces a byte-exact flat MP4. This is the
// inline equivalent of the finalize path's object concatenation, done in a
// single muxl pass with no cross-run determinism assumption.
func TestProcessVODFragmentBuilderPath(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestProcessVODFragmentBuilderPath")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)
	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	// The fragment builder drives streamThroughMuxl to skip the init in dst
	// (writeInit=false), exactly as ProcessVOD now does.
	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	mb := newFragmentMetafileBuilder(ctx, store)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), teeWriter{hasher, out}, mb, signer.SignerInput)
	require.NoError(t, err)
	fragments := out.Bytes()
	require.NotEmpty(t, fragments)

	// dst is bare fragments: the first box is the segment's c2pa 'uuid', NOT
	// the init's 'ftyp' — proving the init was kept out of the blob.
	require.GreaterOrEqual(t, len(fragments), 8)
	require.NotEqual(t, "ftyp", string(fragments[4:8]), "fragments-only blob must not start with the init ftyp box")

	muxlCID := hasher.CID()
	require.NoError(t, writeBlob(ctx, store, "stage/frag.m4s", fragments))
	keys := []string{"stage/frag.m4s"}

	flatHeader, err := synthFlatHeaderForObjects(ctx, store, keys)
	require.NoError(t, err)

	metafile := mb.Finalize(muxlCID, int64(len(fragments)))
	require.Equal(t, int64(0), minFirstOffset(t, metafile), "ProcessVOD metafile must be fragment-relative")
	metafile.FlatHeaderSize = int64(len(flatHeader))

	contentKey := BlobsPrefix + muxlCID + ".mp4"
	require.NoError(t, assembleContentBlob(ctx, store, flatHeader, keys, contentKey))

	var oracle bytes.Buffer
	require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(fragments), "flat", &oracle))
	cr, err := store.Open(ctx, contentKey)
	require.NoError(t, err)
	defer cr.Close()
	assembled, err := io.ReadAll(io.NewSectionReader(cr, 0, cr.Size()))
	require.NoError(t, err)
	require.Equal(t, oracle.Bytes(), assembled, "ProcessVOD-style [flat-header][fragments] must be a byte-exact flat MP4")
}
