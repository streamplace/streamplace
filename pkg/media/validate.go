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
	"stream.place/streamplace/pkg/aqio"
	"stream.place/streamplace/pkg/aqtime"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

type ManifestAndCert struct {
	Manifests           map[string]c2patypes.Manifest `json:"manifests"`
	Certs               map[string]string             `json:"certs"`
	ActiveManifestLabel string                        `json:"active_manifest_label"`
	ValidationResults   c2patypes.ValidationResults   `json:"validation_results"`
}

func (mm *MediaManager) ValidateMP4(ctx context.Context, input io.Reader, local bool) error {
	ctx, span := otel.Tracer("signer").Start(ctx, "ValidateMP4")
	defer span.End()
	buf, err := io.ReadAll(input)
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
		oldSeg, err := mm.model.GetSegment(*label)
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

	// Check streamer key is allowed
	var repoDID string
	var signingKeyDID string
	// special case for test signers that are only signed with a key
	if strings.HasPrefix(meta.Creator, constants.DID_KEY_PREFIX) {
		signingKeyDID = meta.Creator
		repoDID = meta.Creator
	} else {
		repo, err := mm.atsync.SyncBlueskyRepoCached(ctx, meta.Creator, mm.model)
		if err != nil {
			return err
		}
		signingKey, err := mm.model.GetSigningKey(ctx, pub.DIDKey(), repo.DID)
		if err != nil {
			return err
		}
		if signingKey == nil {
			return fmt.Errorf("no signing key found for %s", pub.DIDKey())
		}
		repoDID = repo.DID
		signingKeyDID = signingKey.DID
	}

	// Check the key the publisher used to sign is indeed the key the streamer allowed
	pubSigKey, err := mm.model.GetPublisherKey(ctx, valid.Publisher.Pub.DIDKey(), repoDID)
	if err != nil {
		return err
	}
	if pubSigKey == nil {
		return fmt.Errorf("no broadcaster signing key found for %s", valid.Publisher.Pub.DIDKey())
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

	fd, err := mm.cli.SegmentFileCreate(repoDID, meta.StartTime, "mp4")
	if err != nil {
		return err
	}
	defer fd.Close()

	r := bytes.NewReader(buf)
	if _, err := io.Copy(fd, r); err != nil {
		return err
	}
	var deleteAfter *time.Time
	if meta.DistributionPolicy != nil && meta.DistributionPolicy.DeleteAfterSeconds != nil {
		expiryTime := meta.StartTime.Time().Add(time.Duration(*meta.DistributionPolicy.DeleteAfterSeconds) * time.Second)
		deleteAfter = &expiryTime
	}
	seg := &model.Segment{
		ID:                 *label,
		SigningKeyDID:      signingKeyDID,
		RepoDID:            repoDID,
		StartTime:          meta.StartTime.Time(),
		Title:              meta.Title,
		Size:               len(buf),
		MediaData:          mediaData,
		ContentWarnings:    model.ContentWarningsSlice(meta.ContentWarnings),
		ContentRights:      meta.ContentRights,
		DistributionPolicy: meta.DistributionPolicy,
		DeleteAfter:        deleteAfter,
	}
	mm.newSegmentSubsMutex.RLock()
	defer mm.newSegmentSubsMutex.RUnlock()
	not := &NewSegmentNotification{
		Segment:  seg,
		Data:     buf,
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
	MediaData *model.SegmentMediaData
	Manifest  *c2patypes.Manifest
	Cert      string
	Publisher struct {
		Pub  *atcrypto.PublicKeyK256
		Cert string
	}
}

// validate a signed mp4 file unto itself, ignoring whether this user is allowed and whatnot
func ValidateMP4Media(ctx context.Context, buf []byte) (*ValidationResult, error) {
	var maniCert ManifestAndCert
	maniStr, err := iroh_streamplace.GetManifestAndCert(c2patypes.NewReader(aqio.NewReadWriteSeeker(buf)))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(maniStr), &maniCert)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest and cert: %w", err)
	}
	activeManifest := maniCert.ValidationResults.ActiveManifest
	if activeManifest != nil {
		if activeManifest.Failure == nil {
			return nil, fmt.Errorf("active manifest failure array not found?!")
		}
		if len(activeManifest.Failure) > 0 {
			bs, _ := json.Marshal(activeManifest.Failure)
			return nil, fmt.Errorf("active manifest has failures: %s", string(bs))
		}
	}

	if len(maniCert.Manifests) != 2 {
		log.Error(ctx, "ValidateMP4Media: not two manifest", "maniCert", maniCert)
		return nil, fmt.Errorf("can't handle media without two manifests (streamer and publisher)")
	}

	var valRes ValidationResult
	for label, manifest := range maniCert.Manifests {
		if label == maniCert.ActiveManifestLabel {
			// The active manifest should be the latest one, the publisher signing
			valRes.Publisher.Pub, err = signers.ParseES256KCert([]byte(maniCert.Certs[label]))
			if err != nil {
				return nil, err
			}
			valRes.Publisher.Cert = maniCert.Certs[label]
		} else {
			// First signing operation, streamer sign with metadata
			valRes.Pub, err = signers.ParseES256KCert([]byte(maniCert.Certs[label]))
			if err != nil {
				return nil, err
			}
			valRes.Cert = maniCert.Certs[label]
			valRes.Manifest = &manifest
			valRes.Meta, err = ParseSegmentAssertions(ctx, &manifest)
			if err != nil {
				return nil, err
			}
			valRes.MediaData, err = ParseSegmentMediaData(ctx, buf)
			if err != nil {
				return nil, err
			}
		}
	}

	return &valRes, nil
}
