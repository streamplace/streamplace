package localdb

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
	// Create a ldb instance
	ldb := db.(*LocalDatabase)
	t.Cleanup(func() {
		// os.Remove(dburl)
	})

	defer config.EnableSQLLogging()
	// Create 250000 segments with timestamps 1 hour ago, each one second apart
	wg := sync.WaitGroup{}
	segCount := 250000
	wg.Add(segCount)
	baseTime := time.Now()
	for i := 0; i < segCount; i++ {
		segment := &Segment{
			ID:        fmt.Sprintf("segment-%d", i),
			RepoDID:   "did:plc:test123",
			StartTime: baseTime.Add(-time.Duration(i) * time.Second).UTC(),
		}
		go func() {
			defer wg.Done()
			err = ldb.DB.Create(segment).Error
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
			_, err := ldb.MostRecentSegments()
			require.NoError(t, err)
			// require.Len(t, segments, 1)
		}()
	}
	wg.Wait()
	fmt.Printf("Time taken: %s\n", time.Since(startTime))
	require.Less(t, time.Since(startTime), 10*time.Second)
}
