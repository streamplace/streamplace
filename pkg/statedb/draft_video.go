package statedb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/streamplace"
)

// DraftVideo is a draft VOD: a permissioned-data-style record (CBOR-serialized,
// addressed by an ats:// URI) stored in statedb until the real permissioned-data
// spec ships (bluesky-social/proposals PR #94, the 0016-permissioned-data
// proposal). The CBOR record body (Data) holds the user-editable + processing
// fields exactly as a future place.stream.video value plus status/error; the
// server-internal fields (ContentCID, OriginUploadID) live only in the SQL row
// so the CBOR body is a clean migration target.
type DraftVideo struct {
	// URI is the ats:// URI identifying this draft (personal-space shape:
	// ats://{did}/place.stream.vod.drafts/self/{did}/place.stream.vod.draftVideo/{tid}).
	URI     string `gorm:"column:uri;primaryKey"`
	UserDID string `gorm:"column:user_did;index;not null"`
	// Data is the CBOR-encoded place.stream.vod.draftVideo record body.
	Data []byte `gorm:"column:data;type:bytes"`
	// CID is the sha256-of-CBOR CIDv1 of Data (computed via spid.GetCIDFromBytes).
	CID string `gorm:"column:cid"`
	// ContentCID is the MUXL CID of the processed content blob. Server-internal:
	// used to generate a thumbnail at publish time. Not part of the CBOR record.
	ContentCID string `gorm:"column:content_cid"`
	// OriginUploadID ties this draft back to the Upload row whose processing
	// produced it (so the queue processor can update the draft on completion).
	// Server-internal; not part of the CBOR record.
	OriginUploadID string    `gorm:"column:origin_upload_id;index"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (DraftVideo) TableName() string {
	return "draft_videos"
}

// draftSpaceType / draftSkey encode the personal-space addressing used for all
// draft VODs. The space authority is the user's own DID (personal data); the
// space type is an NSID string only (no "type":"space" lexicon is defined, to
// avoid the indigo-fork lexgen compatibility risk); the skey is the spec's
// reserved personal-data key "self".
const (
	draftSpaceType  = "place.stream.vod.drafts"
	draftSkey       = "self"
	draftCollection = "place.stream.vod.draftVideo"
)

// DraftURI builds the ats:// URI for a draft owned by did with record key tid.
// ats://{did}/place.stream.vod.drafts/self/{did}/place.stream.vod.draftVideo/{tid}
func DraftURI(did, tid string) string {
	return fmt.Sprintf("ats://%s/%s/%s/%s/%s/%s", did, draftSpaceType, draftSkey, did, draftCollection, tid)
}

// marshalDraft CBOR-encodes a draft record and computes its CID.
func marshalDraft(rec *streamplace.VodDraftVideo) (data []byte, cidStr string, err error) {
	buf := bytes.NewBuffer(nil)
	if err := rec.MarshalCBOR(buf); err != nil {
		return nil, "", fmt.Errorf("marshal draft CBOR: %w", err)
	}
	data = buf.Bytes()
	c, err := spid.GetCIDFromBytes(data)
	if err != nil {
		return nil, "", fmt.Errorf("compute draft CID: %w", err)
	}
	return data, c.String(), nil
}

// unmarshalDraft CBOR-decodes a draft record body.
func unmarshalDraft(data []byte) (*streamplace.VodDraftVideo, error) {
	var rec streamplace.VodDraftVideo
	if err := rec.UnmarshalCBOR(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("unmarshal draft CBOR: %w", err)
	}
	return &rec, nil
}

// CreateDraft stores a new draft record. The record's CID and the row's URI are
// derived from the record; originUploadID ties it to its processing job (may be
// empty for drafts created outside an upload/finalize flow).
func (state *StatefulDB) CreateDraft(ctx context.Context, did, originUploadID string, rec *streamplace.VodDraftVideo) (*DraftVideo, error) {
	data, cidStr, err := marshalDraft(rec)
	if err != nil {
		return nil, err
	}
	// The record key (rkey) is a TID; reuse the LexiconTypeID-free path by
	// minting one here. spid.TIDClock is the same source publishVideo uses.
	tid := spid.TIDClock.Next().String()
	dv := &DraftVideo{
		URI:            DraftURI(did, tid),
		UserDID:        did,
		Data:           data,
		CID:            cidStr,
		OriginUploadID: originUploadID,
	}
	if err := state.DB.WithContext(ctx).Create(dv).Error; err != nil {
		return nil, fmt.Errorf("create draft: %w", err)
	}
	return dv, nil
}

// GetDraft fetches a draft by its ats:// URI. Returns (nil, nil) if not found.
func (state *StatefulDB) GetDraft(ctx context.Context, uri string) (*DraftVideo, error) {
	var dv DraftVideo
	err := state.DB.WithContext(ctx).Where("uri = ?", uri).First(&dv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get draft: %w", err)
	}
	return &dv, nil
}

// GetDraftByUpload fetches the draft tied to an Upload row's processing job.
// Returns (nil, nil) if no draft exists for that upload.
func (state *StatefulDB) GetDraftByUpload(ctx context.Context, originUploadID string) (*DraftVideo, error) {
	var dv DraftVideo
	err := state.DB.WithContext(ctx).Where("origin_upload_id = ?", originUploadID).First(&dv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get draft by upload: %w", err)
	}
	return &dv, nil
}

// ListDrafts returns the drafts owned by did, newest first, with simple
// time-cursor pagination (cursor is the CreatedAt of the last item returned,
// RFC3339Nano; empty for the first page).
func (state *StatefulDB) ListDrafts(ctx context.Context, did string, limit int, cursor string) ([]*DraftVideo, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := state.DB.WithContext(ctx).Where("user_did = ?", did).Order("created_at DESC").Limit(limit)
	if cursor != "" {
		t, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		q = q.Where("created_at < ?", t)
	}
	var drafts []*DraftVideo
	if err := q.Find(&drafts).Error; err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	return drafts, nil
}

// UpdateDraftMetadata applies a partial update of editable fields only. It never
// touches source/durationMs/status/error — those are server-authoritative. The
// CID is recomputed from the new CBOR body. Returns the updated row.
func (state *StatefulDB) UpdateDraftMetadata(ctx context.Context, uri string, apply func(rec *streamplace.VodDraftVideo)) (*DraftVideo, error) {
	var dv DraftVideo
	err := state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// FOR UPDATE (Postgres) so a concurrent SetDraftReady/SetDraftError
		// transaction blocks until this one commits — preventing a lost update
		// where the user's metadata edit clobbers the server's ready/error
		// transition (or vice versa). SQLite serializes writes, so the clause
		// is a no-op there.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uri = ?", uri).First(&dv).Error; err != nil {
			return err
		}
		rec, err := unmarshalDraft(dv.Data)
		if err != nil {
			return err
		}
		apply(rec)
		data, cidStr, err := marshalDraft(rec)
		if err != nil {
			return err
		}
		dv.Data = data
		dv.CID = cidStr
		return tx.Model(&DraftVideo{}).Where("uri = ?", uri).
			Updates(map[string]any{
				"data": data,
				"cid":  cidStr,
			}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("update draft metadata: %w", err)
	}
	return &dv, nil
}

// SetDraftReady flips a draft to status "ready" and fills the server-authoritative
// fields (source, durationMs) in the CBOR body, plus content_cid on the SQL row.
// sourceTracks is the JSON string of {uri,cid} track refs from the Upload row.
// The caller is expected to have built the source union member from that JSON.
func (state *StatefulDB) SetDraftReady(ctx context.Context, originUploadID string, source *streamplace.VodDraftVideo_Source, durationMs int64, contentCID string) error {
	return state.updateDraftByUpload(ctx, originUploadID, func(rec *streamplace.VodDraftVideo) {
		rec.Status = "ready"
		rec.Source = source
		rec.DurationMs = &durationMs
		rec.Error = nil
	}, func(dv *DraftVideo) {
		dv.ContentCID = contentCID
	})
}

// markDraftReadyFromUpload re-reads a (now-finished) Upload row and flips its
// tied draft to 'ready', filling source/durationMs from the row and content_cid
// for later thumbnail generation. Called by the queue processors after the VOD
// processor / livestream finalizer returns, since those call SetUploadProcessed
// internally and return only a cid — the processor's results land on the Upload
// row, which we re-read here. A missing draft is a no-op.
func (state *StatefulDB) markDraftReadyFromUpload(ctx context.Context, uploadID string) error {
	upload, err := state.GetUpload(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("get upload: %w", err)
	}
	if upload == nil {
		return nil
	}
	source, err := draftSourceFromTrackURIs(upload.TrackURIs)
	if err != nil {
		return err
	}
	return state.SetDraftReady(ctx, uploadID, source, upload.DurationMS, upload.ContentCID)
}

// draftSourceFromTrackURIs builds the draft's source union (sourceTracks) from
// the {uri,cid} JSON array the Upload row stores — the same conversion
// vod.sourceTracksFromUpload performs for the publishVideo path, replicated
// here because pkg/statedb can't import pkg/vod (cycle).
func draftSourceFromTrackURIs(trackURIsJSON string) (*streamplace.VodDraftVideo_Source, error) {
	if trackURIsJSON == "" {
		return nil, nil
	}
	var refs []struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := json.Unmarshal([]byte(trackURIsJSON), &refs); err != nil {
		return nil, fmt.Errorf("decode track refs: %w", err)
	}
	tracks := make([]*comatproto.RepoStrongRef, 0, len(refs))
	for _, r := range refs {
		tracks = append(tracks, &comatproto.RepoStrongRef{Uri: r.URI, Cid: r.CID})
	}
	return &streamplace.VodDraftVideo_Source{
		MediaDefs_SourceTracks: &streamplace.MediaDefs_SourceTracks{
			LexiconTypeID: "place.stream.media.defs#sourceTracks",
			Tracks:        tracks,
		},
	}, nil
}

// SetDraftError flips a draft to status "error" with an error message.
func (state *StatefulDB) SetDraftError(ctx context.Context, originUploadID, errMsg string) error {
	return state.updateDraftByUpload(ctx, originUploadID, func(rec *streamplace.VodDraftVideo) {
		rec.Status = "error"
		rec.Error = &errMsg
	}, nil)
}

// updateDraftByUpload rewrites the CBOR body (via apply) and, optionally, the
// denormalized SQL columns (via applyRow) for the draft tied to originUploadID.
func (state *StatefulDB) updateDraftByUpload(ctx context.Context, originUploadID string, apply func(rec *streamplace.VodDraftVideo), applyRow func(dv *DraftVideo)) error {
	return state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// FOR UPDATE (Postgres) — see UpdateDraftMetadata. A concurrent user
		// metadata edit blocks until this ready/error transition commits, so it
		// re-reads the new status rather than clobbering it.
		var dv DraftVideo
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("origin_upload_id = ?", originUploadID).First(&dv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// No draft for this upload (e.g. a pre-drafts-era upload, or a
				// draft that was deleted). Nothing to update; not an error.
				return nil
			}
			return err
		}
		rec, err := unmarshalDraft(dv.Data)
		if err != nil {
			return err
		}
		apply(rec)
		data, cidStr, err := marshalDraft(rec)
		if err != nil {
			return err
		}
		updates := map[string]any{
			"data": data,
			"cid":  cidStr,
		}
		if applyRow != nil {
			applyRow(&dv)
			updates["content_cid"] = dv.ContentCID
		}
		return tx.Model(&DraftVideo{}).Where("uri = ?", dv.URI).Updates(updates).Error
	})
}

// DeleteDraft removes a draft row. Returns whether a row was deleted.
func (state *StatefulDB) DeleteDraft(ctx context.Context, uri string) (bool, error) {
	res := state.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&DraftVideo{})
	if res.Error != nil {
		return false, fmt.Errorf("delete draft: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// ToDraftView converts a stored row + its CBOR record into the lexicon view.
func (dv *DraftVideo) ToDraftView() (*streamplace.VodDraftDefs_DraftView, error) {
	rec, err := unmarshalDraft(dv.Data)
	if err != nil {
		return nil, err
	}
	return &streamplace.VodDraftDefs_DraftView{
		Uri:    dv.URI,
		Cid:    dv.CID,
		Record: rec,
	}, nil
}
