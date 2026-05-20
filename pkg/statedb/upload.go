package statedb

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Upload tracks a TUS resumable upload from a Streamplace user, regardless of
// whether the bytes land on local disk or in S3. Lifecycle: a row is created
// by the createUpload XRPC, updated with CompletedAt when the TUS handler's
// OnUploadFinished hook fires, and read by the VOD processing task.
type Upload struct {
	// ID is the TUS upload identifier (also embedded in the bearer token).
	ID          string     `gorm:"column:id;primarykey"`
	RepoDID     string     `gorm:"column:user_did;index;not null"`
	MimeType    string     `gorm:"column:mime_type"`
	Filename    string     `gorm:"column:filename"`
	Size        int64      `gorm:"column:size"`
	Backend     string     `gorm:"column:backend"` // "file" or "s3"
	Location    string     `gorm:"column:location"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`

	// Processing fields — set by the VOD pipeline after the TUS upload finishes.
	// ProcessingStatus is "", "processing", "done", or "error".
	ProcessingStatus   string `gorm:"column:processing_status"`
	ProcessingError    string `gorm:"column:processing_error"`
	ProcessingProgress int    `gorm:"column:processing_progress;default:0"`
	// TrackURIs is a JSON array of {"uri":"at://...","cid":"..."} objects
	// populated once the track records are published and the video is ready
	// for the client to create a place.stream.video record.
	TrackURIs  string `gorm:"column:track_uris"`
	DurationMS int64  `gorm:"column:duration_ms"`
	// ContentCID is the BDASL CID of the processed fMP4 blob. Stored so the
	// server can locate the content blob + metafile later (e.g. for
	// publishVideo's thumbnail generation) without re-deriving it from the
	// published track records.
	ContentCID string `gorm:"column:content_cid"`
}

func (Upload) TableName() string {
	return "uploads"
}

func (state *StatefulDB) CreateUpload(ctx context.Context, u *Upload) error {
	return state.DB.WithContext(ctx).Create(u).Error
}

func (state *StatefulDB) GetUpload(ctx context.Context, id string) (*Upload, error) {
	var u Upload
	err := state.DB.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (state *StatefulDB) CompleteUpload(ctx context.Context, id string, location string) error {
	now := time.Now().UTC()
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"completed_at": &now,
			"location":     location,
		}).Error
}

func (state *StatefulDB) SetUploadProcessing(ctx context.Context, id string) error {
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"processing_status":   "processing",
			"processing_progress": 0,
		}).Error
}

func (state *StatefulDB) SetUploadProgress(ctx context.Context, id string, progress int) error {
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Update("processing_progress", progress).Error
}

func (state *StatefulDB) SetUploadProcessed(ctx context.Context, id string, trackURIsJSON string, durationMS int64, contentCID string) error {
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"processing_status":   "done",
			"processing_progress": 100,
			"track_uris":          trackURIsJSON,
			"duration_ms":         durationMS,
			"content_cid":         contentCID,
		}).Error
}

func (state *StatefulDB) SetUploadFailed(ctx context.Context, id string, errMsg string) error {
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"processing_status": "error",
			"processing_error":  errMsg,
		}).Error
}

// uploadAuthKeySize is the size in bytes of the HMAC key used to sign upload
// bearer tokens. 32 bytes is the recommended size for HS256.
const uploadAuthKeySize = 32

const uploadAuthKeyConfigKey = "upload-auth-key"

// GetOrCreateUploadAuthKey returns the symmetric HMAC key used to sign upload
// bearer tokens, lazily generating one on first call. The key is persisted in
// the stateful database so all nodes in a station agree, exactly like the
// repo-key flow in pkg/atproto/lexicon_repo.go.
func (state *StatefulDB) GetOrCreateUploadAuthKey() ([]byte, error) {
	existing, err := state.GetConfig(uploadAuthKeyConfigKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing.Value, nil
	}
	key := make([]byte, uploadAuthKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := state.PutConfig(uploadAuthKeyConfigKey, key); err != nil {
		return nil, err
	}
	return key, nil
}
