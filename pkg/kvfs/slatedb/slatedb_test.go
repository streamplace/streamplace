package slatedb

import (
	"bytes"
	"strings"
	"testing"

	// pkg/kvfs builds on the sqlite3 C library (through psanford/sqlite3vfs),
	// which in this repo's Go binaries always arrives with mattn's driver.
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/kvfs"
)

// openMemory opens a store in the process-wide in-memory object store. The name
// keeps tests from sharing a database.
func openMemory(t *testing.T, name string) *Store {
	t.Helper()
	store, err := Open("slatedb+memory://"+name+"/state", Options{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

func TestMemoryRoundTrip(t *testing.T) {
	store := openMemory(t, "roundtrip")

	value := bytes.Repeat([]byte("state"), 1024)
	require.NoError(t, store.Write([]kvfs.Mutation{
		{Key: []byte("chunk/state.db/00000000"), Value: value},
		{Key: []byte("chunk/state.db/00000001"), Value: []byte("second")},
		{Key: []byte("chunk/state.db/00000002"), Value: []byte("third")},
	}))
	require.NoError(t, store.Flush())

	got, ok, err := store.Get([]byte("chunk/state.db/00000000"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, value, got)

	_, ok, err = store.Get([]byte("chunk/state.db/00000009"))
	require.NoError(t, err)
	require.False(t, ok, "a key that was never written is not found, not an error")

	// Scan is ordered and stops at the prefix: the ordering is what makes a
	// chunk that lands out of order detectable.
	rows, err := store.Scan([]byte("chunk/state.db/"))
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Equal(t, []byte("chunk/state.db/00000000"), rows[0].Key)
	require.Equal(t, value, rows[0].Value)
	require.Equal(t, []byte("chunk/state.db/00000002"), rows[2].Key)

	// A delete in a batch, alongside a write.
	require.NoError(t, store.Write([]kvfs.Mutation{
		{Key: []byte("chunk/state.db/00000002")},
		{Key: []byte("chunk/state.db/00000003"), Value: []byte("fourth")},
	}))
	rows, err = store.Scan([]byte("chunk/state.db/"))
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Equal(t, []byte("chunk/state.db/00000003"), rows[2].Key)

	// Deleting a prefix removes exactly it: the file next door survives,
	// which is how the write-ahead log is thrown away without touching the
	// database.
	require.NoError(t, store.Write([]kvfs.Mutation{{Key: []byte("chunk/state.db-wal/00000000"), Value: []byte("wal")}}))
	require.NoError(t, store.DeletePrefix([]byte("chunk/state.db-wal/")))
	rows, err = store.Scan([]byte("chunk/"))
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.False(t, strings.HasPrefix(string(rows[0].Key), "chunk/state.db-wal/"))

	// A reopened store finds what the first one wrote.
	require.NoError(t, store.Close())
	reopened, err := Open("slatedb+memory://roundtrip/state", Options{})
	require.NoError(t, err)
	defer reopened.Close()
	got, ok, err = reopened.Get([]byte("chunk/state.db/00000000"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, value, got)
}

// TestSingleWriter: SlateDB is single-writer per path, so a second open of the
// same database in one process has to be refused rather than quietly start
// racing the first.
func TestSingleWriter(t *testing.T) {
	first := openMemory(t, "single-writer")
	_, err := Open("slatedb+memory://single-writer/state", Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already open")

	// A different database in the same store is fine.
	second, err := Open("slatedb+memory://single-writer/other", Options{})
	require.NoError(t, err)
	require.NoError(t, second.Close())

	// Closing releases the claim.
	require.NoError(t, first.Close())
	reopened, err := Open("slatedb+memory://single-writer/state", Options{})
	require.NoError(t, err)
	require.NoError(t, reopened.Close())
}

func TestParseURL(t *testing.T) {
	cases := []struct {
		url     string
		store   string
		path    string
		wantErr string
	}{
		{
			url:   "slatedb+s3://station-state/blue/city?endpoint=http://localhost:9000&access_key_id=k&secret_access_key=s",
			store: "s3://station-state?endpoint=http://localhost:9000&access_key_id=k&secret_access_key=s",
			path:  "blue/city",
		},
		{
			url:   "slatedb+memory://tests/one",
			store: "memory://tests",
			path:  "one",
		},
		{
			url:     "slatedb+s3://bucket",
			wantErr: "names no database",
		},
		{
			url:     "slatedb+s3://",
			wantErr: "names no bucket",
		},
		{
			url:     "slatedb://bucket/path",
			wantErr: "must start with slatedb+",
		},
		{
			url:     "sqlite:///tmp/state.sqlite",
			wantErr: "must start with slatedb+",
		},
	}
	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			store, path, err := parseURL(c.url)
			if c.wantErr != "" {
				require.ErrorContains(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.store, store)
			require.Equal(t, c.path, path)
		})
	}
}

func TestOpenRejectsBadURLs(t *testing.T) {
	_, err := Open("postgres://user@host/db", Options{})
	require.ErrorContains(t, err, "must start with slatedb+")

	// A store the object store layer does not know has to come back as an
	// error naming what was being opened, not a hang or a bare status.
	_, err = Open("slatedb+gopher://bucket/state", Options{})
	require.ErrorContains(t, err, "slatedb: opening")
}

func TestDecodeRowsRejectsMalformed(t *testing.T) {
	// A row count that promises more rows than the payload holds.
	_, err := decodeRows([]byte{1, 0, 0, 0, 5, 0, 0, 0, 'a'})
	require.ErrorContains(t, err, "malformed")

	// Trailing bytes are not a valid row list either.
	value, err := decodeRows([]byte{0, 0, 0, 0, 7})
	require.ErrorContains(t, err, "trailing bytes")
	require.Nil(t, value)

	// The empty row list scans to no rows, not to an error.
	rows, err := decodeRows([]byte{0, 0, 0, 0})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestEncodeBatchLayout(t *testing.T) {
	payload := encodeBatch([]kvfs.Mutation{
		{Key: []byte("ab"), Value: []byte("c")},
		{Key: []byte("de")},
	})
	require.Equal(t, []byte{
		2, 0, 0, 0, // two operations
		opPut, 2, 0, 0, 0, 1, 0, 0, 0, 'a', 'b', 'c',
		opDelete, 2, 0, 0, 0, 0, 0, 0, 0, 'd', 'e',
	}, payload)
}
