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
