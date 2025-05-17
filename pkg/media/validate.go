package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
	"stream.place/streamplace/pkg/model"

	"git.stream.place/streamplace/c2pa-go/pkg/c2pa"
)

func (mm *MediaManager) ValidateMP4(ctx context.Context, input io.Reader) error {
	ctx, span := otel.Tracer("signer").Start(ctx, "ValidateMP4")
	defer span.End()
	buf, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	r := bytes.NewReader(buf)
	reader, err := c2pa.FromStream(r, "video/mp4")
	if err != nil {
		return err
	}
	mani := reader.GetActiveManifest()
	certs := reader.GetProvenanceCertChain()
	pub, err := signers.ParseES256KCert([]byte(certs))
	if err != nil {
		return err
	}
	meta, err := ParseSegmentAssertions(ctx, mani)
	if err != nil {
		return err
	}
	mediaData, err := ParseSegmentMediaData(ctx, buf)
	if err != nil {
		return err
	}
	// special case for test signers that are only signed with a key
	var repoDID string
	var signingKeyDID string
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

	err = mm.cli.StreamIsAllowed(repoDID)
	if err != nil {
		return fmt.Errorf("got valid segment, but user %s is not allowed: %w", repoDID, err)
	}
	fd, err := mm.cli.SegmentFileCreate(repoDID, meta.StartTime, "mp4")
	if err != nil {
		return err
	}
	defer fd.Close()
	go mm.replicator.NewSegment(ctx, buf)
	r = bytes.NewReader(buf)
	if _, err := io.Copy(fd, r); err != nil {
		return err
	}
	scmSeg := &segchanman.Seg{
		Filepath: fd.Name(),
		Data:     buf,
	}
	go mm.PublishSegment(ctx, repoDID, "source", scmSeg)
	seg := &model.Segment{
		ID:            *mani.Label,
		SigningKeyDID: signingKeyDID,
		RepoDID:       repoDID,
		StartTime:     meta.StartTime.Time(),
		Title:         meta.Title,
		MediaData:     mediaData,
	}
	mm.newSegmentSubsMutex.RLock()
	defer mm.newSegmentSubsMutex.RUnlock()
	not := &NewSegmentNotification{
		Segment:  seg,
		Data:     buf,
		Metadata: meta,
	}
	for _, ch := range mm.newSegmentSubs {
		go func() { ch <- not }()
	}
	aqt := aqtime.FromTime(meta.StartTime.Time())
	log.Log(ctx, "successfully ingested segment", "user", repoDID, "signingKey", signingKeyDID, "timestamp", aqt.FileSafeString(), "segmentID", *mani.Label)
	return nil
}
