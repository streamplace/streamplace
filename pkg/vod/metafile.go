package vod

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// Metafile is the per-blob HLS playback index emitted alongside a
// processed VOD blob. JSON shape mirrors what `muxl hls` produces (see
// /home/iameli/code/muxl/src/hls.rs:write_metadata_json) — the playback
// worker and Streamplace's own playback handlers both consume it.
//
// blobCid identifies the primary content-addressed fMP4 in the
// blob.Store; tracks is keyed by stringified track ID. Each track's
// segments give absolute byte ranges within the primary blob, suitable
// for HLS EXT-X-BYTERANGE.
type Metafile struct {
	BlobCID  string                   `json:"blobCid"`
	BlobSize int64                    `json:"blobSize"`
	Tracks   map[string]MetafileTrack `json:"tracks"`
}

// MetafileTrack describes one track within a MUXL container.
type MetafileTrack struct {
	Type      string            `json:"type"`
	Codec     string            `json:"codec"`
	Timescale uint32            `json:"timescale"`
	InitCID   string            `json:"initCid"`
	BlobCID   string            `json:"blobCid"`
	BlobSize  int64             `json:"blobSize"`
	Segments  []MetafileSegment `json:"segments"`
	// Video-only — omitempty matches muxl's "present on video tracks,
	// absent on audio" output shape.
	Width  uint32 `json:"width,omitempty"`
	Height uint32 `json:"height,omitempty"`
	// Audio-only.
	Channels   uint32 `json:"channels,omitempty"`
	SampleRate uint32 `json:"sampleRate,omitempty"`
}

// MetafileSegment is one GOP-sized byte range within the blob.
type MetafileSegment struct {
	Offset        int64  `json:"offset"`
	Size          int64  `json:"size"`
	DurationTicks uint64 `json:"durationTicks"`
	SampleCount   uint32 `json:"sampleCount"`
}

// metafileBuilder consumes the rich event stream from the muxl
// concatenator and incrementally assembles a Metafile. It also writes
// each per-track init segment to the blob.Store eagerly, since they're
// content-addressed (`blobs/<initCid>.mp4`) and idempotent on retry.
//
// The builder doesn't know the primary blob's CID until the whole
// stream has been hashed by the upstream bdasl.Writer; the caller
// passes it in via Finalize.
type metafileBuilder struct {
	ctx   context.Context
	store blob.Store

	catalog       *muxl.MuxlCatalog
	trackInitCIDs map[string]string // trackID -> initCID
	trackSegments map[string][]MetafileSegment

	runningOffset int64 // bytes written to the concatenated output so far
	seenInit      bool
}

func newMetafileBuilder(ctx context.Context, store blob.Store) *metafileBuilder {
	return &metafileBuilder{
		ctx:           ctx,
		store:         store,
		trackInitCIDs: map[string]string{},
		trackSegments: map[string][]MetafileSegment{},
	}
}

