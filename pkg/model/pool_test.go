package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIndexDBPool pins the connection-pool arrangement that lets a boot-time
// reindex run without queueing every read behind it: the pragmas must hold on
// pooled connections (they ride in the DSN -- an Exec would only configure one
// connection), and concurrent readers and writers across the pool must never
// surface SQLITE_BUSY.
func TestIndexDBPool(t *testing.T) {
	m, err := MakeDB(t.TempDir())
	require.NoError(t, err)
	db := m.(*DBModel).DB

	var mode string
	require.NoError(t, db.Raw("PRAGMA journal_mode;").Scan(&mode).Error)
	require.Equal(t, "wal", mode)
	var synchronous int
	require.NoError(t, db.Raw("PRAGMA synchronous;").Scan(&synchronous).Error)
	require.Equal(t, 1, synchronous, "NORMAL")
	var timeout int
	require.NoError(t, db.Raw("PRAGMA busy_timeout;").Scan(&timeout).Error)
	require.Equal(t, int(SQLiteBusyTimeout.Milliseconds()), timeout)

	var wg sync.WaitGroup
	errs := make(chan error, IndexDBPoolSize*2)
	for i := 0; i < IndexDBPoolSize*2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				did := fmt.Sprintf("did:plc:concurrent%02d-%03d", i, j)
				if err := m.UpdateRepo(&Repo{DID: did, Version: "rev"}); err != nil {
					errs <- fmt.Errorf("writer %d: %w", i, err)
					return
				}
				if _, err := m.GetRepo(did); err != nil {
					errs <- fmt.Errorf("reader %d: %w", i, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
