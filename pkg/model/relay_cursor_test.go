package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUpsertRelayCursor is the storage half of the replay-window cap: the
// cursor is only safe to resume from if we also know how old it is, so both
// columns have to survive the round trip and both have to move on an upsert.
func TestUpsertRelayCursor(t *testing.T) {
	db := indexedTestDB(t)
	const host = "wss://relay.example"

	// Nothing recorded yet reads as "fresh subscription", not as an error.
	stored, err := db.GetRelayCursor(host)
	require.NoError(t, err)
	require.Nil(t, stored)

	require.NoError(t, db.UpsertRelayCursor(host, 120, 1700000000))
	stored, err = db.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, host, stored.Host)
	require.Equal(t, int64(120), stored.Cursor)
	require.Equal(t, int64(1700000000), stored.LastEventTime)

	// An upsert overwrites both columns rather than inserting a second row --
	// the event time especially, since a stale one next to a fresh cursor is
	// exactly the state the staleness check exists to catch.
	require.NoError(t, db.UpsertRelayCursor(host, 500, 1700009999))
	stored, err = db.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(500), stored.Cursor)
	require.Equal(t, int64(1700009999), stored.LastEventTime)
	require.Equal(t, int64(1), countRows(t, db, &RelayCursor{}))

	// A reset writes zeroes to both, and that has to persist as written rather
	// than being skipped as a zero value.
	require.NoError(t, db.UpsertRelayCursor(host, 0, 0))
	stored, err = db.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(0), stored.Cursor)
	require.Equal(t, int64(0), stored.LastEventTime)

	// Cursors are per-relay.
	require.NoError(t, db.UpsertRelayCursor("moqt://other.example", 640, 1700000042))
	stored, err = db.GetRelayCursor("moqt://other.example")
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(640), stored.Cursor)
	require.Equal(t, int64(1700000042), stored.LastEventTime)
	require.Equal(t, int64(2), countRows(t, db, &RelayCursor{}))
}
