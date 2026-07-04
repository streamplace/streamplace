package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// segmentValidation mirrors one entry of muxl-sign `verify`'s output: the
// manifest + cert + validation results for a single canonical segment (one
// track). Provenance is validated in-wasm now, so this replaces the old
// iroh-streamplace get_manifest_and_cert ManifestAndCert blob.
type segmentValidation struct {
	TrackID           uint32                      `json:"track_id"`
	Manifest          c2patypes.Manifest          `json:"manifest"`
	Cert              string                      `json:"cert"`
	ValidationResults c2patypes.ValidationResults `json:"validation_results"`
	ValidationState   string                      `json:"validation_state"`
}

func (mm *MediaManager) ValidateMP4(ctx context.Context, input io.Reader, local bool) error {
	tracer := otel.Tracer("signer")
	ctx, span := tracer.Start(ctx, "ValidateMP4")
	defer span.End()

	_, readSpan := tracer.Start(ctx, "ValidateMP4.ReadInput")
	buf, err := io.ReadAll(input)
	readSpan.SetAttributes(attribute.Int("bytes", len(buf)))
	readSpan.End()
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	vs, err := mm.validateSource(ctx, buf, local)
	if err != nil {
		return err
	}
	if vs == nil {
		return nil // already have this segment
	}

	// Complete the segment to carry both AAC and Opus. A single-codec segment is
	// fed to the stream's continuous transcoder, which distributes the completed
	// dual-codec segment asynchronously (~1 GoP later, gapless). Everything else
	// — already dual-codec, no audio, audio-only, an exotic codec, or no node
	// signer — is distributed as-is, immediately.
	if target, need := mm.audioCompletionTarget(ctx, buf); need {
		if cert, keyPEM, serr := mm.transcodeSigner(); serr == nil {
			return mm.feedStreamTranscoder(ctx, vs, buf, target, cert, keyPEM)
		} else {
			log.Warn(ctx, "node transcode signer unavailable, distributing single-codec", "error", serr)
		}
	}
	return mm.distributeSegment(ctx, vs, buf)
}

// validatedSegment carries the per-segment context derived from validating the
// SOURCE segment. It is threaded through to distributeSegment, which may run
// later (and over the COMPLETED dual-codec bytes) when codec completion is
// async — the metadata still comes from the source.
type validatedSegment struct {
	meta          *SegmentMetadata
	mediaData     *localdb.SegmentMediaData
	label         string
	repoDID       string
	signingKeyDID string
	local         bool
}

// validateSource verifies + media-parses a bare canonical .m4s, resolves the
// streamer identity, and runs the dedup / distribution-policy / content /
// allow-list checks. It returns the per-segment context, or (nil, nil) when the
// segment is already known (dedup skip).
func (mm *MediaManager) validateSource(ctx context.Context, buf []byte, local bool) (*validatedSegment, error) {
	tracer := otel.Tracer("signer")

	valid, err := ValidateMP4Media(ctx, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to validate MP4 media: %w", err)
	}
	meta := valid.Meta
	pub := valid.Pub

	if valid.Manifest.Label == nil {
		return nil, fmt.Errorf("segment manifest has no label")
	}
	label := *valid.Manifest.Label
	if mm.model != nil {
		_, dbSpan := tracer.Start(ctx, "ValidateMP4.LocalDB.GetSegment")
		oldSeg, err := mm.localDB.GetSegment(label)
		dbSpan.End()
		if err != nil {
			return nil, fmt.Errorf("failed to get old segment: %w", err)
		}
		if oldSeg != nil {
			log.Warn(ctx, "segment already exists, skipping", "segmentID", label)
			return nil, nil
		}
	}

	if meta.MetadataConfiguration != nil && meta.MetadataConfiguration.DistributionPolicy != nil {
		allowedBroadcasters := meta.MetadataConfiguration.DistributionPolicy.AllowedBroadcasters
		if allowedBroadcasters != nil {
			if !slices.Contains(allowedBroadcasters, "*") && !slices.Contains(allowedBroadcasters, fmt.Sprintf("did:web:%s", mm.cli.BroadcasterHost)) {
				return nil, fmt.Errorf("broadcaster %s is not allowed to distribute content. Allowed broadcasters: %v", fmt.Sprintf("did:web:%s", mm.cli.BroadcasterHost), allowedBroadcasters)
			}
		}
	}

	var repoDID, signingKeyDID string
	// special case for test signers that are only signed with a key
	if strings.HasPrefix(meta.Creator, constants.DID_KEY_PREFIX) {
		signingKeyDID = meta.Creator
		repoDID = meta.Creator
	} else {
		atCtx, atSpan := tracer.Start(ctx, "ValidateMP4.SyncBlueskyRepoCached")
		repo, err := mm.atsync.SyncBlueskyRepoCached(atCtx, meta.Creator)
		atSpan.End()
		if err != nil {
			return nil, err
		}
		modelCtx, modelSpan := tracer.Start(ctx, "ValidateMP4.GetSigningKey")
		signingKey, err := mm.model.GetSigningKey(modelCtx, pub.DIDKey(), repo.DID)
		modelSpan.End()
		if err != nil {
			return nil, err
		}
		if signingKey == nil {
			return nil, fmt.Errorf("no signing key found for %s", pub.DIDKey())
		}
		repoDID = repo.DID
		signingKeyDID = signingKey.DID
	}

	if err := mm.cli.StreamIsAllowed(repoDID); err != nil {
		return nil, fmt.Errorf("got valid segment, but user %s is not allowed: %w", repoDID, err)
	}

	// Defense in depth: a banned streamer's ingest worker is torn down
	// (watchKeyRevocation), but if that ever misses — a raced ban, a failed kill,
	// a worker that somehow lived — this is the chokepoint every ingest path
	// converges on, so refusing here keeps banned content from being distributed
	// regardless.
	_, labelSpan := tracer.Start(ctx, "ValidateMP4.streamerIsBanned")
	banned, err := mm.streamerIsBanned(repoDID)
	labelSpan.End()
	if err != nil {
		return nil, fmt.Errorf("check labels for %s: %w", repoDID, err)
	}
	if banned {
		return nil, fmt.Errorf("got valid segment, but user %s is banned", repoDID)
	}

	// Apply content filtering after metadata is parsed
	if mm.cli.ContentFilters != nil {
		if err := mm.applyContentFilters(ctx, meta); err != nil {
			return nil, err
		}
	}

	return &validatedSegment{
		meta:          meta,
		mediaData:     valid.MediaData,
		label:         label,
		repoDID:       repoDID,
		signingKeyDID: signingKeyDID,
		local:         local,
	}, nil
}

