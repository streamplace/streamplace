package statedb

import (
	"fmt"

	"gorm.io/gorm"
)

// BrandingBlob stores branding assets for broadcasters
type BrandingBlob struct {
	gorm.Model
	BroadcasterID string `gorm:"index:idx_broadcaster_key,priority:1,unique"`
	Key           string `gorm:"index:idx_broadcaster_key,priority:2,unique"` // "mainLogo", "favicon", "siteTitle", etc.
	MimeType      string // "image/svg+xml", "image/png", "text/plain"
	Data          []byte `gorm:"type:bytea"` // actual blob data
	Width         *int   // image width in pixels (nullable)
	Height        *int   // image height in pixels (nullable)
}

// GetBrandingBlob fetches a single branding asset
func (state *StatefulDB) GetBrandingBlob(broadcasterID, key string) (*BrandingBlob, error) {
	var blob BrandingBlob
	err := state.DB.Where("broadcaster_id = ? AND key = ?", broadcasterID, key).First(&blob).Error
	if err != nil {
		return nil, err
	}
	return &blob, nil
}

// PutBrandingBlob stores or updates a branding asset
func (state *StatefulDB) PutBrandingBlob(broadcasterID, key, mimeType string, data []byte, width, height *int) error {
	// try to find existing blob (including soft-deleted ones)
	var existing BrandingBlob
	err := state.DB.Unscoped().Where("broadcaster_id = ? AND key = ?", broadcasterID, key).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// create new blob
		blob := BrandingBlob{
			BroadcasterID: broadcasterID,
			Key:           key,
			MimeType:      mimeType,
			Data:          data,
			Width:         width,
			Height:        height,
		}
		if err := state.DB.Create(&blob).Error; err != nil {
			return fmt.Errorf("error creating branding blob: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking for existing branding blob: %w", err)
	}

	// update existing blob (restore if soft-deleted)
	existing.MimeType = mimeType
	existing.Data = data
	existing.Width = width
	existing.Height = height
	existing.DeletedAt = gorm.DeletedAt{} // clear soft delete
	if err := state.DB.Unscoped().Save(&existing).Error; err != nil {
		return fmt.Errorf("error updating branding blob: %w", err)
	}

	return nil
}

// ListBrandingKeys returns all keys for a broadcaster
func (state *StatefulDB) ListBrandingKeys(broadcasterID string) ([]string, error) {
	var blobs []BrandingBlob
	err := state.DB.Where("broadcaster_id = ?", broadcasterID).Select("key").Find(&blobs).Error
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(blobs))
	for i, blob := range blobs {
		keys[i] = blob.Key
	}
	return keys, nil
}

// DeleteBrandingBlob removes a specific asset
func (state *StatefulDB) DeleteBrandingBlob(broadcasterID, key string) error {
	err := state.DB.Where("broadcaster_id = ? AND key = ?", broadcasterID, key).Delete(&BrandingBlob{}).Error
	if err != nil {
		return fmt.Errorf("error deleting branding blob: %w", err)
	}
	return nil
}
