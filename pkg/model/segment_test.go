package model

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestSegmentPerf(t *testing.T) {
	config.DisableSQLLogging()
	// dburl := filepath.Join(t.TempDir(), "test.db")
	db, err := MakeDB(":memory:")
	require.NoError(t, err)
	// Create a model instance
	model := db.(*DBModel)
	t.Cleanup(func() {
		// os.Remove(dburl)
	})

	// Create a repo for testing
	repo := &Repo{
		DID: "did:plc:test123",
	}
	err = model.DB.Create(repo).Error
	require.NoError(t, err)

	defer config.EnableSQLLogging()
	// Create 250000 segments with timestamps 1 hour ago, each one second apart
	wg := sync.WaitGroup{}
	segCount := 250000
	wg.Add(segCount)
	baseTime := time.Now()
	for i := 0; i < segCount; i++ {
		segment := &Segment{
			ID:        fmt.Sprintf("segment-%d", i),
			RepoDID:   repo.DID,
			StartTime: baseTime.Add(-time.Duration(i) * time.Second).UTC(),
		}
		go func() {
			defer wg.Done()
			err = model.DB.Create(segment).Error
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	startTime := time.Now()
	wg = sync.WaitGroup{}
	runs := 1000
	wg.Add(runs)
	for i := 0; i < runs; i++ {
		go func() {
			defer wg.Done()
			_, err := model.MostRecentSegments()
			require.NoError(t, err)
			// require.Len(t, segments, 1)
		}()
	}
	wg.Wait()
	fmt.Printf("Time taken: %s\n", time.Since(startTime))
	require.Less(t, time.Since(startTime), 10*time.Second)
}
