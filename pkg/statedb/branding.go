package statedb

import (
	"errors"
	"fmt"
	"time"

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

// Branding is read a key at a time from many places (every HTML render
// consults every key for its meta tags; getBranding walks them all), which
// was one query per key — free on sqlite, a network round trip each on
// Postgres. The rows for a broadcaster are fetched with one query and held
// for brandingSnapshotTTL; writes through this node drop the snapshot at
// once, other nodes see them within the TTL.
const brandingSnapshotTTL = 2 * time.Second

type brandingSnapshot struct {
	at    time.Time
	blobs map[string]*BrandingBlob
}

// BrandingBlobs returns every branding asset of a broadcaster keyed by key,
// from the snapshot when it is fresh.
func (state *StatefulDB) BrandingBlobs(broadcasterID string) (map[string]*BrandingBlob, error) {
	if v, ok := state.brandingCache.Load(broadcasterID); ok {
		if snap := v.(*brandingSnapshot); time.Since(snap.at) < brandingSnapshotTTL {
			return snap.blobs, nil
		}
	}
	var rows []BrandingBlob
	if err := state.DB.Where("broadcaster_id = ?", broadcasterID).Find(&rows).Error; err != nil {
		return nil, err
	}
	blobs := make(map[string]*BrandingBlob, len(rows))
	for i := range rows {
		blobs[rows[i].Key] = &rows[i]
	}
	state.brandingCache.Store(broadcasterID, &brandingSnapshot{at: time.Now(), blobs: blobs})
	return blobs, nil
}

func (state *StatefulDB) forgetBranding(broadcasterID string) {
	state.brandingCache.Delete(broadcasterID)
}

// GetBrandingBlob fetches a single branding asset; gorm.ErrRecordNotFound
// when the broadcaster has no such key.
func (state *StatefulDB) GetBrandingBlob(broadcasterID, key string) (*BrandingBlob, error) {
	blobs, err := state.BrandingBlobs(broadcasterID)
	if err != nil {
		return nil, err
	}
	blob, ok := blobs[key]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return blob, nil
}

// PutBrandingBlob stores or updates a branding asset
func (state *StatefulDB) PutBrandingBlob(broadcasterID, key, mimeType string, data []byte, width, height *int) error {
	defer state.forgetBranding(broadcasterID)
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

// BrandingWrite is one key a bundle import sets.
type BrandingWrite struct {
	Key      string
	MimeType string
	Data     []byte
}

// ApplyBrandingWrites sets and removes branding keys in one transaction: a
// bundle import is all of its changes or none of them, never a node left
// with half the old branding and half the new.
func (state *StatefulDB) ApplyBrandingWrites(broadcasterID string, writes []BrandingWrite, removes []string) error {
	defer state.forgetBranding(broadcasterID)
	return state.DB.Transaction(func(tx *gorm.DB) error {
		for _, w := range writes {
			var existing BrandingBlob
			err := tx.Unscoped().Where("broadcaster_id = ? AND key = ?", broadcasterID, w.Key).First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Create(&BrandingBlob{BroadcasterID: broadcasterID, Key: w.Key, MimeType: w.MimeType, Data: w.Data}).Error; err != nil {
					return fmt.Errorf("write %s: %w", w.Key, err)
				}
			case err != nil:
				return fmt.Errorf("check %s: %w", w.Key, err)
			default:
				existing.MimeType = w.MimeType
				existing.Data = w.Data
				existing.Width, existing.Height = nil, nil
				existing.DeletedAt = gorm.DeletedAt{}
				if err := tx.Unscoped().Save(&existing).Error; err != nil {
					return fmt.Errorf("write %s: %w", w.Key, err)
				}
			}
		}
		for _, key := range removes {
			if err := tx.Where("broadcaster_id = ? AND key = ?", broadcasterID, key).Delete(&BrandingBlob{}).Error; err != nil {
				return fmt.Errorf("remove %s: %w", key, err)
			}
		}
		return nil
	})
}

// ListBrandingKeys returns all keys for a broadcaster
func (state *StatefulDB) ListBrandingKeys(broadcasterID string) ([]string, error) {
	blobs, err := state.BrandingBlobs(broadcasterID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(blobs))
	for key := range blobs {
		keys = append(keys, key)
	}
	return keys, nil
}

// DeleteBrandingBlob removes a specific asset
func (state *StatefulDB) DeleteBrandingBlob(broadcasterID, key string) error {
	defer state.forgetBranding(broadcasterID)
	err := state.DB.Where("broadcaster_id = ? AND key = ?", broadcasterID, key).Delete(&BrandingBlob{}).Error
	if err != nil {
		return fmt.Errorf("error deleting branding blob: %w", err)
	}
	return nil
}
