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
	Width   int  `json:"width"`
	Height  int  `json:"height"`
	FPSNum  int  `json:"fpsNum"`
	FPSDen  int  `json:"fpsDen"`
	BFrames bool `json:"bframes"`
}

type SegmentMediadataAudio struct {
	Rate     int `json:"rate"`
	Channels int `json:"channels"`
}

type SegmentMediaData struct {
	Video    []*SegmentMediadataVideo `json:"video"`
	Audio    []*SegmentMediadataAudio `json:"audio"`
	Duration int64                    `json:"duration"`
	Size     int                      `json:"size"`
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (j *SegmentMediaData) Scan(value any) error {
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

// ContentRights represents content rights and attribution information
type ContentRights struct {
	CopyrightNotice *string `json:"copyrightNotice,omitempty"`
	CopyrightYear   *int64  `json:"copyrightYear,omitempty"`
	Creator         *string `json:"creator,omitempty"`
	CreditLine      *string `json:"creditLine,omitempty"`
	License         *string `json:"license,omitempty"`
}

// Scan scan value into ContentRights, implements sql.Scanner interface
func (c *ContentRights) Scan(value any) error {
	if value == nil {
		*c = ContentRights{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal ContentRights value:", value))
	}

	result := ContentRights{}
	err := json.Unmarshal(bytes, &result)
	*c = ContentRights(result)
	return err
}

// Value return json value, implement driver.Valuer interface
func (c ContentRights) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// DistributionPolicy represents distribution policy information
type DistributionPolicy struct {
	DeleteAfterSeconds *int64 `json:"deleteAfterSeconds,omitempty"`
}

// Scan scan value into DistributionPolicy, implements sql.Scanner interface
func (d *DistributionPolicy) Scan(value any) error {
	if value == nil {
		*d = DistributionPolicy{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal DistributionPolicy value:", value))
	}

	result := DistributionPolicy{}
	err := json.Unmarshal(bytes, &result)
	*d = DistributionPolicy(result)
	return err
}

// Value return json value, implement driver.Valuer interface
func (d DistributionPolicy) Value() (driver.Value, error) {
	return json.Marshal(d)
}

// ContentWarningsSlice is a custom type for storing content warnings as JSON in the database
type ContentWarningsSlice []string

// Scan scan value into ContentWarningsSlice, implements sql.Scanner interface
func (c *ContentWarningsSlice) Scan(value any) error {
	if value == nil {
		*c = ContentWarningsSlice{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal ContentWarningsSlice value:", value))
	}

	result := ContentWarningsSlice{}
	err := json.Unmarshal(bytes, &result)
	*c = ContentWarningsSlice(result)
	return err
}

// Value return json value, implement driver.Valuer interface
func (c ContentWarningsSlice) Value() (driver.Value, error) {
	return json.Marshal(c)
}

type Segment struct {
	ID                 string               `json:"id"                   gorm:"primaryKey"`
	SigningKeyDID      string               `json:"signingKeyDID"        gorm:"column:signing_key_did"`
	SigningKey         *SigningKey          `json:"signingKey,omitempty" gorm:"foreignKey:DID;references:SigningKeyDID"`
	StartTime          time.Time            `json:"startTime"            gorm:"index:latest_segments,priority:2;index:start_time"`
	RepoDID            string               `json:"repoDID"              gorm:"index:latest_segments,priority:1;column:repo_did"`
	Repo               *Repo                `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	Title              string               `json:"title"`
	Size               int                  `json:"size"                gorm:"column:size"`
	MediaData          *SegmentMediaData    `json:"mediaData,omitempty"`
	ContentWarnings    ContentWarningsSlice `json:"contentWarnings,omitempty"`
	ContentRights      *ContentRights       `json:"contentRights,omitempty"`
	DistributionPolicy *DistributionPolicy  `json:"distributionPolicy,omitempty"`
	DeleteAfter        *time.Time           `json:"deleteAfter,omitempty" gorm:"column:delete_after;index:delete_after"`
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
	sizei64 := int64(s.Size)

	// Convert model metadata to streamplace metadata
	var contentRights *streamplace.MetadataContentRights
	if s.ContentRights != nil {
		contentRights = &streamplace.MetadataContentRights{
			CopyrightNotice: s.ContentRights.CopyrightNotice,
			CopyrightYear:   s.ContentRights.CopyrightYear,
			Creator:         s.ContentRights.Creator,
			CreditLine:      s.ContentRights.CreditLine,
			License:         s.ContentRights.License,
		}
	}

	var contentWarnings *streamplace.MetadataContentWarnings
	if len(s.ContentWarnings) > 0 {
		contentWarnings = &streamplace.MetadataContentWarnings{
			Warnings: []string(s.ContentWarnings),
		}
	}

	var distributionPolicy *streamplace.MetadataDistributionPolicy
	if s.DistributionPolicy != nil && s.DistributionPolicy.DeleteAfterSeconds != nil {
		distributionPolicy = &streamplace.MetadataDistributionPolicy{
			DeleteAfter: s.DistributionPolicy.DeleteAfterSeconds,
		}
	}

	return &streamplace.Segment{
		LexiconTypeID:      "place.stream.segment",
		Creator:            s.RepoDID,
		Id:                 s.ID,
		SigningKey:         s.SigningKeyDID,
		StartTime:          string(aqt),
		Duration:           &duration,
		Size:               &sizei64,
		ContentRights:      contentRights,
		ContentWarnings:    contentWarnings,
		DistributionPolicy: distributionPolicy,
		Video: []*streamplace.Segment_Video{
			{
				Codec:  "h264",
				Width:  int64(s.MediaData.Video[0].Width),
				Height: int64(s.MediaData.Video[0].Height),
				Framerate: &streamplace.Segment_Framerate{
					Num: int64(s.MediaData.Video[0].FPSNum),
					Den: int64(s.MediaData.Video[0].FPSDen),
				},
				Bframes: &s.MediaData.Video[0].BFrames,
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
		Where("start_time > ?", thirtySecondsAgo.UTC()).
		Order("start_time DESC").
		Find(&segments).Error
	if err != nil {
		return nil, err
	}
	if segments == nil {
		return []Segment{}, nil
	}

	segmentMap := make(map[string]Segment)
	for _, seg := range segments {
		prev, ok := segmentMap[seg.RepoDID]
		if !ok {
			segmentMap[seg.RepoDID] = seg
		} else {
			if seg.StartTime.After(prev.StartTime) {
				segmentMap[seg.RepoDID] = seg
			}
		}
	}

	filteredSegments := []Segment{}
	for _, seg := range segmentMap {
		filteredSegments = append(filteredSegments, seg)
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

func (m *DBModel) FilterLiveRepoDIDs(repoDIDs []string) ([]string, error) {
	if len(repoDIDs) == 0 {
		return []string{}, nil
	}

	thirtySecondsAgo := time.Now().Add(-30 * time.Second)

	var liveDIDs []string

	err := m.DB.Table("segments").
		Select("DISTINCT repo_did").
		Where("repo_did IN ? AND start_time > ?", repoDIDs, thirtySecondsAgo.UTC()).
		Pluck("repo_did", &liveDIDs).Error

	if err != nil {
		return nil, err
	}

	return liveDIDs, nil
}

func (m *DBModel) LatestSegmentsForUser(user string, limit int, before *time.Time, after *time.Time) ([]Segment, error) {
	var segs []Segment
	if before == nil {
		later := time.Now().Add(1000 * time.Hour)
		before = &later
	}
	if after == nil {
		earlier := time.Time{}
		after = &earlier
	}
	err := m.DB.Model(Segment{}).Where("repo_did = ? AND start_time < ? AND start_time > ?", user, before.UTC(), after.UTC()).Order("start_time DESC").Limit(limit).Find(&segs).Error
	if err != nil {
		return nil, err
	}
	return segs, nil
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

func (m *DBModel) GetExpiredSegments(ctx context.Context) ([]Segment, error) {

	var expiredSegments []Segment
	now := time.Now()
	err := m.DB.
		Where("delete_after IS NOT NULL AND delete_after < ?", now.UTC()).
		Find(&expiredSegments).Error
	if err != nil {
		return nil, err
	}

	return expiredSegments, nil
}

func (m *DBModel) DeleteSegment(ctx context.Context, id string) error {
	return m.DB.Delete(&Segment{}, "id = ?", id).Error
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
