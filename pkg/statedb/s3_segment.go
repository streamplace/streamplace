package statedb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type S3Segment struct {
	ID      string `gorm:"column:id;primarykey"`
	RepoDID string `gorm:"column:user_did;index;not null"`
	// LivestreamURI ties this object to the place.stream.livestream it was
	// recorded for, so live-to-VOD finalize can enumerate exactly the objects
	// of one stream. May be empty for objects started before the URI was known.
	LivestreamURI string     `gorm:"column:livestream_uri;index"`
	Bucket        string     `gorm:"column:bucket;not null"`
	Key           string     `gorm:"column:key;not null"`
	URL           string     `gorm:"column:url"`
	StartedAt     time.Time  `gorm:"column:started_at"`
	CompletedAt   *time.Time `gorm:"column:completed_at"`
	Size          int64      `gorm:"column:size"`
	PartCount     int32      `gorm:"column:part_count"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (s *S3Segment) TableName() string {
	return "s3_segments"
}

// RecordStart inserts a new S3Segment row at the start of a multipart upload
// and returns its ID. Implements s3.Recorder.
func (state *StatefulDB) RecordStart(ctx context.Context, repoDID, bucket, key, livestreamURI string, started time.Time) (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	seg := &S3Segment{
		ID:            uu.String(),
		RepoDID:       repoDID,
		LivestreamURI: livestreamURI,
		Bucket:        bucket,
		Key:           key,
		StartedAt:     started,
	}
	if err := state.DB.WithContext(ctx).Create(seg).Error; err != nil {
		return "", err
	}
	return seg.ID, nil
}

// ListS3SegmentsForLivestream returns the completed S3 objects recorded for one
// livestream, ordered by StartedAt — i.e. the chronological byte order the
// objects must be concatenated in to reconstruct the stream. In-progress
// objects (CompletedAt == nil) are excluded; finalize runs after teardown, by
// which point every object of a finished stream has completed.
func (state *StatefulDB) ListS3SegmentsForLivestream(ctx context.Context, livestreamURI string) ([]S3Segment, error) {
	var segs []S3Segment
	err := state.DB.WithContext(ctx).
		Where("livestream_uri = ? AND completed_at IS NOT NULL", livestreamURI).
		Order("started_at ASC").
		Find(&segs).Error
	if err != nil {
		return nil, err
	}
	return segs, nil
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