// Observe processes one MuxlEvent. Order matters — events must arrive
// in the same order they're written to the output stream, since that's
// how byte offsets get computed.
func (b *metafileBuilder) Observe(ev *muxl.MuxlEvent) error {
	switch ev.Type {
	case "init":
		if b.seenInit {
			// Mid-stream init swap (catalog change). Doesn't happen in
			// today's single-input VOD pipeline; if it ever does we'd
			// need a richer schema (sub-archives per init). Warn loudly
			// rather than silently produce a wrong metafile.
			log.Warn(b.ctx, "metafile: mid-stream init swap; offsets after this point may be wrong")
		}
		b.seenInit = true
		b.catalog = ev.Catalog
		// Write per-track init bytes to the blob.Store keyed by their
		// own BDASL CID. The primary blob's init occupies bytes
		// [0, len(ev.Data)) in the output; advance runningOffset by
		// that amount so the first segment's offset is correct.
		for tid, initBytes := range ev.TrackInits {
			cid, err := writeInitBlob(b.ctx, b.store, initBytes)
			if err != nil {
				return fmt.Errorf("write init blob for track %s: %w", tid, err)
			}
			b.trackInitCIDs[tid] = cid
		}
		b.runningOffset = int64(len(ev.Data))
	case "segment", "signed-segment":
		// Within a single segment event, per-track byte slices are
		// concatenated in sorted key order (matching ParseMuxlEvents'
		// byte-channel dispatch). Track that order here so offsets
		// match the actual byte layout. For "signed-segment" events
		// each chunk carries a leading c2pa-uuid box; the offset math
		// is unchanged since it derives from the actual chunk length.
		keys := make([]string, 0, len(ev.Tracks))
		for k := range ev.Tracks {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, tid := range keys {
			chunk := ev.Tracks[tid]
			b.trackSegments[tid] = append(b.trackSegments[tid], MetafileSegment{
				Offset:        b.runningOffset,
				Size:          int64(len(chunk)),
				DurationTicks: ev.Durations[tid],
				SampleCount:   ev.SampleCounts[tid],
			})
			b.runningOffset += int64(len(chunk))
		}
	default:
		log.Warn(b.ctx, "metafile: unexpected event type; skipping", "type", ev.Type)
	}
	return nil
}

// Finalize assembles and returns the Metafile. cid is the primary
// blob's BDASL CID (from the upstream hasher); size is the total
// blob size in bytes.
func (b *metafileBuilder) Finalize(cid string, size int64) *Metafile {
	tracks := map[string]MetafileTrack{}
	for tid, segments := range b.trackSegments {
		track := MetafileTrack{
			Type:     "unknown",
			InitCID:  b.trackInitCIDs[tid],
			BlobCID:  cid,
			BlobSize: size,
			Segments: segments,
		}
		// Look up the per-track config in the catalog. Track keys are
		// stringified u32; the catalog stores them in CMAF Container
		// metadata.
		if b.catalog != nil {
			tidUint, err := strconv.ParseUint(tid, 10, 32)
			if err == nil {
				targetTID := uint32(tidUint)
				if b.catalog.Video != nil {
					for _, c := range b.catalog.Video.Renditions {
						if c.TrackID() == targetTID {
							track.Type = "video"
							track.Codec = c.Codec
							track.Timescale = c.Timescale()
							track.Width = c.CodedWidth
							track.Height = c.CodedHeight
							break
						}
					}
				}
				if track.Type == "unknown" && b.catalog.Audio != nil {
					for _, c := range b.catalog.Audio.Renditions {
						if c.TrackID() == targetTID {
							track.Type = "audio"
							track.Codec = c.Codec
							track.Timescale = c.Timescale()
							track.Channels = c.NumberOfChannels
							track.SampleRate = c.SampleRate
							break
						}
					}
				}
			}
		}
		tracks[tid] = track
	}
	return &Metafile{
		BlobCID:  cid,
		BlobSize: size,
		Tracks:   tracks,
	}
}

// writeMetafile JSON-encodes m and writes it to the store at
// blobs/<cid>.json. Wraps the upload in a span so the cost is visible.
func writeMetafile(ctx context.Context, store blob.Store, cid string, m *Metafile) error {
	ctx, span := vodTracer.Start(ctx, "vod.writeMetafile", trace.WithAttributes(
		attribute.String("cid", cid),
		attribute.Int("track_count", len(m.Tracks)),
	))
	defer span.End()

	body, err := json.Marshal(m)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("marshal metafile: %w", err)
	}
	key := BlobsPrefix + cid + ".json"
	w, err := store.NewWriter(ctx, key, "application/json")
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("open metafile writer: %w", err)
	}
	defer w.Close()
	if _, err := w.Write(body); err != nil {
		span.RecordError(err)
		return fmt.Errorf("write metafile: %w", err)
	}
	if err := w.Complete(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("complete metafile: %w", err)
	}
	span.SetAttributes(attribute.Int("metafile_bytes", len(body)))
	log.Log(ctx, "wrote VOD metafile",
		"key", key,
		"bytes", len(body),
		"tracks", len(m.Tracks),
	)
	return nil
}

// readMetafile loads and decodes the metafile written by writeMetafile
// (blobs/<cid>.json). Used by server-side thumbnail backfill, which needs
// the per-track segment table to pick a frame to render.
func readMetafile(ctx context.Context, store blob.Store, cid string) (*Metafile, error) {
	data, err := readWholeBlob(ctx, store, BlobsPrefix+cid+".json")
	if err != nil {
		return nil, fmt.Errorf("open metafile: %w", err)
	}
	var m Metafile
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode metafile: %w", err)
	}
	return &m, nil
}

// writeInitBlob writes per-track init bytes to the blob.Store keyed by
// their BDASL CID. Content-addressed and idempotent: if a previous run
// (or another VOD with the same init bytes) already wrote this blob,
// the Move is a no-op overwrite.
func writeInitBlob(ctx context.Context, store blob.Store, initBytes []byte) (string, error) {
	hasher := bdasl.NewWriter()
	if _, err := hasher.Write(initBytes); err != nil {
		return "", fmt.Errorf("hash init bytes: %w", err)
	}
	cid := hasher.CID()
	key := BlobsPrefix + cid + ".mp4"
	w, err := store.NewWriter(ctx, key, "video/mp4")
	if err != nil {
		return "", fmt.Errorf("open init writer: %w", err)
	}
	defer w.Close()
	if _, err := w.Write(initBytes); err != nil {
		return "", fmt.Errorf("write init bytes: %w", err)
	}
	if err := w.Complete(); err != nil {
		return "", fmt.Errorf("complete init upload: %w", err)
	}
	log.Debug(ctx, "wrote init blob", "cid", cid, "size", len(initBytes))
	return cid, nil
}