// streamerIsBanned reports whether repoDID currently carries an active ban
// label. It's the defense-in-depth gate validateSource applies so a banned
// streamer's segments are refused even if their ingest worker wasn't torn down.
// Returns false when there's no model (the minimal worker/test managers);
// enforcement runs in main, which has the model + label feed.
func (mm *MediaManager) streamerIsBanned(repoDID string) (bool, error) {
	if mm.model == nil {
		return false, nil
	}
	labels, err := mm.model.GetActiveLabels(repoDID)
	if err != nil {
		return false, err
	}
	return atproto.IsBanned(labels...), nil
}

// distributeSegment archives the segment, folds it into the streamer's live-HLS
// window, and notifies subscribers. seg is the bytes to store/distribute — the
// completed dual-codec segment when completion ran, else the validated source
// segment; vs carries the metadata derived from the source. May be invoked
// synchronously from ValidateMP4 or asynchronously from the stream transcoder,
// so it takes its own (non-request) context in the latter case.
func (mm *MediaManager) distributeSegment(ctx context.Context, vs *validatedSegment, seg []byte) error {
	tracer := otel.Tracer("signer")
	meta := vs.meta

	_, fileSpan := tracer.Start(ctx, "ValidateMP4.SegmentArchiveWrite", trace.WithAttributes(
		attribute.Int("bytes", len(seg)),
	))
	fd, err := mm.cli.SegmentFileCreate(vs.repoDID, meta.StartTime, "m4s")
	if err != nil {
		fileSpan.End()
		return err
	}
	defer fd.Close()
	if _, err := io.Copy(fd, bytes.NewReader(seg)); err != nil {
		fileSpan.End()
		return err
	}
	fileSpan.End()

	// Fold the validated segment into the streamer's live-HLS window, off the
	// critical path. This runs for every segment — locally signed or replicated
	// from another node — so any node that validates a stream's segments can
	// serve its live HLS. WithoutCancel keeps the feed alive past this request.
	// Only published segments are actually folded in (see feedLiveWindow).
	go mm.feedLiveWindow(context.WithoutCancel(ctx), vs.repoDID, seg, meta.Published)

	// The on-disk segment is transient now: durable copies live in S3/VOD, and
	// disk archival is only a short-lived scratch for moderation clips + the
	// no-S3 fallback. Every segment gets an expiry capped at SegmentArchiveRetention
	// (default 1h) so nothing lingers — a -1 ("indefinite archival") distribution
	// policy no longer pins segments to disk forever. An explicit *shorter* policy
	// expiry still wins. SegmentArchiveRetention <= 0 is the operator opt-out that
	// keeps segments until manually cleaned.
	var deleteAfter *time.Time
	if mm.cli.SegmentArchiveRetention > 0 {
		expiry := time.Now().Add(mm.cli.SegmentArchiveRetention).UTC()
		if meta.DistributionPolicy != nil && meta.DistributionPolicy.DeleteAfterSeconds != nil {
			if secs := *meta.DistributionPolicy.DeleteAfterSeconds; secs >= 0 {
				if policyExpiry := meta.StartTime.Time().Add(time.Duration(secs) * time.Second); policyExpiry.Before(expiry) {
					expiry = policyExpiry
				}
			}
		}
		deleteAfter = &expiry
	}
	dbSeg := &localdb.Segment{
		ID:                 vs.label,
		SigningKeyDID:      vs.signingKeyDID,
		RepoDID:            vs.repoDID,
		StartTime:          meta.StartTime.Time(),
		Title:              meta.Title,
		Size:               len(seg),
		MediaData:          vs.mediaData,
		ContentWarnings:    localdb.ContentWarningsSlice(meta.ContentWarnings),
		ContentRights:      meta.ContentRights,
		DistributionPolicy: meta.DistributionPolicy,
		DeleteAfter:        deleteAfter,
		Published:          meta.Published,
	}
	// The archived bytes are the bare canonical .m4s. The local distribution
	// pipelines (HLS transmux, WebRTC, transcode, thumbnail) still consume a
	// presentation MP4, so synthesize a flat MP4 over the canonical segments for
	// the notification's Data field; the segment bytes (and their signatures)
	// pass through verbatim in the mdat envelope. Replication forwarders instead
	// ship the bare canonical Muxl bytes (see the iroh and websocket senders),
	// which a receiving node re-validates unchanged.
	var playable bytes.Buffer
	if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(seg), "flat", &playable); err != nil {
		return fmt.Errorf("wrap segment for distribution: %w", err)
	}

	mm.newSegmentSubsMutex.RLock()
	defer mm.newSegmentSubsMutex.RUnlock()
	not := &NewSegmentNotification{
		Segment:  dbSeg,
		Data:     playable.Bytes(),
		Muxl:     seg,
		Metadata: meta,
		Local:    vs.local,
	}
	for _, ch := range mm.newSegmentSubs {
		go func() {
			select {
			case ch <- not:
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Minute):
				log.Warn(ctx, "failed to send segment to channel, timing out", "streamer", vs.repoDID, "signingKey", vs.signingKeyDID, "segmentID", vs.label)
				return
			}
		}()
	}
	aqt := aqtime.FromTime(meta.StartTime.Time())
	log.Log(ctx, "successfully ingested segment", "user", vs.repoDID, "signingKey", vs.signingKeyDID, "timestamp", aqt.FileSafeString(), "segmentID", vs.label)
	return nil
}

