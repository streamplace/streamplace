package statedb

import (
	"regexp"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	placestream "stream.place/streamplace/pkg/placestream"
)

// Storage represents S3 storage configuration for a user
type Storage struct {
	ID        string    `gorm:"column:id;primarykey"`
	IsActive  bool      `gorm:"column:is_active;default:true"`
	UserDID   string    `gorm:"column:user_did;not null;unique"`
	URL       string    `gorm:"column:url;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (s *Storage) TableName() string {
	return "storage"
}

func maskSecretKey(url string) string {
	// Format: s3+https://ACCESS_KEY:SECRET_KEY@endpoint/bucket
	re := regexp.MustCompile(`(s3\+https?://[^:]+:)([^@]+)(@.+)`)
	return re.ReplaceAllString(url, "${1}***${3}")
}

func (state *StatefulDB) UpsertStorage(storage *Storage) error {
	if storage.ID == "" {
		storage.ID = uuid.New().String()
	}

	var existing Storage
	err := state.DB.Where("user_did = ?", storage.UserDID).First(&existing).Error
	switch err {
	case nil:
		storage.ID = existing.ID
		storage.CreatedAt = existing.CreatedAt
		return state.DB.Save(storage).Error
	case gorm.ErrRecordNotFound:
		return state.DB.Create(storage).Error
	}

	return err
}

func (state *StatefulDB) GetStorage(userDID string) (*Storage, error) {
	var storage Storage
	err := state.DB.Where("user_did = ?", userDID).First(&storage).Error
	if err != nil {
		return nil, err
	}
	return &storage, nil
}

func (state *StatefulDB) DeleteStorage(userDID string) error {
	return state.DB.Where("user_did = ?", userDID).Delete(&Storage{}).Error
}

func (s *Storage) ToLexicon() placestream.ServerDefs_Storage {
	return placestream.ServerDefs_Storage{
		IsActive: s.IsActive,
		Url:      maskSecretKey(s.URL),
	}
}

func StorageFromLexiconInput(input placestream.ServerUpsertStorage_Input, userDID string) *Storage {

	var url string
	if input.Url != nil {
		url = *input.Url
	}

	storage := &Storage{
		UserDID:  userDID,
		URL:      url,
		IsActive: true,
	}

	if input.IsActive != nil {
		storage.IsActive = *input.IsActive
	}

	return storage
}
