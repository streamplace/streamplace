package vod

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
)

// thumbnailFormat / thumbnailMimeType are the encoding used for the
// auto-generated VOD thumbnail. Kept together so the uploadBlob
// content-type stays in sync with what media.Thumbnail emits.
const (
	thumbnailFormat   = "jpeg"
	thumbnailMimeType = "image/jpeg"
)

// generateThumbnail produces a JPEG thumbnail from roughly the middle of
// a processed VOD. It reads the video track's per-track init segment
// plus the midpoint segment's video chunk out of the blob store — a
// video-only fragmented MP4 — and hands them to
// media.ThumbnailFromSegment.
//
// Video-only is deliberate: the downstream decoder only needs a frame,
// and a single-track input keeps the gstreamer pipeline simple (one
// decodebin pad, no audio to drain).
//
// A missing thumbnail is not fatal to VOD processing: callers should log
// and continue if this returns an error.
func generateThumbnail(ctx context.Context, store blob.Store, cid string, meta *Metafile) ([]byte, error) {
	ctx, span := vodTracer.Start(ctx, "vod.generateThumbnail", trace.WithAttributes(
		attribute.String("cid", cid),
	))
	defer span.End()

	var video *MetafileTrack
	for tid := range meta.Tracks {
		tr := meta.Tracks[tid]
		if tr.Type == "video" {
			video = &tr
			break
		}
	}
	if video == nil || len(video.Segments) == 0 {
		return nil, fmt.Errorf("no video segments to thumbnail")
	}

	// "Approximately halfway" — the middle segment by index. VOD GoPs are
	// short and roughly uniform, so index-midpoint is close enough to
	// time-midpoint without summing durations.
	idx := len(video.Segments) / 2
	seg := video.Segments[idx]
	span.SetAttributes(
		attribute.Int("segment_index", idx),
		attribute.Int("segment_count", len(video.Segments)),
		attribute.Int64("segment_offset", seg.Offset),
		attribute.Int64("segment_size", seg.Size),
	)

	readStart := time.Now()
	initSeg, err := readWholeBlob(ctx, store, BlobsPrefix+video.InitCID+".mp4")
	if err != nil {
		return nil, fmt.Errorf("read init blob: %w", err)
	}

	content, err := store.Open(ctx, BlobsPrefix+cid+".mp4")
	if err != nil {
		return nil, fmt.Errorf("open content blob: %w", err)
	}
	defer content.Close()
	// Hand muxl the random-access handle and the segment's index coordinates
	// and let it produce the bytes, rather than slicing the blob here. Doing
	// it by hand meant adding Metafile.FlatHeaderSize to reach the fragments
	// past a flat blob's synthesized header, and omitting it read a header's
	// worth of the wrong bytes — which a decoder reports only as "this file is
	// invalid and cannot be played". muxl owns the layout it synthesized, so
	// it owns the arithmetic, derives the segment's extent from its own
	// boundaries, and errors loudly on an offset that misses one.
	//
	// Which coordinate space the offsets are in is the metafile's to declare:
	// FlatHeaderSize is set for the [flat-header][fragments] shape, whose
	// offsets are fragment-relative, and zero for legacy [init][segments]
	// blobs, whose offsets are already absolute.
	readSegments := muxl.RunMuxlReadSegments
	if meta.FlatHeaderSize == 0 {
		readSegments = muxl.RunMuxlReadSegmentsAtFileOffset
	}
	segBytes, err := readSegments(ctx, content, content.Size(), seg.Offset, 1)
	if err != nil {
		return nil, fmt.Errorf("read segment bytes: %w", err)
	}
	// Timing breadcrumb: blob read vs. render (flatten + decode, logged
	// inside ThumbnailFromSegment) so a slow thumbnail names its own step.
	log.Log(ctx, "thumbnail: read inputs",
		"ms", time.Since(readStart).Milliseconds(),
		"init_bytes", len(initSeg), "segment_bytes", len(segBytes),
		"segment_index", idx, "segment_count", len(video.Segments))

	var thumb bytes.Buffer
	if err := media.ThumbnailFromSegment(ctx, initSeg, segBytes, &thumb, thumbnailFormat); err != nil {
		return nil, fmt.Errorf("render thumbnail: %w", err)
	}
	span.SetAttributes(attribute.Int("thumbnail_bytes", thumb.Len()))
	return thumb.Bytes(), nil
}

// readWholeBlob reads an entire blob.Store object into memory. Used for
// the small per-track init segment, not for the (potentially large)
// content blob.
func readWholeBlob(ctx context.Context, store blob.Store, key string) ([]byte, error) {
	r, err := store.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.NewSectionReader(r, 0, r.Size()))
}