// applyContentFilters applies content filtering based on configured rules
func (mm *MediaManager) applyContentFilters(ctx context.Context, meta *SegmentMetadata) error {
	// Check content warnings (if enabled)
	if mm.cli.ContentFilters.ContentWarnings.Enabled {
		for _, warning := range meta.ContentWarnings {
			if mm.isWarningBlocked(warning) {
				reason := fmt.Sprintf("content warning blocked: %s", warning)
				log.Log(ctx, "content filtered",
					"reason", reason,
					"filter_type", "content_warning",
					"creator", meta.Creator,
					"warning", warning)
				return fmt.Errorf("content filtered: %s", reason)
			}
		}
	}

	// Check distribution policy (if enabled)
	if mm.cli.ContentFilters.DistributionPolicy.Enabled && meta.DistributionPolicy != nil {
		if meta.DistributionPolicy.DeleteAfterSeconds != nil {
			expiresAt := meta.StartTime.Time().Add(time.Duration(*meta.DistributionPolicy.DeleteAfterSeconds) * time.Second)
			if time.Now().After(expiresAt) {
				reason := fmt.Sprintf("distribution policy expired: segment expires at %s", expiresAt)
				log.Log(ctx, "content filtered",
					"reason", reason,
					"filter_type", "distribution_policy",
					"creator", meta.Creator,
					"start_time", meta.StartTime,
					"expires_at", expiresAt)
				return fmt.Errorf("content filtered: %s", reason)
			}
		}
	}

	return nil
}

// isWarningBlocked checks if a content warning is in the blocked list
func (mm *MediaManager) isWarningBlocked(warning string) bool {
	for _, blocked := range mm.cli.ContentFilters.ContentWarnings.BlockedWarnings {
		if warning == blocked {
			return true
		}
	}
	return false
}

type ValidationResult struct {
	Pub       *atcrypto.PublicKeyK256
	Meta      *SegmentMetadata
	MediaData *localdb.SegmentMediaData
	Manifest  *c2patypes.Manifest
	Cert      string
}

