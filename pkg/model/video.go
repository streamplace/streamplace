package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Video is the indexed view of a place.stream.video record. Indexing
// happens via the firehose for any user repo we're observing.
//
// The Record column holds the CBOR-encoded record bytes; everything
// else is a denormalized projection used to make playback queries
// (by author, recency, etc.) cheap. Title and Duration are surfaced
// since they're tiny and ubiquitously needed by listing UIs.
type Video struct {
	URI        string    `gorm:"primaryKey;column:uri"`
	CID        string    `gorm:"column:cid"`
	RepoDID    string    `gorm:"column:repo_did;index:idx_videos_repo_created,priority:1"`
	Repo       *Repo     `gorm:"foreignKey:DID;references:RepoDID"`
	RKey       string    `gorm:"column:rkey"`
	Title      string    `gorm:"column:title"`
	DurationMS *int64    `gorm:"column:duration_ms"`
	Record     []byte    `gorm:"column:record"`
	IndexedAt  time.Time `gorm:"column:indexed_at;index:idx_videos_repo_created,priority:2"`
}

func (m *DBModel) UpsertVideo(ctx context.Context, v *Video) error {
	return m.DB.WithContext(ctx).Save(v).Error
}

func (m *DBModel) DeleteVideo(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&Video{}).Error
}

func (m *DBModel) GetVideoByURI(ctx context.Context, uri string) (*Video, error) {
	var v Video
	err := m.DB.WithContext(ctx).Preload("Repo").Where("uri = ?", uri).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get video by uri: %w", err)
	}
	return &v, nil
}

// GetLatestVideosForRepo returns the most recent N videos published
// by the given user. Used to populate per-streamer VOD feeds.
func (m *DBModel) GetLatestVideosForRepo(ctx context.Context, repoDID string, limit int) ([]*Video, error) {
	if limit <= 0 {
		limit = 25
	}
	var out []*Video
	err := m.DB.WithContext(ctx).
		Preload("Repo").
		Where("repo_did = ?", repoDID).
		Order("indexed_at DESC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list videos for repo: %w", err)
	}
	return out, nil
}
