package vod

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/test/remote"
)

// TestGenerateThumbnailFlatBlob drives the production thumbnail step against a
// real finalized VOD content blob — the [flat-header][fragments] shape, where
// the fragments do not start at byte 0.
//
// That gap is what broke thumbnailing: segment offsets are fragment-relative,
// so reading them as absolute blob offsets lands a whole flat header early and
// the decoder reports only "This file is invalid and cannot be played."
// generateThumbnail now hands the offset to muxl instead of adding the header
// size itself, so this passes without the metafile carrying FlatHeaderSize at
// all — which is exactly the coupling that regressed.
//
// Point SP_THUMBNAIL_BLOB at a blobs/<cid>.mp4-shaped file to run it.
func TestGenerateThumbnailFlatBlob(t *testing.T) {
	path := remote.RemoteFixture("db1aef16dc0c302e25ecef5a90d1338d9207078948b3379c9e22ec5a8239dc8f/thumbnail-test.mp4")
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestGenerateThumbnailFlatBlob")

	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	info, err := os.Stat(abs)
	require.NoError(t, err)

	// The blob's filename is its CID; the store is a temp dir with the blob
	// symlinked in so we don't copy gigabytes around.
	cid := filepath.Base(abs)
	for _, ext := range []string{".m4s", ".mp4"} {
		cid = strings.TrimSuffix(cid, ext)
	}

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "blobs"), 0755))
	require.NoError(t, os.Symlink(abs, filepath.Join(root, BlobsPrefix+cid+".mp4")))

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)

	// Rebuild the sidecars (metafile + per-track inits) from the blob itself.
	// newFragmentMetafileBuilder is what the paths that produce a flat blob
	// use (ProcessVOD, finalizeLivestream), so the offsets here are
	// fragment-relative exactly as a production metafile's are.
	//
	// Deliberately NOT regenerateSidecars: that uses newMetafileBuilder, whose
	// legacy [init][segments] assumption shifts every offset by the init
	// length — a separate transfer-path bug (see transfer.go's follow-up note).
	meta, err := buildFragmentMetafile(ctx, t, store, cid, info.Size())
	require.NoError(t, err)
	var video *MetafileTrack
	for tid := range meta.Tracks {
		tr := meta.Tracks[tid]
		if tr.Type == "video" {
			video = &tr
			break
		}
	}
	require.NotNil(t, video, "blob must have a video track")
	t.Logf("%d tracks, %d video segments, flatHeaderSize=%d",
		len(meta.Tracks), len(video.Segments), meta.FlatHeaderSize)

	thumb, err := generateThumbnail(ctx, store, cid, meta)
	require.NoError(t, err)
	require.NotEmpty(t, thumb)
	require.True(t, bytes.HasPrefix(thumb, []byte{0xFF, 0xD8, 0xFF}), "expected JPEG SOI marker")
	t.Logf("thumbnail: %d bytes", len(thumb))
}

// buildFragmentMetafile derives a fragment-relative metafile (and the
// per-track init blobs) from a stored content blob, mirroring what the
// flat-blob producers do.
func buildFragmentMetafile(ctx context.Context, t *testing.T, store blob.Store, cid string, size int64) (*Metafile, error) {
	t.Helper()
	r, err := store.Open(ctx, BlobsPrefix+cid+".mp4")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	mb := newFragmentMetafileBuilder(ctx, store)
	eventCh := make(chan *muxl.MuxlEvent, 16)
	producerErr := make(chan error, 1)
	go func() {
		producerErr <- muxl.RunMuxlUnwrapEvents(ctx, io.NewSectionReader(r, 0, size), eventCh)
		close(eventCh)
	}()
	for ev := range eventCh {
		if e := mb.Observe(ev); e != nil {
			return nil, e
		}
	}
	if err := <-producerErr; err != nil {
		return nil, err
	}
	meta := mb.Finalize(cid, size)

	// ProcessVOD/finalizeLivestream set this from the header they synthesized;
	// here the blob is already assembled, so recover it as the bytes the
	// fragments don't account for ([flat-header][fragments]).
	var fragments int64
	for _, tr := range meta.Tracks {
		for _, s := range tr.Segments {
			fragments += s.Size
		}
	}
	meta.FlatHeaderSize = size - fragments
	return meta, nil
}
