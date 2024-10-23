package model

import (
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
	thumb.ID = uu.String()
	err = m.DB.Model(Thumbnail{}).Create(thumb).Error
	if err != nil {
		return err
	}
	return nil
}

// return the most recent thumbnail for a user
func (m *DBModel) LatestThumbnailForUser(user string) (Thumbnail, error) {
	var thumbnail Thumbnail

	err := m.DB.Table("thumbnails AS t").
		Select("t.*").
		Joins("JOIN segments AS s ON t.segment_id = s.id").
		Where("s.user = ?", user).
		Order("s.start_time DESC").
		Limit(1).
		Scan(&thumbnail).Error
	if err != nil {
		return Thumbnail{}, err
	}

	var seg Segment
	err = m.DB.First(&seg, "id = ?", thumbnail.SegmentID).Error
	if err != nil {
		return Thumbnail{}, err
	}

	thumbnail.Segment = seg

	return thumbnail, nil
}
