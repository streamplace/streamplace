package vod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/muxl"
)

// The flat-MP4 VOD model: the stored blob is [flat-header][canonical fragments],
// content-addressed by the MUXL CID — the BDASL hash of the fragments ALONE,
// not the assembled blob. Keying off the immutable fragments means a better
// flat-header can be re-synthesized and rewritten in place without changing a
// VOD's id (or any media.origin/media.track record), and one blob serves both
// flat-MP4 (the whole file) and HLS (per-track init blobs + byte-ranges into
// the fragments at flatHeaderSize+offset).

// openFragmentReaders opens the canonical fragment objects as sequential
// readers in order. The caller closes the returned closers.
func openFragmentReaders(ctx context.Context, store blob.Store, keys []string) ([]io.Reader, []io.Closer, error) {
	readers := make([]io.Reader, 0, len(keys))
	closers := make([]io.Closer, 0, len(keys))
	for _, key := range keys {
		r, err := store.Open(ctx, key)
		if err != nil {
			for _, c := range closers {
				_ = c.Close()
			}
			return nil, nil, fmt.Errorf("open object %s: %w", key, err)
		}
		closers = append(closers, r)
		readers = append(readers, io.NewSectionReader(r, 0, r.Size()))
	}
	return readers, closers, nil
}

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		_ = c.Close()
	}
}

// hashAndBuildFragmentMetafile streams the bare canonical fragments once,
// teeing into a bdasl hasher (→ the MUXL CID) and muxl unwrap → a
// fragment-relative metafileBuilder (→ the HLS metafile + per-track init
// blobs). The MUXL CID hashes the fragments alone, so it's stable across
// header re-synthesis. Returns the MUXL CID, the fragments' total size, and the
// metafile (with fragment-relative segment offsets).
func hashAndBuildFragmentMetafile(ctx context.Context, store blob.Store, keys []string) (string, int64, *Metafile, error) {
	readers, closers, err := openFragmentReaders(ctx, store, keys)
	if err != nil {
		return "", 0, nil, err
	}
	defer closeAll(closers)

	hasher := bdasl.NewWriter()
	counter := &countingWriter{}
	tee := io.TeeReader(io.MultiReader(readers...), io.MultiWriter(hasher, counter))

	mb := newFragmentMetafileBuilder(ctx, store)
	eventCh := make(chan *muxl.MuxlEvent, 16)
	producerErr := make(chan error, 1)
	go func() {
		producerErr <- muxl.RunMuxlUnwrapEvents(ctx, tee, eventCh)
		close(eventCh)
	}()

	var obsErr error
	for ev := range eventCh {
		if e := mb.Observe(ev); e != nil && obsErr == nil {
			obsErr = e
		}
	}
	if perr := <-producerErr; perr != nil {
		return "", 0, nil, fmt.Errorf("muxl unwrap: %w", perr)
	}
	if obsErr != nil {
		return "", 0, nil, fmt.Errorf("metafile build: %w", obsErr)
	}
	if err := ctx.Err(); err != nil {
		return "", 0, nil, err
	}

	muxlCID := hasher.CID()
	size := counter.load()
	return muxlCID, size, mb.Finalize(muxlCID, size), nil
}

// synthFlatHeaderForObjects emits the metafile stream for the canonical
// fragments and synthesizes the flat-MP4 faststart header from it. muxl owns
// all the co64 offset math over the [header][fragments] layout, so the returned
// header is exactly the bytes to prepend to the fragments.
func synthFlatHeaderForObjects(ctx context.Context, store blob.Store, keys []string) ([]byte, error) {
	readers, closers, err := openFragmentReaders(ctx, store, keys)
	if err != nil {
		return nil, err
	}
	defer closeAll(closers)

	var metas bytes.Buffer
	if err := muxl.RunMuxlMetafiles(ctx, io.MultiReader(readers...), &metas); err != nil {
		return nil, fmt.Errorf("emit metafiles: %w", err)
	}
	var header bytes.Buffer
	if err := muxl.RunMuxlSynthesizeFlatHeader(ctx, bytes.NewReader(metas.Bytes()), &header); err != nil {
		return nil, fmt.Errorf("synthesize flat header: %w", err)
	}
	if header.Len() == 0 {
		return nil, errors.New("muxl produced an empty flat header")
	}
	return header.Bytes(), nil
}
