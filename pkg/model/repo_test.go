package model

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fullRepo is a row with something in every column the sync engine cares
// about, so that a test asserting "only this column moved" means it.
func fullRepo(did string) *Repo {
	return &Repo{
		DID:           did,
		Handle:        "someone.example",
		PDS:           "https://pds.example",
		Version:       "3lprev0000000",
		RootCID:       "bafyreiabc",
		BackfillFloor: "3lpfloor00000",
		BackfillDone:  true,
	}
}

// TestAdvanceRepoVersion is the compare-and-swap the firehose's contiguity
// check rests on: it moves the rev only from the value the caller saw, and it
// says whether it did.
func TestAdvanceRepoVersion(t *testing.T) {
	db := indexedTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpdateRepo(fullRepo("did:plc:a")))

	// The rev this event follows is not ours: nothing happens, and the caller
	// is told so rather than left to assume.
	applied, err := db.AdvanceRepoVersion(ctx, "did:plc:a", "3lpsomethingelse", "3lpnext000000")
	require.NoError(t, err)
	require.False(t, applied)
	got, err := db.GetRepo("did:plc:a")
	require.NoError(t, err)
	require.Equal(t, "3lprev0000000", got.Version)

	applied, err = db.AdvanceRepoVersion(ctx, "did:plc:a", "3lprev0000000", "3lpnext000000")
	require.NoError(t, err)
	require.True(t, applied)

	// Only the rev moved. Everything else is the sync state a repair would
	// otherwise have to rebuild.
	got, err = db.GetRepo("did:plc:a")
	require.NoError(t, err)
	require.Equal(t, "3lpnext000000", got.Version)
	require.Equal(t, "bafyreiabc", got.RootCID, "root_c_id is not a column to lose")
	require.Equal(t, "3lpfloor00000", got.BackfillFloor)
	require.True(t, got.BackfillDone)
	require.Equal(t, "someone.example", got.Handle)

	// The same event again -- a redelivery from a second relay -- is a no-op.
	applied, err = db.AdvanceRepoVersion(ctx, "did:plc:a", "3lprev0000000", "3lpnext000000")
	require.NoError(t, err)
	require.False(t, applied)

	// An empty from is the wedge that means "this repo needs a backfill".
	// Filling it in from an event would cancel a repair nobody has done.
	require.NoError(t, db.UpdateRepo(&Repo{DID: "did:plc:wedged", Version: ""}))
	applied, err = db.AdvanceRepoVersion(ctx, "did:plc:wedged", "", "3lpnext000000")
	require.NoError(t, err)
	require.False(t, applied)
	got, err = db.GetRepo("did:plc:wedged")
	require.NoError(t, err)
	require.Empty(t, got.Version, "a wedged repo stays wedged")

	// A repo we have never heard of is not created by an event.
	applied, err = db.AdvanceRepoVersion(ctx, "did:plc:stranger", "3lprev0000000", "3lpnext000000")
	require.NoError(t, err)
	require.False(t, applied)
	got, err = db.GetRepo("did:plc:stranger")
	require.NoError(t, err)
	require.Nil(t, got)
}

// TestAdvanceRepoVersionRace: firehose events are handled a goroutine each, so
// commits on one repo race. Exactly one of them may win each hop, and the row
// must end up on the chain rather than wherever the last writer happened to be.
func TestAdvanceRepoVersionRace(t *testing.T) {
	db := indexedTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpdateRepo(&Repo{DID: "did:plc:a", Version: "3lprev0000000"}))

	const racers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			applied, err := db.AdvanceRepoVersion(ctx, "did:plc:a", "3lprev0000000", "3lpnext000000")
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if applied {
				winners++
			}
		}()
	}
	close(start)
	wg.Wait()

	require.Equal(t, 1, winners, "one commit follows a given rev, so one CAS applies")
	got, err := db.GetRepo("did:plc:a")
	require.NoError(t, err)
	require.Equal(t, "3lpnext000000", got.Version)
}

// TestMarkRepoForRepair: the mark reuses the wedge every repair path already
// keys on, and must not take the repo's history down with it.
func TestMarkRepoForRepair(t *testing.T) {
	db := indexedTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpdateRepo(fullRepo("did:plc:a")))

	marked, err := db.MarkRepoForRepair(ctx, "did:plc:a", "3lprev0000000")
	require.NoError(t, err)
	require.True(t, marked)

	got, err := db.GetRepo("did:plc:a")
	require.NoError(t, err)
	require.Empty(t, got.Version, "the wedge is what makes the sweep pick it up")
	require.Equal(t, "3lprev0000000", got.RepairFrom, "where the missed span starts")
	require.Equal(t, "bafyreiabc", got.RootCID)
	require.Equal(t, "3lpfloor00000", got.BackfillFloor)
	require.True(t, got.BackfillDone, "a gap in the last hour does not un-index five years")
	require.Equal(t, "someone.example", got.Handle)
	require.Equal(t, RepoStatusOK, got.Status)

	// It is a CAS too: a row somebody already wedged, or already moved past,
	// is left exactly as they left it.
	marked, err = db.MarkRepoForRepair(ctx, "did:plc:a", "3lprev0000000")
	require.NoError(t, err)
	require.False(t, marked)
	got, err = db.GetRepo("did:plc:a")
	require.NoError(t, err)
	require.Equal(t, "3lprev0000000", got.RepairFrom)

	marked, err = db.MarkRepoForRepair(ctx, "did:plc:a", "")
	require.NoError(t, err)
	require.False(t, marked, "there is nothing to repair from")
}

// TestSQLiteBusyTimeout: the pragma is per-connection, so the only proof it
// took is asking the connection.
func TestSQLiteBusyTimeout(t *testing.T) {
	db := indexedTestDB(t)
	var timeout int
	require.NoError(t, db.DB.Raw("PRAGMA busy_timeout").Scan(&timeout).Error)
	require.Equal(t, int(SQLiteBusyTimeout.Milliseconds()), timeout)
}
