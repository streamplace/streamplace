package vod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// transferMimeType is the MIME we record on the media.origin for a
// transferred blob. Every VOD content + init blob is a fragmented MP4.
const transferMimeType = "video/mp4"

// TransferResult summarizes a completed VOD transfer for the caller (the
// internal admin endpoint).
type TransferResult struct {
	// ContentCID is the BDASL CID of the primary blob we now host.
	ContentCID string `json:"contentCid"`
	// Size is the primary blob's size in bytes.
	Size int64 `json:"size"`
	// DID is the account whose track the blob was fetched for (used for
	// the source's egress attribution + labeler check).
	DID string `json:"did"`
	// InitCIDs are the per-track init segment CIDs we regenerated and
	// wrote locally, deduplicated and sorted.
	InitCIDs []string `json:"initCids"`
	// TrackCount is the number of tracks in the regenerated metafile.
	TrackCount int `json:"trackCount"`
	// Downloaded is false when the content blob was already present
	// locally and the network fetch was skipped.
	Downloaded bool `json:"downloaded"`
}

// TransferVOD pulls a content-addressed VOD blob from a remote Streamplace
// node into this node's playback store, regenerates the playback sidecars
// (metafile + per-track init segments) locally from the blob, and publishes
// a place.stream.media.origin attestation that this node now hosts it.
//
// Only the primary content blob crosses the network. The metafile and the
// per-track init segments are deterministically derivable from it (via
// `muxl unwrap`), so we rebuild them locally rather than fetch them — the
// wire protocol is a single content-addressed GET against the source's
// existing place.stream.playback.getVideoBlob endpoint, and no
// non-content-addressed surface is required on the source.
//
// The fetched bytes are verified against contentCID before they're
// published; a source that serves the wrong bytes fails the transfer
// rather than poisoning the store. The published origin is byte-for-byte
// what the normal processing pipeline emits (rkey = CID), so it's
// idempotent across retries and indistinguishable from a locally-produced
// VOD once indexed.
//
// sourceBaseURL is the origin node's base URL (e.g. "https://source.example").
// contentCID is the primary blob's BDASL CID. did is an account that owns a
// track in the blob — the source's getVideoBlob requires it for egress
// attribution and labeler enforcement on content blobs.
func TransferVOD(ctx context.Context, cli *config.CLI, store blob.Store, httpClient *http.Client, sourceBaseURL, contentCID, did string) (*TransferResult, error) {
	ctx = log.WithLogValues(ctx, "func", "TransferVOD", "cid", contentCID, "did", did)
	ctx, span := vodTracer.Start(ctx, "vod.TransferVOD", trace.WithAttributes(
		attribute.String("cid", contentCID),
		attribute.String("did", did),
	))
	defer span.End()

	if _, err := bdasl.Parse(contentCID); err != nil {
		return nil, fmt.Errorf("invalid content CID %q: %w", contentCID, err)
	}
	if did == "" {
		return nil, fmt.Errorf("did is required")
	}
	base, err := url.Parse(sourceBaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid source base URL %q", sourceBaseURL)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	key := BlobsPrefix + contentCID + ".mp4"

	// Skip the (potentially multi-gigabyte) download if we already hold
	// the blob. Sidecars are regenerated unconditionally below so a prior
	// transfer that died after the download still converges.
	downloaded := false
	size, err := existingBlobSize(ctx, store, key)
	switch {
	case errors.Is(err, blob.ErrNotFound):
		size, err = downloadContentBlob(ctx, httpClient, store, base, key, contentCID, did)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "download")
			return nil, err
		}
		downloaded = true
	case err != nil:
		return nil, fmt.Errorf("stat existing blob: %w", err)
	default:
		log.Log(ctx, "vod transfer: content blob already present, skipping download", "cid", contentCID, "size", size)
	}

	meta, initCIDs, err := ensureSidecars(ctx, store, contentCID, size)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "regenerate")
		return nil, fmt.Errorf("regenerate sidecars: %w", err)
	}

	if err := publishOrigin(ctx, cli, contentCID, size, transferMimeType); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "origin")
		return nil, fmt.Errorf("publish origin: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("size", size),
		attribute.Int("track_count", len(meta.Tracks)),
		attribute.Bool("downloaded", downloaded),
	)
	span.SetStatus(codes.Ok, "")
	log.Log(ctx, "vod transfer complete",
		"cid", contentCID,
		"size", size,
		"tracks", len(meta.Tracks),
		"initCids", len(initCIDs),
		"downloaded", downloaded,
	)
	return &TransferResult{
		ContentCID: contentCID,
		Size:       size,
		DID:        did,
		InitCIDs:   initCIDs,
		TrackCount: len(meta.Tracks),
		Downloaded: downloaded,
	}, nil
}