func ValidateMP4Media(ctx context.Context, buf []byte) (*ValidationResult, error) {
	g, ctx := errgroup.WithContext(ctx)
	var mediaData *localdb.SegmentMediaData
	var validationResult *ValidationResult
	g.Go(func() error {
		// gstreamer needs a parseable MP4, but the bytes on the wire/disk are
		// bare canonical .m4s. Synthesize a flat MP4 header over them — the
		// per-track uuid-prefixed segments land in the mdat envelope with
		// correct co64 offsets, exactly the shape qtdemux already parses.
		var flat bytes.Buffer
		if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(buf), "flat", &flat); err != nil {
			return fmt.Errorf("wrap segment for media parse: %w", err)
		}
		var err error
		mediaData, err = ValidateMP4MediaData(ctx, flat.Bytes())
		return err
	})
	g.Go(func() error {
		var err error
		validationResult, err = ValidateMP4MediaC2PA(ctx, buf)
		return err
	})
	err := g.Wait()
	if err != nil {
		return nil, err
	}
	validationResult.MediaData = mediaData
	return validationResult, nil
}

func ValidateMP4MediaData(ctx context.Context, buf []byte) (*localdb.SegmentMediaData, error) {
	tracer := otel.Tracer("signer")
	// Drives a gst pipeline (qtdemux + h264parse + opusparse + appsinks) —
	// any tail here means a gst element is misbehaving, not the c2pa
	// signing stack.
	mdCtx, mdSpan := tracer.Start(ctx, "ValidateMP4Media.ParseSegmentMediaData")
	mediaData, err := ParseSegmentMediaData(mdCtx, buf)
	mdSpan.End()
	if err != nil {
		return nil, err
	}
	return mediaData, nil
}

// validate a signed mp4 file unto itself, ignoring whether this user is allowed and whatnot
func ValidateMP4MediaC2PA(ctx context.Context, buf []byte) (*ValidationResult, error) {
	tracer := otel.Tracer("signer")
	ctx, span := tracer.Start(ctx, "ValidateMP4Media", trace.WithAttributes(
		attribute.Int("bytes", len(buf)),
	))
	defer span.End()

	// Validate every canonical segment entirely inside the muxl wasm. `verify`
	// unwraps any wrapper — a bare .m4s stream, a MUXL fMP4, or the legacy
	// signed flat MP4 — into its per-track signed segments and runs c2pa-rs's
	// Reader over each as the standalone "m4s" asset it was signed as. This
	// replaces the iroh-streamplace get_manifest_and_cert binding: the c2pa
	// dependency no longer leaves the wasm sandbox.
	_, verifySpan := tracer.Start(ctx, "ValidateMP4Media.muxl.Verify")
	out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(buf))
	verifySpan.End()
	if err != nil {
		return nil, fmt.Errorf("muxl verify failed: %w", err)
	}

	var doc struct {
		Segments []segmentValidation `json:"segments"`
	}
	_, unmarshalSpan := tracer.Start(ctx, "ValidateMP4Media.UnmarshalManifest", trace.WithAttributes(
		attribute.Int("json_bytes", len(out)),
	))
	err = json.Unmarshal([]byte(out), &doc)
	unmarshalSpan.End()
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal verify output: %w", err)
	}
	if len(doc.Segments) == 0 {
		return nil, fmt.Errorf("no signed canonical segments found in input")
	}

	// Every track's segment must validate — a tampered audio track fails the
	// whole GoP, not just the primary one.
	for _, seg := range doc.Segments {
		if am := seg.ValidationResults.ActiveManifest; am != nil && len(am.Failure) > 0 {
			bs, _ := json.Marshal(am.Failure)
			return nil, fmt.Errorf("track %d active manifest has failures: %s", seg.TrackID, string(bs))
		}
		if seg.ValidationState == "Invalid" {
			return nil, fmt.Errorf("track %d canonical segment failed c2pa validation", seg.TrackID)
		}
	}

	// The primary (first) segment carries the segment-level metadata + cert.
	// All tracks share the same manifest body (segment == wrapper manifest at
	// sign time), so this preserves the old wrapper-manifest semantics.
	primary := doc.Segments[0]

	_, certSpan := tracer.Start(ctx, "ValidateMP4Media.ParseES256KCert")
	pub, err := signers.ParseES256KCert([]byte(primary.Cert))
	certSpan.End()
	if err != nil {
		return nil, err
	}

	assCtx, assSpan := tracer.Start(ctx, "ValidateMP4Media.ParseSegmentAssertions")
	meta, err := ParseSegmentAssertions(assCtx, &primary.Manifest)
	assSpan.End()
	if err != nil {
		return nil, err
	}

	return &ValidationResult{
		Pub:       pub,
		Meta:      meta,
		MediaData: nil, // filled in by ValidateMP4MediaData
		Manifest:  &primary.Manifest,
		Cert:      primary.Cert,
	}, nil
}
