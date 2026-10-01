package vod

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// firstTFDT walks the ISO-BMFF boxes of a segment chunk (per-track moof+mdat,
// possibly prefixed by c2pa/muxl uuid boxes) and returns the
// baseMediaDecodeTime from the first tfdt box it finds. Minimal walker:
// recurses into moof/traf containers and skips everything else. ok=false if no
// tfdt is present or a box uses 64-bit/extends-to-EOF sizing (not expected in
// canonical MUXL segments).
func firstTFDT(box []byte) (uint64, bool) {
	for len(box) >= 8 {
		size := int(binary.BigEndian.Uint32(box[0:4]))
		typ := string(box[4:8])
		if size < 8 || size > len(box) {
			return 0, false
		}
		payload := box[8:size]
		switch typ {
		case "moof", "traf":
			if v, ok := firstTFDT(payload); ok {
				return v, true
			}
		case "tfdt":
			if len(payload) >= 1 {
				switch payload[0] { // version
				case 0:
					if len(payload) >= 8 {
						return uint64(binary.BigEndian.Uint32(payload[4:8])), true
					}
				case 1:
					if len(payload) >= 12 {
						return binary.BigEndian.Uint64(payload[4:12]), true
					}
				}
			}
		}
		box = box[size:]
	}
	return 0, false
}

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
	// FlatHeaderSize is the byte length of the synthesized flat-MP4 header that
	// the canonical fragments are stored behind in the blob ([flat-header][
	// fragments]). Segment Offsets are fragment-relative (from 0), so HLS adds
	// this to reach the right byte range in the blob; the flat-MP4 path serves
	// the whole blob directly. Zero for legacy [init][segments] blobs whose
	// offsets are already absolute.
	FlatHeaderSize int64 `json:"flatHeaderSize,omitempty"`
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
	// Text-track encoding: language is BCP 47, label is captions.Source.
	Language string `json:"language,omitempty"`
	Label    string `json:"label,omitempty"`
}

// MetafileSegment is one GOP-sized byte range within the blob.
type MetafileSegment struct {
	Offset        int64  `json:"offset"`
	Size          int64  `json:"size"`
	DurationTicks uint64 `json:"durationTicks"`
	SampleCount   uint32 `json:"sampleCount"`
	// DecodeTimeKnown distinguishes a genuine zero tfdt from legacy indexes
	// that lack decode times. Caption placement uses the reference AV clock.
	FirstDecodeTicks uint64 `json:"firstDecodeTicks,omitempty"`
	DecodeTimeKnown  bool   `json:"decodeTimeKnown,omitempty"`
	// Text identity and reference clock belong to this GoP, not to the final
	// merged catalog or the physical ordering of neighboring byte ranges.
	CaptionConfigKnown    bool   `json:"captionConfigKnown,omitempty"`
	CaptionLanguage       string `json:"captionLanguage,omitempty"`
	CaptionLabel          string `json:"captionLabel,omitempty"`
	CaptionReferenceTicks uint64 `json:"captionReferenceTicks,omitempty"`
	CaptionReferenceScale uint32 `json:"captionReferenceScale,omitempty"`
	CaptionOffsetNanos    int64  `json:"captionOffsetNanos,omitempty"`
	// Discontinuity marks a segment that begins a new continuous timeline —
	// its decode time jumped backward relative to the previous segment of the
	// same track. This happens when a recording concatenates multiple ingest
	// sessions (the streamer disconnected/reconnected or stopped and restarted),
	// each restarting its tfdt near zero. The HLS playlist generator emits an
	// EXT-X-DISCONTINUITY before such a segment so players re-anchor the
	// timeline instead of choking on the backward jump. Omitted (false) for the
	// common single-session case.
	Discontinuity bool `json:"discontinuity,omitempty"`
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

	// leadingInitInBlob is true when the blob being measured physically begins
	// with the init segment ([init][segments], the legacy/transfer shape), so
	// segment offsets start after it. False for the flat-MP4 shape, where the
	// stored blob is [flat-header][bare fragments]: segment offsets are
	// fragment-relative (from 0) and HLS adds the flat-header size at serve time.
	leadingInitInBlob bool

	// lastTFDT / tfdtSeen track each track's previous baseMediaDecodeTime so a
	// backward jump (a concatenated reconnect/restart) can be flagged as a
	// discontinuity. See MetafileSegment.Discontinuity.
	lastTFDT        map[string]uint64
	tfdtSeen        map[string]bool
	textConfigs     map[string]textProbeJSON
	referenceScale  uint32
	referenceTicks  uint64
	referenceOffset time.Duration
	// Set only when indexing untouched archived fragments: compare their
	// original content hash with the ascending-numeric event serialization.
	canonicalHash *bdasl.Writer
}

func newMetafileBuilder(ctx context.Context, store blob.Store) *metafileBuilder {
	return &metafileBuilder{
		ctx:               ctx,
		store:             store,
		trackInitCIDs:     map[string]string{},
		trackSegments:     map[string][]MetafileSegment{},
		leadingInitInBlob: true,
		lastTFDT:          map[string]uint64{},
		tfdtSeen:          map[string]bool{},
		textConfigs:       map[string]textProbeJSON{},
	}
}

