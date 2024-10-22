package model

import (
	"time"
)

type Segment struct {
	ID        string    `json:"id"        gorm:"primaryKey"`
	User      string    `json:"user"      gorm:"index:latest_segments"`
	StartTime time.Time `json:"startTime" gorm:"index:latest_segments"`
	EndTime   time.Time `json:"endTime"`
}

func (m *DBModel) CreateSegment(seg *Segment) error {
	err := m.DB.Model(Segment{}).Create(seg).Error
	if err != nil {
		return err
	}
	return nil
}

// should return the most recent segment for each user
func (m *DBModel) MostRecentSegments() ([]Segment, error) {
	var segments []Segment

	err := m.DB.Table("segments AS s1").
		Select("s1.*").
		Where("start_time = (SELECT MAX(start_time) FROM segments AS s2 WHERE s2.user = s1.user)").
		Scan(&segments).Error

	if err != nil {
		return nil, err
	}

	return segments, nil
}
