package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// MediaTrack is the indexed view of a place.stream.media.track record.
// One record per A/V track within a MUXL container; multiple tracks
// share a Blob (the BDASL CID of the container they live in).
//
// Blob is indexed because "list all tracks for this blob" is the
// canonical playback query — having an Origin pointing at a blob
// tells you it's available; having Tracks for that blob tells you
// what's actually in it.
type MediaTrack struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did;index"`
	Repo      *Repo     `gorm:"foreignKey:DID;references:RepoDID"`
	RKey      string    `gorm:"column:rkey"`
	Blob      string    `gorm:"column:blob;index"`
	TrackID   string    `gorm:"column:track_id"`
	MediaType string    `gorm:"column:media_type;index"`
	Language  *string   `gorm:"column:language"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

func (m *DBModel) UpsertMediaTrack(ctx context.Context, t *MediaTrack) error {
	return m.DB.WithContext(ctx).Save(t).Error
}

func (m *DBModel) DeleteMediaTrack(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&MediaTrack{}).Error
}

func (m *DBModel) GetMediaTrackByURI(ctx context.Context, uri string) (*MediaTrack, error) {
	var t MediaTrack
	err := m.DB.WithContext(ctx).Preload("Repo").Where("uri = ?", uri).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get media track by uri: %w", err)
	}
	return &t, nil
}

// GetMediaTracksByBlob returns every track record that claims to live
// inside the given MUXL container, across all repos. Playback uses
// this to assemble the per-track manifest set for an HLS playlist.
func (m *DBModel) GetMediaTracksByBlob(ctx context.Context, blob string) ([]*MediaTrack, error) {
	var out []*MediaTrack
	err := m.DB.WithContext(ctx).
		Preload("Repo").
		Where("blob = ?", blob).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list tracks for blob: %w", err)
	}
	return out, nil
}