// newFragmentMetafileBuilder builds a metafile whose segment offsets are
// relative to the bare canonical fragments (from 0), for the flat-MP4 blob
// shape [flat-header][fragments] where the fragments don't start at byte 0.
func newFragmentMetafileBuilder(ctx context.Context, store blob.Store) *metafileBuilder {
	b := newMetafileBuilder(ctx, store)
	b.leadingInitInBlob = false
	return b
}

// Observe processes one MuxlEvent. Order matters — events must arrive
// in the same order they're written to the output stream, since that's
// how byte offsets get computed.
func (b *metafileBuilder) Observe(ev *muxl.MuxlEvent) error {
	switch ev.Type {
	case "init":
		if ev.Catalog != nil && ev.Catalog.Text != nil {
			for _, c := range ev.Catalog.Text.Renditions {
				id := strconv.FormatUint(uint64(c.TrackID()), 10)
				b.textConfigs[id] = textProbeJSON{TrackID: id, Language: c.Language, Label: c.Label}
			}
		}
		b.catalog = mergeCatalog(b.catalog, ev.Catalog)
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
		// Advance past the leading init only when it physically prefixes the
		// blob. For the flat-MP4 shape the fragments start at 0 (the flat-header
		// is added at serve/store time, not measured here).
		if !b.seenInit {
			if b.leadingInitInBlob {
				b.runningOffset = int64(len(ev.Data))
			}
			b.seenInit = true
		}
	case "segment", "signed-segment":
		refID, scale := catalogCaptionReference(b.catalog, ev.Tracks)
		refTicks, referenceKnown := firstTFDT(ev.Tracks[refID])
		if b.referenceScale != scale {
			if b.referenceScale != 0 {
				b.referenceOffset += captionTicks(b.referenceTicks, b.referenceScale)
			}
			b.referenceScale = scale
			b.referenceTicks = 0
		}
		var captionOffset time.Duration
		if scale != 0 {
			captionOffset = b.referenceOffset + captionTicks(b.referenceTicks, scale)
		}
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
		sort.Slice(keys, func(i, j int) bool {
			a, _ := strconv.ParseUint(keys[i], 10, 32)
			c, _ := strconv.ParseUint(keys[j], 10, 32)
			return a < c
		})
		for _, tid := range keys {
			chunk := ev.Tracks[tid]
			if b.canonicalHash != nil {
				if _, err := b.canonicalHash.Write(chunk); err != nil {
					return fmt.Errorf("hash canonical MUXL layout: %w", err)
				}
			}
			// Flag a discontinuity when this track's decode time jumps backward
			// vs its previous segment — the signature of a concatenated
			// reconnect/restart. A normal stream's tfdt is strictly increasing
			// (tfdt[n] = tfdt[n-1] + duration[n-1]), so this never fires for a
			// clean single-session recording.
			disc := false
			tfdt, known := firstTFDT(chunk)
			if known {
				if b.tfdtSeen[tid] && tfdt < b.lastTFDT[tid] {
					disc = true
				}
				b.lastTFDT[tid] = tfdt
				b.tfdtSeen[tid] = true
			}
			entry := MetafileSegment{
				Offset:           b.runningOffset,
				Size:             int64(len(chunk)),
				DurationTicks:    ev.Durations[tid],
				SampleCount:      ev.SampleCounts[tid],
				Discontinuity:    disc,
				FirstDecodeTicks: tfdt,
				DecodeTimeKnown:  known,
			}
			if config, ok := b.textConfigs[tid]; ok {
				entry.CaptionConfigKnown = true
				entry.CaptionLanguage = config.Language
				entry.CaptionLabel = config.Label
				if referenceKnown {
					entry.CaptionReferenceTicks = refTicks
					entry.CaptionReferenceScale = scale
					entry.CaptionOffsetNanos = int64(captionOffset)
				}
			}
			b.trackSegments[tid] = append(b.trackSegments[tid], entry)
			b.runningOffset += int64(len(chunk))
		}
		b.referenceTicks += ev.Durations[refID]
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
				if track.Type == "unknown" && b.catalog.Text != nil {
					for _, c := range b.catalog.Text.Renditions {
						if c.TrackID() == targetTID {
							track.Type = "text"
							track.Codec = c.Codec
							track.Timescale = c.Timescale()
							track.Language = c.Language
							track.Label = c.Label
							break
						}
					}
				}
			}
		}
		if track.Type == "text" {
			first := true
			for _, seg := range segments {
				if !seg.CaptionConfigKnown {
					continue
				}
				if first {
					track.Language = seg.CaptionLanguage
					track.Label = seg.CaptionLabel
					first = false
					continue
				}
				if track.Language != seg.CaptionLanguage {
					track.Language = "und"
				}
				if track.Label != seg.CaptionLabel {
					track.Label = ""
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
