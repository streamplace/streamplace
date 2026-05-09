package statedb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type S3Segment struct {
	ID          string     `gorm:"column:id;primarykey"`
	RepoDID     string     `gorm:"column:user_did;index;not null"`
	Bucket      string     `gorm:"column:bucket;not null"`
	Key         string     `gorm:"column:key;not null"`
	URL         string     `gorm:"column:url"`
	StartedAt   time.Time  `gorm:"column:started_at"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	Size        int64      `gorm:"column:size"`
	PartCount   int32      `gorm:"column:part_count"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (s *S3Segment) TableName() string {
	return "s3_segments"
}

// RecordStart inserts a new S3Segment row at the start of a multipart upload
// and returns its ID. Implements s3.Recorder.
func (state *StatefulDB) RecordStart(ctx context.Context, repoDID, bucket, key string, started time.Time) (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	seg := &S3Segment{
		ID:        uu.String(),
		RepoDID:   repoDID,
		Bucket:    bucket,
		Key:       key,
		StartedAt: started,
	}
	if err := state.DB.WithContext(ctx).Create(seg).Error; err != nil {
		return "", err
	}
	return seg.ID, nil
}

// RecordComplete marks an S3 multipart upload as completed and records the
// final part count and size. Implements s3.Recorder.
func (state *StatefulDB) RecordComplete(ctx context.Context, id string, parts int32, size int64) error {
	now := time.Now().UTC()
	return state.DB.WithContext(ctx).Model(&S3Segment{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"completed_at": &now,
			"size":         size,
			"part_count":   parts,
		}).Error
}

// GetS3Segment fetches an S3Segment by ID. Returns (nil, nil) if not found.
func (state *StatefulDB) GetS3Segment(ctx context.Context, id string) (*S3Segment, error) {
	var seg S3Segment
	err := state.DB.WithContext(ctx).Where("id = ?", id).First(&seg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &seg, nil
}
