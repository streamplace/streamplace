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
	// TrackURIs is a JSON array of {"uri":"at://...","cid":"..."} objects.
	// Vestigial since track publication was deferred to publishDraft time:
	// new uploads leave this empty and PublishDraft publishes the tracks on
	// demand. Retained so existing rows / the legacy publishVideo path still
	// read it.
	TrackURIs  string `gorm:"column:track_uris"`
	DurationMS int64  `gorm:"column:duration_ms"`
	// ContentCID is the BDASL CID of the processed fMP4 blob. Stored so the
	// server can locate the content blob + metafile later (e.g. for
	// publishVideo's thumbnail generation) without re-deriving it from the
	// published track records.
	ContentCID string `gorm:"column:content_cid"`
	// SigningKey is the did:key whose ephemeral private half C2PA-signed the
	// segments. Stored at processing time so PublishDraft can publish the
	// place.stream.media.track records (which carry it) at publish time.
	SigningKey string `gorm:"column:signing_key"`
	// ProbeJSON is the gstreamer probe metadata (video/audio codec, dims,
	// fps, rate, channels) serialized as JSON. Stored at processing time so
	// PublishDraft can publish the track records (deferred from processing)
	// without re-probing the blob.
	ProbeJSON string `gorm:"column:probe_json"`
	// BlobSize is the byte size of the processed MUXL content blob (distinct
	// from Size, the raw upload size). Stored at processing time so
	// PublishDraft can populate the track records' size field without
	// re-statting the blob.
	BlobSize int64 `gorm:"column:blob_size"`
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

// SetUploadProcessed marks an upload done and stores the processing results
// the publishDraft path needs: duration, the content blob's CID, the C2PA
// signing key, and the gstreamer probe metadata (JSON). Track records are
// NOT published here — they're deferred to publishDraft time so half-
// published tracks don't go live before the video record. trackURIs is left
// empty (vestigial; the legacy publishVideo path may still set it).
func (state *StatefulDB) SetUploadProcessed(ctx context.Context, id string, durationMS int64, contentCID, signingKey, probeJSON string, blobSize int64) error {
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"processing_status":   "done",
			"processing_progress": 100,
			"duration_ms":         durationMS,
			"content_cid":         contentCID,
			"signing_key":         signingKey,
			"probe_json":          probeJSON,
			"blob_size":           blobSize,
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
