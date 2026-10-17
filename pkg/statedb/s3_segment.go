package statedb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"stream.place/streamplace/pkg/s3"
)

type S3Segment struct {
	ID      string `gorm:"column:id;primarykey"`
	RepoDID string `gorm:"column:user_did;index;not null"`
	// LivestreamURI ties this object to the place.stream.livestream it was
	// recorded for, so live-to-VOD finalize can enumerate exactly the objects
	// of one stream. May be empty for objects started before the URI was known.
	LivestreamURI string `gorm:"column:livestream_uri;index"`
	Bucket        string `gorm:"column:bucket;not null"`
	Key           string `gorm:"column:key;not null"`
	URL           string `gorm:"column:url"`
	// StartedAt is the media start time of the object's earliest segment.
	// Segments can reach the uploader slightly out of start-time order, so
	// it's the first one queued until RecordComplete lowers it to the
	// earliest in the index.
	StartedAt   time.Time  `gorm:"column:started_at"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	// Segments indexes the object's segments, so a clip can range-read just
	// the ones it needs. Set on completion; empty for objects recorded before
	// the index existed, which clips skip.
	Segments  []s3.RecordedSegment `gorm:"column:segments;type:json;serializer:json"`
	Size      int64                `gorm:"column:size"`
	PartCount int32                `gorm:"column:part_count"`
	CreatedAt time.Time            `gorm:"column:created_at"`
	UpdatedAt time.Time            `gorm:"column:updated_at"`
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
	return state.ListS3SegmentsForLivestreams(ctx, []string{livestreamURI})
}

// ListS3SegmentsForLivestreams is ListS3SegmentsForLivestream over several
// livestream records at once, in one chronological order: a streamer who
// started a new livestream record mid-stream (a title change on some
// clients) split one recording across two records, and the VOD wants the
// objects of both in the order they were recorded.
func (state *StatefulDB) ListS3SegmentsForLivestreams(ctx context.Context, livestreamURIs []string) ([]S3Segment, error) {
	if len(livestreamURIs) == 0 {
		return nil, nil
	}
	var segs []S3Segment
	err := state.DB.WithContext(ctx).
		Where("livestream_uri IN ? AND completed_at IS NOT NULL", livestreamURIs).
		Order("started_at ASC").
		Find(&segs).Error
	if err != nil {
		return nil, err
	}
	return segs, nil
}

// ListS3SegmentsInRange returns repoDID's completed objects that may hold
// segments starting in [from, to], in recording order. An object's segments
// all start between its StartedAt and its CompletedAt, so this is a superset;
// callers pick the exact segments out of each object's index.
func (state *StatefulDB) ListS3SegmentsInRange(ctx context.Context, repoDID string, from, to time.Time) ([]S3Segment, error) {
	var segs []S3Segment
	err := state.DB.WithContext(ctx).
		Where("user_did = ? AND completed_at IS NOT NULL AND started_at <= ? AND completed_at >= ?", repoDID, to, from).
		Order("started_at ASC").
		Find(&segs).Error
	return segs, err
}

// LatestOpenS3Segment returns repoDID's most recently started object whose
// upload has not completed and that started at or after since, or nil.
func (state *StatefulDB) LatestOpenS3Segment(ctx context.Context, repoDID string, since time.Time) (*S3Segment, error) {
	var segs []S3Segment
	err := state.DB.WithContext(ctx).
		Where("user_did = ? AND completed_at IS NULL AND started_at >= ?", repoDID, since).
		Order("started_at DESC").
		Limit(1).
		Find(&segs).Error
	if err != nil || len(segs) == 0 {
		return nil, err
	}
	return &segs[0], nil
}

// CountOpenS3Segments counts the objects recorded for one livestream whose
// uploads have not completed yet.
func (state *StatefulDB) CountOpenS3Segments(ctx context.Context, livestreamURI string) (int64, error) {
	var n int64
	err := state.DB.WithContext(ctx).Model(&S3Segment{}).
		Where("livestream_uri = ? AND completed_at IS NULL", livestreamURI).
		Count(&n).Error
	return n, err
}

// RecordComplete marks an S3 multipart upload as completed and records the
// final part count, size and segment index. Implements s3.Recorder.
func (state *StatefulDB) RecordComplete(ctx context.Context, id string, parts int32, size int64, segments []s3.RecordedSegment) error {
	now := time.Now().UTC()
	row := S3Segment{CompletedAt: &now, Size: size, PartCount: parts, Segments: segments}
	cols := []string{"completed_at", "size", "part_count", "segments"}
	if len(segments) > 0 {
		earliest := segments[0].StartMs
		for _, seg := range segments[1:] {
			earliest = min(earliest, seg.StartMs)
		}
		row.StartedAt = time.UnixMilli(earliest)
		cols = append(cols, "started_at")
	}
	return state.DB.WithContext(ctx).Model(&S3Segment{ID: id}).Select(cols).Updates(&row).Error
}

// RecordAbort removes the row of an aborted S3 multipart upload: the object
// never comes into existence, so nothing should wait on it or read from it.
// Implements s3.Recorder.
func (state *StatefulDB) RecordAbort(ctx context.Context, id string) error {
	return state.DB.WithContext(ctx).Delete(&S3Segment{ID: id}).Error
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