// verifyResult is the outcome of the concurrent unwrap in downloadContentBlob:
// the BDASL CID of the canonical fragment stream it re-derived, plus any
// unwrap error.
type verifyResult struct {
	cid string
	err error
}

// hashEventChunks feeds one unwrap event's per-track bytes to h in sorted
// track-ID order — the byte layout of the canonical fragment stream. Init
// events carry no fragment bytes. Returns the number of bytes hashed.
func hashEventChunks(h *bdasl.Writer, ev *muxl.MuxlEvent) int64 {
	if ev.Type != "segment" && ev.Type != "signed-segment" {
		return 0
	}
	keys := make([]string, 0, len(ev.Tracks))
	for k := range ev.Tracks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var n int64
	for _, k := range keys {
		_, _ = h.Write(ev.Tracks[k])
		n += int64(len(ev.Tracks[k]))
	}
	return n
}

// existingBlobSize returns the size of the blob at key, or blob.ErrNotFound
// (unwrapped via errors.Is by the caller) if it isn't present.
func existingBlobSize(ctx context.Context, store blob.Store, key string) (int64, error) {
	r, err := store.Open(ctx, key)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	return r.Size(), nil
}

// downloadContentBlob streams the blob from the source node's getVideoBlob
// endpoint into the store, hashing as it goes. The store write is only
// Completed (made visible) once the canonical fragment stream or, for legacy
// blobs, the whole file hashes to contentCID, so a source that serves the
// wrong or corrupt bytes leaves nothing behind.
//
// A content blob is [flat-header][canonical fragments], but its CID is the
// BDASL of the fragments alone (stable across header re-synthesis), so a
// whole-file hash is not enough. muxl unwrap re-derives the fragment stream
// from the download as it streams; that hash is the primary check, with the
// whole-file hash retained for legacy [init][segments] blobs.
func downloadContentBlob(ctx context.Context, httpClient *http.Client, store blob.Store, base *url.URL, key, contentCID, did string) (int64, error) {
	endpoint := strings.TrimRight(base.String(), "/") + "/xrpc/place.stream.playback.getVideoBlob"
	u, err := url.Parse(endpoint)
	if err != nil {
		return 0, fmt.Errorf("build getVideoBlob URL: %w", err)
	}
	u.RawQuery = url.Values{"cid": {contentCID}, "did": {did}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetch blob from %s: %w", base.Redacted(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return 0, fmt.Errorf("source returned %s for cid %s: %s", resp.Status, contentCID, strings.TrimSpace(string(body)))
	}

	w, err := store.NewWriter(ctx, key, transferMimeType)
	if err != nil {
		return 0, fmt.Errorf("open store writer: %w", err)
	}
	// Close aborts the staged bytes unless Complete ran first, so a CID
	// mismatch or copy error discards the partial blob.
	defer w.Close()

	whole := bdasl.NewWriter()
	pr, pw := io.Pipe()
	verified := make(chan verifyResult, 1)
	go func() {
		events := make(chan *muxl.MuxlEvent, 16)
		fragCID := make(chan string, 1)
		go func() {
			h := bdasl.NewWriter()
			for ev := range events {
				hashEventChunks(h, ev)
			}
			fragCID <- h.CID()
		}()
		err := muxl.RunMuxlUnwrapEvents(ctx, pr, events)
		close(events)
		// Unblocks a copy still feeding the pipe if unwrap stopped early
		// (including the nil return it uses for context cancellation).
		_ = pr.CloseWithError(err)
		verified <- verifyResult{cid: <-fragCID, err: err}
	}()

	n, copyErr := io.Copy(io.MultiWriter(w, whole, pw), resp.Body)
	_ = pw.CloseWithError(copyErr)
	res := <-verified
	if copyErr != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("stream blob to store: %w", copyErr)
	}
	if res.err != nil {
		return 0, fmt.Errorf("verify blob: %w", res.err)
	}
	wholeCID := whole.CID()
	if res.cid != contentCID && wholeCID != contentCID {
		return 0, fmt.Errorf("CID mismatch: source served a blob with canonical-fragment CID %s and whole-file CID %s, expected %s (%d bytes)", res.cid, wholeCID, contentCID, n)
	}
	if err := w.Complete(); err != nil {
		return 0, fmt.Errorf("complete store write: %w", err)
	}
	log.Log(ctx, "vod transfer: downloaded content blob", "cid", contentCID, "bytes", n, "source", base.Redacted())
	return n, nil
}

