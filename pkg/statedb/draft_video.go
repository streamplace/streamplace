package statedb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	glex "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// DraftVideo anchors a draft's processing state. Data has editable metadata
// only on legacy rows that have not moved to the user's personal space.
type DraftVideo struct {
	// URI is the ats:// URI identifying this draft (personal-space shape:
	// ats://{did}/place.stream.vod.drafts/self/{did}/place.stream.vod.draftVideo/{tid}).
	URI     string `gorm:"column:uri;primaryKey"`
	UserDID string `gorm:"column:user_did;index;not null"`
	// Data is the CBOR-encoded place.stream.vod.draftVideo record body.
	Data []byte `gorm:"column:data;type:bytes"`
	// CID is the sha256-of-CBOR CIDv1 of Data (computed via spid.GetCIDFromBytes).
	CID string `gorm:"column:cid"`
	// Legacy (empty), pending worker creation, or materialized in the space.
	MetadataState string `gorm:"column:metadata_state"`
	// Non-nil while a create may have committed without a local acknowledgment.
	MetadataAttemptRev *string `gorm:"column:metadata_attempt_rev"`
	// Confirmed public result while private draft cleanup is outstanding.
	PublishedURI string `gorm:"column:published_uri"`
	PublishedCID string `gorm:"column:published_cid"`
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

// These are the stable API URI components, not the PDS collection address.
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
func marshalDraft(rec *placestream.VodDraftVideo) (data []byte, cidStr string, err error) {
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

// Record decodes the draft API record. Use Drafts to read joined metadata.
func (dv *DraftVideo) Record() (*placestream.VodDraftVideo, error) {
	return unmarshalDraft(dv.Data)
}

func unmarshalDraft(data []byte) (*placestream.VodDraftVideo, error) {
	var rec placestream.VodDraftVideo
	if err := rec.UnmarshalCBOR(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("unmarshal draft CBOR: %w", err)
	}
	return &rec, nil
}

func (state *StatefulDB) insertDraft(ctx context.Context, did, uploadID, metadataState, contentCID string, rec *placestream.VodDraftVideo) (*DraftVideo, error) {
	data, cidStr, err := marshalDraft(rec)
	if err != nil {
		return nil, err
	}
	dv := &DraftVideo{
		URI:            DraftURI(did, spid.TIDClock.Next().String()),
		UserDID:        did,
		Data:           data,
		CID:            cidStr,
		OriginUploadID: uploadID,
		ContentCID:     contentCID,
		MetadataState:  metadataState,
	}
	err = state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(dv).Error; err != nil {
			return err
		}
		if uploadID == "" {
			return nil
		}
		return tx.Model(&Upload{}).Where("id = ? AND user_did = ?", uploadID, did).Update("draft_uri", dv.URI).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create draft: %w", err)
	}
	return dv, nil
}

func (state *StatefulDB) getDraft(ctx context.Context, did, uri string) (*DraftVideo, error) {
	var dv DraftVideo
	err := state.DB.WithContext(ctx).Where("uri = ? AND user_did = ?", uri, did).First(&dv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get draft: %w", err)
	}
	return &dv, nil
}

func (state *StatefulDB) getDraftByUpload(ctx context.Context, originUploadID string) (*DraftVideo, error) {
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

// draftSourceFromTrackURIs builds the draft's source union (sourceTracks) from
// the {uri,cid} JSON array the Upload row stores — the same conversion
// vod.sourceTracksFromUpload performs for the publishVideo path, replicated
// here because pkg/statedb can't import pkg/vod (cycle).
func draftSourceFromTrackURIs(trackURIsJSON string) (*placestream.VodDraftVideo_Source, error) {
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
	tracks := make([]comatproto.RepoStrongRef, 0, len(refs))
	for _, r := range refs {
		tracks = append(tracks, comatproto.RepoStrongRef{Uri: r.URI, Cid: r.CID})
	}
	return &placestream.VodDraftVideo_Source{
		MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
			LexiconTypeID: "place.stream.media.defs#sourceTracks",
			Tracks:        tracks,
		},
	}, nil
}

// ToDraftView converts a stored row + its CBOR record into the lexicon view.
// The record field is typed as `unknown` in the lexicon (the @atproto/api
// client validator can't validate a ref to a record-type lexicon), which
// generates a *glex.LexiconTypeDecoder in Go — so we wrap the decoded
// record the same way commentView/livestreamView do.
func (dv *DraftVideo) ToDraftView() (placestream.VodDraftDefs_DraftView, error) {
	rec, err := dv.Record()
	if err != nil {
		return placestream.VodDraftDefs_DraftView{}, err
	}
	if rec.LexiconTypeID == "" {
		rec.LexiconTypeID = "place.stream.vod.draftVideo"
	}
	return placestream.VodDraftDefs_DraftView{
		Uri:    dv.URI,
		Cid:    dv.CID,
		Record: &glex.LexiconTypeDecoder{Val: rec},
	}, nil
}
