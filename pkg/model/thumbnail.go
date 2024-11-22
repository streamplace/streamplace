package model

import (
	"fmt"

	"github.com/google/uuid"
)

type Thumbnail struct {
	ID        string  `json:"id"                gorm:"primaryKey"`
	Format    string  `json:"format"`
	SegmentID string  `json:"segmentId"         gorm:"index"`
	Segment   Segment `json:"segment,omitempty" gorm:"foreignKey:SegmentID;references:id"`
}

func (m *DBModel) CreateThumbnail(thumb *Thumbnail) error {
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
func (m *DBModel) LatestThumbnailForUser(user string) (*Thumbnail, error) {
	var thumbnail Thumbnail

	res := m.DB.Table("thumbnails AS t").
		Select("t.*").
		Joins("JOIN segments AS s ON t.segment_id = s.id").
		Where("s.user = ?", user).
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