// ensureSidecars returns the playback sidecars for an already-stored
// content blob, regenerating them only if they're not already present. The
// sidecars are a deterministic function of the (CID-verified) content blob,
// so an existing metafile is necessarily correct — and since the builder
// writes the per-track init blobs before the metafile, a present metafile
// implies the inits are present too. This fast path matters because
// re-deriving is a full `muxl unwrap` pass over a blob that can be many
// gigabytes; an idempotent re-run shouldn't pay for it twice.
func ensureSidecars(ctx context.Context, store blob.Store, contentCID string, size int64) (*Metafile, []string, error) {
	meta, err := readMetafile(ctx, store, contentCID)
	if err == nil {
		log.Log(ctx, "vod transfer: sidecars already present, skipping regeneration", "cid", contentCID)
		return meta, dedupedInitCIDs(meta), nil
	}
	if !errors.Is(err, blob.ErrNotFound) {
		return nil, nil, fmt.Errorf("check existing metafile: %w", err)
	}
	return regenerateSidecars(ctx, store, contentCID, size)
}

// regenerateSidecars rebuilds the playback sidecars for an already-stored
// content blob: the per-track init segments and the metafile JSON. It feeds
// the stored blob through `muxl unwrap --events` (which re-derives the
// per-track event stream byte-for-byte) into the same metafileBuilder the
// processing pipeline uses, so the regenerated sidecars are identical to
// what the originating node produced.
//
// The same pass identifies the blob's shape by which hash matches contentCID:
// a flat [flat-header][fragments] blob hashes only the fragments, while a
// legacy [init][segments] blob hashes as a whole. Fragment-relative offsets
// and FlatHeaderSize are set for the former; the init length is added back to
// offsets for the latter.
func regenerateSidecars(ctx context.Context, store blob.Store, contentCID string, size int64) (*Metafile, []string, error) {
	r, err := store.Open(ctx, BlobsPrefix+contentCID+".mp4")
	if err != nil {
		return nil, nil, fmt.Errorf("open content blob: %w", err)
	}
	defer r.Close()

	mb := newFragmentMetafileBuilder(ctx, store)
	whole := bdasl.NewWriter()
	fragments := bdasl.NewWriter()
	var fragSize, initLen int64

	// RunMuxlUnwrapEvents writes events synchronously and does not close
	// the channel (the caller owns it), so the producer runs in its own
	// goroutine while we drain on this one.
	eventCh := make(chan *muxl.MuxlEvent, 16)
	producerErr := make(chan error, 1)
	go func() {
		producerErr <- muxl.RunMuxlUnwrapEvents(ctx, io.TeeReader(io.NewSectionReader(r, 0, size), whole), eventCh)
		close(eventCh)
	}()

	var obsErr error
	for ev := range eventCh {
		if ev.Type == "init" && initLen == 0 {
			initLen = int64(len(ev.Data))
		}
		fragSize += hashEventChunks(fragments, ev)
		if e := mb.Observe(ev); e != nil && obsErr == nil {
			obsErr = e
		}
	}
	if perr := <-producerErr; perr != nil {
		return nil, nil, fmt.Errorf("muxl unwrap: %w", perr)
	}
	if obsErr != nil {
		return nil, nil, fmt.Errorf("metafile build: %w", obsErr)
	}
	// `muxl unwrap` returns nil (not an error) when the context is
	// cancelled mid-stream, which would otherwise let a truncated event
	// stream produce a partial metafile. Refuse to publish that.
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	fragCID, wholeCID := fragments.CID(), whole.CID()
	metaSize := fragSize
	flat := true
	switch {
	case fragCID == contentCID:
	case wholeCID == contentCID:
		flat = false
		metaSize = size
	default:
		return nil, nil, fmt.Errorf("stored blob does not match %s (canonical fragments %s, whole file %s)", contentCID, fragCID, wholeCID)
	}

	meta := mb.Finalize(contentCID, metaSize)
	if flat {
		if fragSize > size {
			return nil, nil, fmt.Errorf("canonical fragments (%d bytes) exceed blob size (%d)", fragSize, size)
		}
		meta.FlatHeaderSize = size - fragSize
	} else {
		if initLen == 0 {
			return nil, nil, fmt.Errorf("legacy blob %s has no leading init segment", contentCID)
		}
		for tid, tr := range meta.Tracks {
			for i := range tr.Segments {
				tr.Segments[i].Offset += initLen
			}
			meta.Tracks[tid] = tr
		}
	}
	if err := writeMetafile(ctx, store, contentCID, meta); err != nil {
		return nil, nil, err
	}

	return meta, dedupedInitCIDs(meta), nil
}

// dedupedInitCIDs returns the distinct per-track init CIDs from a metafile,
// sorted for a stable result. The builder already wrote each init blob to
// the store; this is just the summary for the transfer result.
func dedupedInitCIDs(meta *Metafile) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(meta.Tracks))
	for _, tr := range meta.Tracks {
		if tr.InitCID == "" {
			continue
		}
		if _, ok := seen[tr.InitCID]; ok {
			continue
		}
		seen[tr.InitCID] = struct{}{}
		out = append(out, tr.InitCID)
	}
	sort.Strings(out)
	return out
}
