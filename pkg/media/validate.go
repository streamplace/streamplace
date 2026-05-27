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

	valid, err := ValidateMP4Media(ctx, buf)
	if err != nil {
		return fmt.Errorf("failed to validate MP4 media: %w", err)
	}
	meta := valid.Meta
	pub := valid.Pub
	mediaData := valid.MediaData
	manifest := valid.Manifest

	label := manifest.Label
	if label != nil && mm.model != nil {
		_, dbSpan := tracer.Start(ctx, "ValidateMP4.LocalDB.GetSegment")
		oldSeg, err := mm.localDB.GetSegment(*label)
		dbSpan.End()
		if err != nil {
			return fmt.Errorf("failed to get old segment: %w", err)
		}
		if oldSeg != nil {
			log.Warn(ctx, "segment already exists, skipping", "segmentID", *label)
			return nil
		}
	}

	if meta.MetadataConfiguration != nil {
		if meta.MetadataConfiguration.DistributionPolicy != nil {
			allowedBroadcasters := meta.MetadataConfiguration.DistributionPolicy.AllowedBroadcasters
			if allowedBroadcasters != nil {
				if !slices.Contains(allowedBroadcasters, "*") && !slices.Contains(allowedBroadcasters, fmt.Sprintf("did:web:%s", mm.cli.BroadcasterHost)) {
					return fmt.Errorf("broadcaster %s is not allowed to distribute content. Allowed broadcasters: %v", fmt.Sprintf("did:web:%s", mm.cli.BroadcasterHost), allowedBroadcasters)
				}
			}
		}
	}

	var repoDID string
	var signingKeyDID string
	// special case for test signers that are only signed with a key
	if strings.HasPrefix(meta.Creator, constants.DID_KEY_PREFIX) {
		signingKeyDID = meta.Creator
		repoDID = meta.Creator
	} else {
		atCtx, atSpan := tracer.Start(ctx, "ValidateMP4.SyncBlueskyRepoCached")
		repo, err := mm.atsync.SyncBlueskyRepoCached(atCtx, meta.Creator)
		atSpan.End()
		if err != nil {
			return err
		}
		modelCtx, modelSpan := tracer.Start(ctx, "ValidateMP4.GetSigningKey")
		signingKey, err := mm.model.GetSigningKey(modelCtx, pub.DIDKey(), repo.DID)
		modelSpan.End()
		if err != nil {
			return err
		}
		if signingKey == nil {
			return fmt.Errorf("no signing key found for %s", pub.DIDKey())
		}
		repoDID = repo.DID
		signingKeyDID = signingKey.DID
	}

	err = mm.cli.StreamIsAllowed(repoDID)
	if err != nil {
		return fmt.Errorf("got valid segment, but user %s is not allowed: %w", repoDID, err)
	}

	// Apply content filtering after metadata is parsed
	if mm.cli.ContentFilters != nil {
		if err := mm.applyContentFilters(ctx, meta); err != nil {
			return err
		}
	}

	// Complete the segment's audio so it carries both AAC and Opus (no-op when
	// already dual-codec, audio-only, or the codec is neither). The added track
	// is signed under the node's own identity as a c2pa.transcoded derivative
	// of the source audio track. Everything archived/distributed below uses the
	// completed bytes; replicas re-run this and find it already complete.
	completed, err := mm.completeAudioCodecs(ctx, buf)
	if err != nil {
		return fmt.Errorf("complete audio codecs: %w", err)
	}
	buf = completed

	_, fileSpan := tracer.Start(ctx, "ValidateMP4.SegmentArchiveWrite", trace.WithAttributes(
		attribute.Int("bytes", len(buf)),
	))
	fd, err := mm.cli.SegmentFileCreate(repoDID, meta.StartTime, "m4s")
	if err != nil {
		fileSpan.End()
		return err
	}
	defer fd.Close()

	r := bytes.NewReader(buf)
	if _, err := io.Copy(fd, r); err != nil {
		fileSpan.End()
		return err
	}
	fileSpan.End()

	// Fold the validated segment into the streamer's live-HLS window, off the
	// critical path. This runs for every segment — locally signed or
	// replicated from another node — so any node that validates a stream's
	// segments can serve its live HLS. WithoutCancel keeps the feed alive past
	// this request's context.
	go mm.feedLiveWindow(context.WithoutCancel(ctx), repoDID, buf)

	var deleteAfter *time.Time
	if meta.DistributionPolicy != nil && meta.DistributionPolicy.DeleteAfterSeconds != nil {
		secs := *meta.DistributionPolicy.DeleteAfterSeconds
		if secs == -1 {
			deleteAfter = nil
		} else {
			expiryTime := meta.StartTime.Time().Add(time.Duration(secs) * time.Second)
			deleteAfter = &expiryTime
		}
	} else {
		if mm.cli.SegmentArchiveRetention.Seconds() != 0 {
			tomorrow := time.Now().Add(mm.cli.SegmentArchiveRetention).UTC()
			deleteAfter = &tomorrow
		}
	}
	seg := &localdb.Segment{
		ID:                 *label,
		SigningKeyDID:      signingKeyDID,
		RepoDID:            repoDID,
		StartTime:          meta.StartTime.Time(),
		Title:              meta.Title,
		Size:               len(buf),
		MediaData:          mediaData,
		ContentWarnings:    localdb.ContentWarningsSlice(meta.ContentWarnings),
		ContentRights:      meta.ContentRights,
		DistributionPolicy: meta.DistributionPolicy,
		DeleteAfter:        deleteAfter,
		Published:          meta.Published,
	}
	// The bytes archived above are the bare canonical .m4s. The local
	// distribution pipelines (HLS transmux, WebRTC, transcode, thumbnail) still
	// consume a presentation MP4, so synthesize a flat MP4 over the canonical
	// segments for the notification's Data field; the segment bytes (and their
	// signatures) pass through verbatim in the mdat envelope. Replication
	// forwarders instead ship the bare canonical Muxl bytes (see the iroh and
	// websocket senders), which a receiving node re-validates unchanged.
	var playable bytes.Buffer
	if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(buf), "flat", &playable); err != nil {
		return fmt.Errorf("wrap segment for distribution: %w", err)
	}

	mm.newSegmentSubsMutex.RLock()
	defer mm.newSegmentSubsMutex.RUnlock()
	not := &NewSegmentNotification{
		Segment:  seg,
		Data:     playable.Bytes(),
		Muxl:     buf,
		Metadata: meta,
		Local:    local,
	}
	for _, ch := range mm.newSegmentSubs {
		go func() {
			select {
			case ch <- not:
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Minute):
				log.Warn(ctx, "failed to send segment to channel, timing out", "streamer", repoDID, "signingKey", signingKeyDID, "segmentID", *label)
				return
			}
		}()
	}
	aqt := aqtime.FromTime(meta.StartTime.Time())
	log.Log(ctx, "successfully ingested segment", "user", repoDID, "signingKey", signingKeyDID, "timestamp", aqt.FileSafeString(), "segmentID", *label)
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
