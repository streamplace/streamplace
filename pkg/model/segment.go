package model

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

type SegmentMediadataVideo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPSNum int `json:"fpsNum"`
	FPSDen int `json:"fpsDen"`
}

type SegmentMediadataAudio struct {
	Rate     int `json:"rate"`
	Channels int `json:"channels"`
}

type SegmentMediaData struct {
	Video    []*SegmentMediadataVideo `json:"video"`
	Audio    []*SegmentMediadataAudio `json:"audio"`
	Duration int64                    `json:"duration"`
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (j *SegmentMediaData) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := SegmentMediaData{}
	err := json.Unmarshal(bytes, &result)
	*j = SegmentMediaData(result)
	return err
}

// Value return json value, implement driver.Valuer interface
func (j SegmentMediaData) Value() (driver.Value, error) {
	return json.Marshal(j)
}

type Segment struct {
	ID            string            `json:"id"                   gorm:"primaryKey"`
	SigningKeyDID string            `json:"signingKeyDID"        gorm:"column:signing_key_did"`
	SigningKey    *SigningKey       `json:"signingKey,omitempty" gorm:"foreignKey:DID;references:SigningKeyDID"`
	StartTime     time.Time         `json:"startTime"            gorm:"index:latest_segments"`
	RepoDID       string            `json:"repoDID"              gorm:"index:latest_segments;column:repo_did"`
	Repo          *Repo             `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	Title         string            `json:"title"`
	MediaData     *SegmentMediaData `json:"mediaData,omitempty"`
}

func (s *Segment) ToStreamplaceSegment() (*streamplace.Segment, error) {
	aqt := aqtime.FromTime(s.StartTime)
	if s.MediaData == nil {
		return nil, fmt.Errorf("media data is nil")
	}
	if len(s.MediaData.Video) == 0 || s.MediaData.Video[0] == nil {
		return nil, fmt.Errorf("video data is nil")
	}
	if len(s.MediaData.Audio) == 0 || s.MediaData.Audio[0] == nil {
		return nil, fmt.Errorf("audio data is nil")
	}
	duration := s.MediaData.Duration
	return &streamplace.Segment{
		LexiconTypeID: "place.stream.segment",
		Creator:       s.RepoDID,
		Id:            s.ID,
		SigningKey:    s.SigningKeyDID,
		StartTime:     string(aqt),
		Duration:      &duration,
		Video: []*streamplace.Segment_Video{
			{
				Codec:  "h264",
				Width:  int64(s.MediaData.Video[0].Width),
				Height: int64(s.MediaData.Video[0].Height),
				Framerate: &streamplace.Segment_Framerate{
					Num: int64(s.MediaData.Video[0].FPSNum),
					Den: int64(s.MediaData.Video[0].FPSDen),
				},
			},
		},
		Audio: []*streamplace.Segment_Audio{
			{
				Codec:    "opus",
				Rate:     int64(s.MediaData.Audio[0].Rate),
				Channels: int64(s.MediaData.Audio[0].Channels),
			},
		},
	}, nil
}

func (m *DBModel) CreateSegment(seg *Segment) error {
	err := m.DB.Model(Segment{}).Create(seg).Error
	if err != nil {
		return err
	}
	return nil
}

// should return the most recent segment for each user, ordered by most recent first
// only includes segments from the last 30 seconds
func (m *DBModel) MostRecentSegments() ([]Segment, error) {
	var segments []Segment
	thirtySecondsAgo := time.Now().Add(-30 * time.Second)

	err := m.DB.Table("segments").
		Select("segments.*").
		Where("id IN (?)",
			m.DB.Table("segments").
				Select("id").
				Where("(repo_did, start_time) IN (?)",
					m.DB.Table("segments").
						Select("repo_did, MAX(start_time)").
						Group("repo_did"))).
		Order("start_time DESC").
		Joins("JOIN repos ON segments.repo_did = repos.did").
		Preload("Repo").
		Find(&segments).Error

	if err != nil {
		return nil, err
	}
	if segments == nil {
		return []Segment{}, nil
	}

	filteredSegments := []Segment{}
	for _, seg := range segments {
		if seg.StartTime.After(thirtySecondsAgo) {
			filteredSegments = append(filteredSegments, seg)
		}
	}

	return filteredSegments, nil
}

func (m *DBModel) LatestSegmentForUser(user string) (*Segment, error) {
	var seg Segment
	err := m.DB.Model(Segment{}).Where("repo_did = ?", user).Order("start_time DESC").First(&seg).Error
	if err != nil {
		return nil, err
	}
	return &seg, nil
}

func (m *DBModel) GetLiveUsers() ([]Segment, error) {
	var liveUsers []Segment
	thirtySecondsAgo := aqtime.FromTime(time.Now().Add(-30 * time.Second)).Time()

	err := m.DB.Model(&Segment{}).
		Preload("Repo").
		Where("start_time >= ?", thirtySecondsAgo).
		Where("start_time = (SELECT MAX(start_time) FROM segments s2 WHERE s2.repo_did = segments.repo_did)").
		Order("start_time DESC").
		Find(&liveUsers).Error

	if err != nil {
		return nil, err
	}
	if liveUsers == nil {
		return []Segment{}, nil
	}

	return liveUsers, nil
}

func (m *DBModel) GetSegment(id string) (*Segment, error) {
	var seg Segment

	err := m.DB.Model(&Segment{}).
		Preload("Repo").
		Where("id = ?", id).
		First(&seg).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &seg, nil
}

func (m *DBModel) StartSegmentCleaner(ctx context.Context) error {
	err := m.SegmentCleaner(ctx)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := m.SegmentCleaner(ctx)
			if err != nil {
				log.Error(ctx, "Failed to clean segments", "error", err)
			}
		}
	}
}

func (m *DBModel) SegmentCleaner(ctx context.Context) error {
	// Calculate the cutoff time (10 minutes ago)
	cutoffTime := aqtime.FromTime(time.Now().Add(-10 * time.Minute)).Time()

	// Find all unique repo_did values
	var repoDIDs []string
	if err := m.DB.Model(&Segment{}).Distinct("repo_did").Pluck("repo_did", &repoDIDs).Error; err != nil {
		log.Error(ctx, "Failed to get unique repo_dids for segment cleaning", "error", err)
		return err
	}

	// For each user, keep their last 10 segments and delete older ones
	for _, repoDID := range repoDIDs {
		// Get IDs of the last 10 segments for this user
		var keepSegmentIDs []string
		if err := m.DB.Model(&Segment{}).
			Where("repo_did = ?", repoDID).
			Order("start_time DESC").
			Limit(10).
			Pluck("id", &keepSegmentIDs).Error; err != nil {
			log.Error(ctx, "Failed to get segment IDs to keep", "repo_did", repoDID, "error", err)
			return err
		}

		// Delete old segments except the ones we want to keep
		result := m.DB.Where("repo_did = ? AND start_time < ? AND id NOT IN ?",
			repoDID, cutoffTime, keepSegmentIDs).Delete(&Segment{})

		if result.Error != nil {
			log.Error(ctx, "Failed to clean old segments", "repo_did", repoDID, "error", result.Error)
		} else if result.RowsAffected > 0 {
			log.Log(ctx, "Cleaned old segments", "repo_did", repoDID, "count", result.RowsAffected)
		}
	}
	return nil
}
