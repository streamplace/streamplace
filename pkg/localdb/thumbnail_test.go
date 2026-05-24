package localdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestThumbnailCleaner(t *testing.T) {
	config.DisableSQLLogging()
	defer config.EnableSQLLogging()

	db, err := MakeDB(":memory:")
	require.NoError(t, err)
	ldb := db.(*LocalDatabase)

	const (
		userA = "did:plc:aaa"
		userB = "did:plc:bbb"
		userC = "did:plc:ccc"
	)
	base := time.Now().UTC()

	mkSeg := func(id, user string, startOffset time.Duration) {
		require.NoError(t, ldb.CreateSegment(&Segment{
			ID:        id,
			RepoDID:   user,
			StartTime: base.Add(startOffset),
			Published: true,
		}))
	}
	mkThumb := func(segID string) {
		require.NoError(t, ldb.CreateThumbnail(&Thumbnail{Format: "jpeg", SegmentID: segID}))
	}
	countThumbs := func() int64 {
		var n int64
		require.NoError(t, ldb.DB.Model(&Thumbnail{}).Count(&n).Error)
		return n
	}

	// userA: three segments, each with a thumbnail; a3 is the most recent.
	mkSeg("a1", userA, -3*time.Minute)
	mkThumb("a1")
	mkSeg("a2", userA, -2*time.Minute)
	mkThumb("a2")
	mkSeg("a3", userA, -1*time.Minute)
	mkThumb("a3")

	// userB: two segments with thumbnails; b2 is the most recent.
	mkSeg("b1", userB, -5*time.Minute)
	mkThumb("b1")
	mkSeg("b2", userB, -4*time.Minute)
	mkThumb("b2")

	// userC: has a segment but no thumbnail (must not break anything).
	mkSeg("c1", userC, -1*time.Minute)

	// An orphaned thumbnail whose segment was already cleaned up.
	mkThumb("ghost")

	require.EqualValues(t, 6, countThumbs())

	require.NoError(t, ldb.ThumbnailCleaner(context.Background()))

	// One thumbnail per user with thumbnails should remain; orphan is gone.
	require.EqualValues(t, 2, countThumbs())

	ta, err := ldb.LatestThumbnailForUser(userA)
	require.NoError(t, err)
	require.NotNil(t, ta)
	require.Equal(t, "a3", ta.SegmentID)

	tb, err := ldb.LatestThumbnailForUser(userB)
	require.NoError(t, err)
	require.NotNil(t, tb)
	require.Equal(t, "b2", tb.SegmentID)

	// Running again is a no-op.
	require.NoError(t, ldb.ThumbnailCleaner(context.Background()))
	require.EqualValues(t, 2, countThumbs())
}
