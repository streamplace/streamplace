package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSegmentCleaner(t *testing.T) {
	db, err := MakeDB(":memory:")
	require.NoError(t, err)
	// Create a model instance
	model := db.(*DBModel)

	// Create a repo for testing
	repo := &Repo{
		DID: "did:plc:test123",
	}
	err = model.DB.Create(repo).Error
	require.NoError(t, err)

	// Create 100 segments with timestamps 1 hour ago, each one second apart
	baseTime := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 100; i++ {
		segment := &Segment{
			ID:        fmt.Sprintf("segment-%d", i),
			RepoDID:   repo.DID,
			StartTime: baseTime.Add(time.Duration(i) * time.Second),
		}
		err = model.DB.Create(segment).Error
		require.NoError(t, err)
	}

	// Verify we have 100 segments
	var count int64
	err = model.DB.Model(&Segment{}).Count(&count).Error
	require.NoError(t, err)
	require.Equal(t, int64(100), count)

	// Run the segment cleaner
	err = model.SegmentCleaner(context.Background())
	require.NoError(t, err)

	// Verify we now have only 10 segments
	err = model.DB.Model(&Segment{}).Count(&count).Error
	require.NoError(t, err)
	require.Equal(t, int64(10), count)

	// Verify the remaining segments are the most recent ones
	var segments []Segment
	err = model.DB.Model(&Segment{}).Order("start_time DESC").Find(&segments).Error
	require.NoError(t, err)
	require.Len(t, segments, 10)

	// The segments should be the last 10 we created (the most recent ones)
	for i, segment := range segments {
		expectedTime := baseTime.Add(time.Duration(99-i) * time.Second)
		// Allow a small tolerance for time comparison
		require.WithinDuration(t, expectedTime, segment.StartTime, time.Second)
	}
}
