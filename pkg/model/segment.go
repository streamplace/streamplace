package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/streamplace"
)

type SegmentMediadataVideo struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Framerate string `json:"framerate"`
}

type SegmentMediadataAudio struct {
	Rate     int `json:"rate"`
	Channels int `json:"channels"`
}

type SegmentMediaData struct {
	Video []*SegmentMediadataVideo `json:"video"`
	Audio []*SegmentMediadataAudio `json:"audio"`
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

func (s *Segment) ToStreamplaceSegment() *streamplace.Segment {
	aqt := aqtime.FromTime(s.StartTime)
	return &streamplace.Segment{
		LexiconTypeID: "place.stream.segment",
		Creator:       s.RepoDID,
		Id:            s.ID,
		SigningKey:    s.SigningKeyDID,
		StartTime:     string(aqt),
		Video: []*streamplace.Segment_Video{
			{
				Codec:  "h264",
				Width:  int64(s.MediaData.Video[0].Width),
				Height: int64(s.MediaData.Video[0].Height),
			},
		},
		Audio: []*streamplace.Segment_Audio{
			{
				Codec:    "opus",
				Rate:     int64(s.MediaData.Audio[0].Rate),
				Channels: int64(s.MediaData.Audio[0].Channels),
			},
		},
	}
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
