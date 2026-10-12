package vod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/statedb"
)

// MaxClipDuration is the longest span one clip may cover.
const MaxClipDuration = time.Hour

var (
	// ErrClipNotFound: this node holds no segments of the stream in the range.
	ErrClipNotFound = errors.New("no segments in the requested range")
	// ErrClipUnavailable: part of the range is in a recording object still
	// being uploaded by another node, which alone holds those segments.
	ErrClipUnavailable = errors.New("part of the requested range is still being recorded on another node")
)

// clipPart is a run of canonical segments: bytes held in memory, or a byte
// range of a completed recording object.
type clipPart struct {
	startMs   int64
	data      []byte
	key       string
	off, size int64
}

// Clip is a stream between two instants, as the canonical segments starting
// in that span, drawn from wherever this node holds them. The segments are
// blindly concatenatable MUXL, so the clip's MP4 is a synthesized faststart
// header followed by their bytes verbatim, C2PA signatures included.
type Clip struct {
	store blob.Store
	parts []clipPart
}

// SegmentBuffer is a node's in-memory buffer of recent segments; a
// *media.MediaManager.
type SegmentBuffer interface {
	BufferedSegments(did string) []media.BufferedSegment
}

// FindClip collects the segments of did's stream that start in [from, to]:
// from completed recordings (skipped when state or store is nil) and from the
// node's in-memory buffer, which holds what isn't readable from a recording
// yet. Recordings only hold published segments; includeUnpublished also takes
// pre-live segments from memory, for moderation.
func FindClip(ctx context.Context, state *statedb.StatefulDB, store blob.Store, buf SegmentBuffer, did string, from, to time.Time, includeUnpublished bool) (*Clip, error) {
	fromMs, toMs := from.UnixMilli(), to.UnixMilli()
	c := &Clip{store: store}
	recorded := map[int64]bool{}
	if state != nil && store != nil {
		objs, err := state.ListS3SegmentsInRange(ctx, did, from, to)
		if err != nil {
			return nil, fmt.Errorf("list recordings: %w", err)
		}
		for _, obj := range objs {
			// An object's segments are in time order, so those in range are
			// one contiguous run of its bytes.
			var run *clipPart
			var off int64
			for _, seg := range obj.Segments {
				if seg.StartMs >= fromMs && seg.StartMs <= toMs {
					if run == nil {
						run = &clipPart{startMs: seg.StartMs, key: obj.Key, off: off}
					}
					run.size += seg.Size
					recorded[seg.StartMs] = true
				}
				off += seg.Size
			}
			if run != nil {
				c.parts = append(c.parts, *run)
			}
		}
	}

	buffered := buf.BufferedSegments(did)
	for _, seg := range buffered {
		ms := seg.Start.UnixMilli()
		if ms < fromMs || ms > toMs || recorded[ms] || (!seg.Published && !includeUnpublished) {
			continue
		}
		c.parts = append(c.parts, clipPart{startMs: ms, data: seg.Data, size: int64(len(seg.Data))})
	}
	sort.SliceStable(c.parts, func(i, j int) bool { return c.parts[i].startMs < c.parts[j].startMs })

	if state != nil {
		if err := checkOpenRecording(ctx, state, did, fromMs, toMs, buffered); err != nil {
			return nil, err
		}
	}
	if len(c.parts) == 0 {
		return nil, ErrClipNotFound
	}
	return c, nil
}

// checkOpenRecording fails with ErrClipUnavailable when the range reaches into
// a recording object still being uploaded whose segments this node doesn't
// hold: the node recording the stream holds them all in memory until the
// object completes (see MediaManager.RetainSegments), but another node may
// only hold the last few minutes. The buffer runs contiguously from its
// oldest segment to now, so it covers the range if it reaches back to either
// the object's first segment or the range's start.
func checkOpenRecording(ctx context.Context, state *statedb.StatefulDB, did string, fromMs, toMs int64, buffered []media.BufferedSegment) error {
	open, err := state.LatestOpenS3Segment(ctx, did, time.Now().Add(-s3.MaxOpenObjectAge))
	if err != nil {
		return fmt.Errorf("find open recording: %w", err)
	}
	if open == nil || open.StartedAt.UnixMilli() > toMs {
		return nil
	}
	if len(buffered) > 0 {
		oldest := buffered[0].Start.UnixMilli()
		if oldest <= open.StartedAt.UnixMilli() || oldest <= fromMs {
			return nil
		}
	}
	return ErrClipUnavailable
}

// Header synthesizes the clip's flat-MP4 faststart header. The MP4 is the
// header followed by WriteSegments: len(header) + BodySize() bytes.
func (c *Clip) Header(ctx context.Context) ([]byte, error) {
	r, closeParts, err := c.open(ctx)
	if err != nil {
		return nil, err
	}
	defer closeParts()
	return synthFlatHeader(ctx, r)
}

// BodySize is the byte length of the clip's segments.
func (c *Clip) BodySize() int64 {
	var n int64
	for _, p := range c.parts {
		n += p.size
	}
	return n
}

// WriteSegments writes the clip's segments, the body of its MP4.
func (c *Clip) WriteSegments(ctx context.Context, w io.Writer) error {
	r, closeParts, err := c.open(ctx)
	if err != nil {
		return err
	}
	defer closeParts()
	_, err = io.Copy(w, r)
	return err
}

// WriteMP4 writes the clip as a complete flat MP4.
func (c *Clip) WriteMP4(ctx context.Context, w io.Writer) error {
	header, err := c.Header(ctx)
	if err != nil {
		return err
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	return c.WriteSegments(ctx, w)
}

// open returns a reader over the clip's segment bytes, in order, and a func
// closing the recordings it opened.
func (c *Clip) open(ctx context.Context) (io.Reader, func(), error) {
	readers := make([]io.Reader, 0, len(c.parts))
	var closers []io.Closer
	for _, p := range c.parts {
		if p.key == "" {
			readers = append(readers, bytes.NewReader(p.data))
			continue
		}
		r, err := c.store.Open(ctx, p.key)
		if err != nil {
			closeAll(closers)
			return nil, nil, fmt.Errorf("open recording %s: %w", p.key, err)
		}
		closers = append(closers, r)
		readers = append(readers, io.NewSectionReader(r, p.off, p.size))
	}
	return io.MultiReader(readers...), func() { closeAll(closers) }, nil
}
