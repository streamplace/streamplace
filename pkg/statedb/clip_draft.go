package statedb

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// ClipDraft is an ephemeral clip created by a viewer from a streamer's live
// broadcast. It stores the muxed MP4 path and metadata until the viewer
// publishes or the 10-minute TTL expires. Unlike VOD drafts, clip drafts
// don't go through the upload pipeline — the content is already muxed by
// ClipUser and the metadata is edited client-side.
type ClipDraft struct {
	ID          string `gorm:"column:id;primaryKey"`
	ClipperDID  string `gorm:"column:clipper_did;index;not null"`
	StreamerDID string `gorm:"column:streamer_did;index;not null"`
	FilePath    string `gorm:"column:file_path;not null"`
	DurationMs  int64  `gorm:"column:duration_ms"`
	// SigningKey is the streamer's C2PA signing key (did:key) used for the
	// live segments. Stored at creation time so publish can create track records.
	SigningKey string `gorm:"column:signing_key"`
	// ProbeJSON is serialized codec/dimension/fps info for the stream,
	// matching the VOD pipeline's probeJSONShape. Stored at creation time.
	ProbeJSON string    `gorm:"column:probe_json"`
	CreatedAt time.Time `gorm:"column:created_at"`
	ExpiresAt time.Time `gorm:"column:expires_at;index"`
	Published bool      `gorm:"column:published;default:false"`
}

func (ClipDraft) TableName() string {
	return "clip_drafts"
}

func (state *StatefulDB) CreateClipDraft(ctx context.Context, cd *ClipDraft) error {
	return state.DB.WithContext(ctx).Create(cd).Error
}

func (state *StatefulDB) GetClipDraft(ctx context.Context, id string) (*ClipDraft, error) {
	var cd ClipDraft
	err := state.DB.WithContext(ctx).Where("id = ?", id).First(&cd).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cd, nil
}

func (state *StatefulDB) MarkClipDraftPublished(ctx context.Context, id string) error {
	return state.DB.WithContext(ctx).Model(&ClipDraft{}).
		Where("id = ?", id).
		Update("published", true).Error
}

// DeleteClipDraft removes a clip draft row. The caller is responsible for
// deleting the ephemeral file first.
func (state *StatefulDB) DeleteClipDraft(ctx context.Context, id string) error {
	return state.DB.WithContext(ctx).Where("id = ?", id).Delete(&ClipDraft{}).Error
}

// DeleteExpiredClipDrafts removes clip drafts past their TTL. Returns the
// count deleted. The caller is responsible for deleting the ephemeral files.
func (state *StatefulDB) DeleteExpiredClipDrafts(ctx context.Context, now time.Time) (int64, error) {
	result := state.DB.WithContext(ctx).
		Where("expires_at < ? AND published = false", now).
		Delete(&ClipDraft{})
	return result.RowsAffected, result.Error
}

// ListExpiredClipDrafts returns unpublished clip drafts past their TTL, for
// file cleanup before deletion.
func (state *StatefulDB) ListExpiredClipDrafts(ctx context.Context, now time.Time) ([]ClipDraft, error) {
	var drafts []ClipDraft
	err := state.DB.WithContext(ctx).
		Where("expires_at < ? AND published = false", now).
		Find(&drafts).Error
	return drafts, err
}
