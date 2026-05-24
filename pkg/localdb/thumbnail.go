package localdb

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"stream.place/streamplace/pkg/log"
)

type Thumbnail struct {
	ID        string  `json:"id"                gorm:"primaryKey"`
	Format    string  `json:"format"`
	SegmentID string  `json:"segmentId"         gorm:"index"`
	Segment   Segment `json:"segment,omitempty" gorm:"foreignKey:SegmentID;references:id"`
}

func (m *LocalDatabase) CreateThumbnail(thumb *Thumbnail) error {
	uu, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if thumb.SegmentID == "" {
		return fmt.Errorf("segmentID is required")
	}
	thumb.ID = uu.String()
	err = m.DB.Model(Thumbnail{}).Create(thumb).Error
	if err != nil {
		return err
	}
	return nil
}

// return the most recent thumbnail for a user
func (m *LocalDatabase) LatestThumbnailForUser(user string) (*Thumbnail, error) {
	var thumbnail Thumbnail

	res := m.DB.Table("thumbnails AS t").
		Select("t.*").
		Joins("JOIN segments AS s ON t.segment_id = s.id").
		Where("s.repo_did = ?", user).
		Order("s.start_time DESC").
		Limit(1).
		Scan(&thumbnail)

	if res.RowsAffected == 0 {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}

	var seg Segment
	err := m.DB.First(&seg, "id = ?", thumbnail.SegmentID).Error
	if err != nil {
		return nil, fmt.Errorf("could not find segment for thumbnail SegmentID=%s", thumbnail.SegmentID)
	}

	thumbnail.Segment = seg

	return &thumbnail, nil
}

// ThumbnailCleaner keeps only the most recent thumbnail for each user and
// deletes the rest. Thumbnails are created roughly once a minute per active
// user but are never removed when their segment is cleaned up, so the table
// grows without bound and bloats the database. Only the latest thumbnail per
// user is ever read (see LatestThumbnailForUser), so the older ones are dead
// weight.
func (m *LocalDatabase) ThumbnailCleaner(ctx context.Context) error {
	var cleaned int64

	// Drop any thumbnail whose segment has already been deleted. These are
	// orphans we can never serve and can't even associate with a user anymore.
	orphans := m.DB.
		Where("segment_id NOT IN (?)", m.DB.Model(&Segment{}).Select("id")).
		Delete(&Thumbnail{})
	if orphans.Error != nil {
		log.Error(ctx, "Failed to clean orphaned thumbnails", "error", orphans.Error)
		return orphans.Error
	}
	cleaned += orphans.RowsAffected

	// Find all unique repo_did values.
	var repoDIDs []string
	if err := m.DB.Model(&Segment{}).Distinct("repo_did").Pluck("repo_did", &repoDIDs).Error; err != nil {
		log.Error(ctx, "Failed to get unique repo_dids for thumbnail cleaning", "error", err)
		return err
	}

	// For each user, keep the thumbnail on their most recent segment (matching
	// LatestThumbnailForUser) and delete every other thumbnail of theirs.
	for _, repoDID := range repoDIDs {
		var keepIDs []string
		if err := m.DB.Table("thumbnails AS t").
			Joins("JOIN segments AS s ON t.segment_id = s.id").
			Where("s.repo_did = ?", repoDID).
			Order("s.start_time DESC").
			Limit(1).
			Pluck("t.id", &keepIDs).Error; err != nil {
			log.Error(ctx, "Failed to get thumbnail to keep", "repo_did", repoDID, "error", err)
			return err
		}
		if len(keepIDs) == 0 {
			continue
		}

		result := m.DB.
			Where("segment_id IN (?) AND id NOT IN ?",
				m.DB.Model(&Segment{}).Select("id").Where("repo_did = ?", repoDID),
				keepIDs).
			Delete(&Thumbnail{})
		if result.Error != nil {
			log.Error(ctx, "Failed to clean old thumbnails", "repo_did", repoDID, "error", result.Error)
			return result.Error
		}
		cleaned += result.RowsAffected
	}

	if cleaned > 0 {
		log.Log(ctx, "Cleaned old thumbnails", "count", cleaned)
	}
	return nil
}
