package kvfs

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/psanford/sqlite3vfs"
	"github.com/stretchr/testify/require"
)

// fakeStore is an in-memory Store that models object storage: a write is
// visible to this process immediately, but only Flush moves it into what a new
// reader (a restarted process) would find. snapshot is that reader's view.
type fakeStore struct {
	mu      sync.Mutex
	live    map[string][]byte
	durable map[string][]byte

	gets, scans, batches, flushes int
}

func newFakeStore() *fakeStore {
	return &fakeStore{live: map[string][]byte{}, durable: map[string][]byte{}}
}

func (f *fakeStore) Get(key []byte) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	value, ok := f.live[string(key)]
	if !ok {
		return nil, false, nil
	}
	return bytes.Clone(value), true, nil
}

func (f *fakeStore) Write(mutations []Mutation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches++
	for _, m := range mutations {
		if m.Value == nil {
			delete(f.live, string(m.Key))
			continue
		}
		f.live[string(m.Key)] = bytes.Clone(m.Value)
	}
	return nil
}

func (f *fakeStore) Scan(prefix []byte) ([]Row, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scans++
	var rows []Row
	for key, value := range f.live {
		if strings.HasPrefix(key, string(prefix)) {
			rows = append(rows, Row{Key: []byte(key), Value: bytes.Clone(value)})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return bytes.Compare(rows[i].Key, rows[j].Key) < 0 })
	return rows, nil
}

func (f *fakeStore) DeletePrefix(prefix []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for key := range f.live {
		if strings.HasPrefix(key, string(prefix)) {
			delete(f.live, key)
		}
	}
	return nil
}

func (f *fakeStore) Flush() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushes++
	f.durable = map[string][]byte{}
	for key, value := range f.live {
		f.durable[key] = bytes.Clone(value)
	}
	return nil
}

// snapshot is the store a process would find if this one stopped right now,
// without the clean shutdown that would have flushed its remaining writes.
func (f *fakeStore) snapshot() *fakeStore {
	f.mu.Lock()
	defer f.mu.Unlock()
	snap := newFakeStore()
	for key, value := range f.durable {
		snap.live[key] = bytes.Clone(value)
		snap.durable[key] = bytes.Clone(value)
	}
	return snap
}

func (f *fakeStore) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.live))
	for key := range f.live {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (f *fakeStore) counters() (batches, flushes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.batches, f.flushes
}

// openTestDB opens state.db through a fresh VFS over store. One connection, as
// a state database runs: exclusive locking means a second one would just wait
// on the first. The journal mode is write-ahead log, which is where the state
// database runs every SQLite database (statedb's sqlitePragmas) and what makes
// crash recovery a question about the log in the store.
func openTestDB(t *testing.T, store Store, opts Options) *sql.DB {
	t.Helper()
	v, err := New(store, opts)
	require.NoError(t, err)
	db, err := sql.Open("sqlite3", v.DSN("state.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA journal_mode = WAL")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestRoundTrip runs a real SQLite database over a store: schema, rows, a value
// larger than a chunk, a temp table, and a reopen that finds all of it again.
func TestRoundTrip(t *testing.T) {
	store := newFakeStore()
	db := openTestDB(t, store, Options{})

	// The helper opens in write-ahead log mode, as the state database does.
	// That works over this VFS only because the DSN put the connection in
	// exclusive locking mode: with no shared memory to map, SQLite keeps the
	// wal-index in heap memory.
	var mode string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&mode))
	require.Equal(t, "wal", mode)

	_, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT, blob BLOB)")
	require.NoError(t, err)

	big := bytes.Repeat([]byte("streamplace"), 32*1024) // 352 KiB, many chunks
	for i := range 20 {
		_, err := db.Exec("INSERT INTO t (id, name, blob) VALUES (?, ?, ?)", i, fmt.Sprintf("row-%d", i), big)
		require.NoError(t, err)
	}
	// Rows written in one transaction, and a rewrite of an existing page.
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec("UPDATE t SET name = ? WHERE id = ?", "rewritten", 3)
	require.NoError(t, err)
	_, err = tx.Exec("INSERT INTO t (id, name, blob) VALUES (?, ?, ?)", 100, "late", []byte("small"))
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// Temp tables are served from memory, not the store. temp_store has to be
	// set first: changing it closes the temp database, and with it any temp
	// table already in it.
	_, err = db.Exec("PRAGMA temp_store = FILE")
	require.NoError(t, err)
	_, err = db.Exec("CREATE TEMP TABLE scratch AS SELECT * FROM t")
	require.NoError(t, err)
	var ordered int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM (SELECT * FROM scratch ORDER BY blob)").Scan(&ordered))
	require.Equal(t, 21, ordered)

	requireRows := func(db *sql.DB) {
		t.Helper()
		var name string
		require.NoError(t, db.QueryRow("SELECT name FROM t WHERE id = 3").Scan(&name))
		require.Equal(t, "rewritten", name)
		var blob []byte
		require.NoError(t, db.QueryRow("SELECT blob FROM t WHERE id = 7").Scan(&blob))
		require.True(t, bytes.Equal(blob, big), "large value round-tripped intact")
		var count int
		require.NoError(t, db.QueryRow("SELECT count(*) FROM t").Scan(&count))
		require.Equal(t, 21, count)
	}
	requireRows(db)

	batches, flushes := store.counters()
	require.Greater(t, batches, 0, "commits reached the store")
	require.Greater(t, flushes, 0, "commits waited for the store's flush barrier")

	require.NoError(t, db.Close())

	// A fresh VFS over the same store: the database is a database, not a
	// process-local cache.
	reopened := openTestDB(t, store, Options{})
	requireRows(reopened)
	require.NoError(t, reopened.Close())
}

// TestRecoverFromCrash checks the durability story: a process that dies without
// closing leaves behind exactly what it synced, and a new process replays the
// write-ahead log it finds in the store.
func TestRecoverFromCrash(t *testing.T) {
	store := newFakeStore()
	db := openTestDB(t, store, Options{})

	_, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	// Committed by now, so durable: synchronous=FULL makes SQLite ask for a
	// sync per commit, and a sync is where the store is flushed.
	_, err = db.Exec("INSERT INTO t (id, name) VALUES (1, 'survives')")
	require.NoError(t, err)

	// The process stops here, without closing the database: no checkpoint, no
	// deleted write-ahead log, just whatever made it into the store.
	survived := store.snapshot()

	recovered := openTestDB(t, survived, Options{})
	var name string
	require.NoError(t, recovered.QueryRow("SELECT name FROM t WHERE id = 1").Scan(&name))
	require.Equal(t, "survives", name)
	require.NoError(t, recovered.Close())
}

// TestSmallCacheWritesBack forces chunk evictions while a transaction is still
// running, which is the path where a cache that dropped dirty chunks instead of
// writing them back would quietly eat data.
func TestSmallCacheWritesBack(t *testing.T) {
	store := newFakeStore()
	db := openTestDB(t, store, Options{CacheBytes: 8 << 10})

	_, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, blob BLOB)")
	require.NoError(t, err)

	// Position-dependent bytes, so a chunk landing at the wrong offset cannot
	// pass by accident.
	payload := func(i, size int) []byte {
		out := make([]byte, size)
		for j := range out {
			out[j] = byte(i*7 + j)
		}
		return out
	}
	for i := range 64 {
		_, err := db.Exec("INSERT INTO t (id, blob) VALUES (?, ?)", i, payload(i, 4096*(i%7+1)))
		require.NoError(t, err)
	}
	requireRows := func(db *sql.DB) {
		t.Helper()
		for i := range 64 {
			var got []byte
			require.NoError(t, db.QueryRow("SELECT blob FROM t WHERE id = ?", i).Scan(&got))
			require.Equal(t, payload(i, 4096*(i%7+1)), got, "row %d", i)
		}
	}
	requireRows(db)
	require.NoError(t, db.Close())

	reopened := openTestDB(t, store, Options{CacheBytes: 8 << 10})
	requireRows(reopened)
	require.NoError(t, reopened.Close())
}

// TestTruncateDropsChunks is the focused case for shrinking a file: the chunks
// past the new end have to leave the store, and the region they covered has to
// read back as zeros rather than as what used to be there.
func TestTruncateDropsChunks(t *testing.T) {
	store := newFakeStore()
	v, err := New(store, Options{})
	require.NoError(t, err)

	f, _, err := v.Open("data.db", 0)
	require.Error(t, err, "opening a file that is not there without create fails")
	require.Nil(t, f)

	f, _, err = v.Open("data.db", sqlite3vfs.OpenCreate)
	require.NoError(t, err)

	// Three chunks of data, then a sync so all of it is in the store.
	full := v.chunkSize * 3
	payload := bytes.Repeat([]byte("x"), int(full))
	n, err := f.WriteAt(payload, 0)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	require.NoError(t, f.Sync(0))
	keys := store.keys()
	require.Equal(t, []string{"chunk/data.db/00000000", "chunk/data.db/00000001", "chunk/data.db/00000002", "len/data.db"}, keys)

	// Shrink to a single chunk: the other two are gone from the store, not
	// just hidden.
	require.NoError(t, f.Truncate(v.chunkSize))
	size, err := f.FileSize()
	require.NoError(t, err)
	require.Equal(t, v.chunkSize, size)
	require.NoError(t, f.Sync(0))
	keys = store.keys()
	require.Equal(t, []string{"chunk/data.db/00000000", "len/data.db"}, keys)

	// Grow again over the truncated region: those bytes are zeros, not the
	// payload that used to live there.
	tail := bytes.Repeat([]byte("y"), 16)
	n, err = f.WriteAt(tail, v.chunkSize*2)
	require.NoError(t, err)
	require.Equal(t, 16, n)
	require.NoError(t, f.Sync(0))

	read := make([]byte, v.chunkSize)
	n, err = f.ReadAt(read, v.chunkSize)
	require.NoError(t, err)
	require.Equal(t, len(read), n)
	require.True(t, bytes.Equal(read, make([]byte, v.chunkSize)), "holes read as zeros")

	n, err = f.ReadAt(read[:32], v.chunkSize*2-16)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	require.Equal(t, make([]byte, 16), read[:16], "the gap before the new data is zeros")
	require.Equal(t, tail, read[16:32])

	// A file shorter than the chunk it ends in does not hand out the bytes
	// past its length: a read stops at the end of the file, not at the end of
	// the chunk holding it.
	require.NoError(t, f.Truncate(100))
	short := make([]byte, 256)
	n, err = f.ReadAt(short, 0)
	require.Equal(t, 100, n)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, payload[:100], short[:100], "the bytes the file still has are the ones it reads")

	// Shrinking into the middle of a chunk takes the bytes it cut out of the
	// chunk itself, not just off the length: the store is told about them,
	// and growing back over them reads zeros.
	require.NoError(t, f.Sync(0))
	stored, ok, err := store.Get([]byte("chunk/data.db/00000000"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, make([]byte, v.chunkSize-100), stored[100:], "the tail a truncate cut is zeroed in the store")

	require.NoError(t, f.Truncate(v.chunkSize))
	n, err = f.ReadAt(short[:100], 100)
	require.NoError(t, err)
	require.Equal(t, 100, n)
	require.Equal(t, make([]byte, 100), short[:100], "the bytes a truncate cut do not come back")

	require.NoError(t, f.Close())
}

// TestFileLifecycle covers the file questions SQLite asks: does this file
// exist, may I create it, and what does deleting it leave behind.
func TestFileLifecycle(t *testing.T) {
	store := newFakeStore()
	v, err := New(store, Options{})
	require.NoError(t, err)

	exists, err := v.Access("journal", sqlite3vfs.AccessExists)
	require.NoError(t, err)
	require.False(t, exists)

	f, _, err := v.Open("journal", sqlite3vfs.OpenCreate)
	require.NoError(t, err)
	exists, err = v.Access("journal", sqlite3vfs.AccessExists)
	require.NoError(t, err)
	require.True(t, exists, "a file SQLite just created exists, even at zero length")
	size, err := f.FileSize()
	require.NoError(t, err)
	require.Equal(t, int64(0), size)

	// Exclusive create refuses what is already there.
	_, _, err = v.Open("journal", sqlite3vfs.OpenCreate|sqlite3vfs.OpenExclusive)
	require.Error(t, err)

	_, err = f.WriteAt([]byte("hot journal"), 0)
	require.NoError(t, err)
	require.NoError(t, f.Sync(0))
	require.Contains(t, store.keys(), "chunk/journal/00000000")

	require.NoError(t, f.Close())
	require.NoError(t, v.Delete("journal", false))
	exists, err = v.Access("journal", sqlite3vfs.AccessExists)
	require.NoError(t, err)
	require.False(t, exists)
	require.Empty(t, store.keys(), "a deleted file leaves nothing behind")

	// Names that could reach outside the file's own key space are refused.
	_, _, err = v.Open("../escape", sqlite3vfs.OpenCreate)
	require.Error(t, err)

	// Temporary files (the empty name) live in memory and never touch a store.
	temp, _, err := v.Open("", sqlite3vfs.OpenCreate|sqlite3vfs.OpenTempDB)
	require.NoError(t, err)
	_, err = temp.WriteAt([]byte("scratch"), 0)
	require.NoError(t, err)
	buf := make([]byte, 7)
	n, err := temp.ReadAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, "scratch", string(buf[:n]))
	require.NoError(t, temp.Close())
	require.Empty(t, store.keys())
}

// TestRedactURLSecrets: a database URL's credentials live in its query string
// (an object store URL is written with access keys and session tokens), and
// everything that formats one - a log line, an error message built from an
// error that quoted the URL back - has to take them out first.
func TestRedactURLSecrets(t *testing.T) {
	rawURL := "slatedb+s3://bucket/state?endpoint=http://localhost:9000&access_key_id=key&secret_access_key=hunter2&session_token=toksecret"
	redacted := RedactURLSecrets(rawURL)
	require.NotContains(t, redacted, "hunter2")
	require.NotContains(t, redacted, "toksecret")
	require.Contains(t, redacted, "access_key_id=key", "the key's identifier is not a secret")
	require.Contains(t, redacted, "endpoint=", "the rest of the URL is what makes the message useful")

	// The secrets come back on their own too, for scrubbing text that was not
	// built here.
	require.Equal(t, []string{"hunter2", "toksecret"}, URLSecrets(rawURL))

	// A query that will not percent-decode still loses its secrets, and the
	// value it wrote is still one to scrub for: a URL that is broken is
	// exactly the one that must not be quoted whole.
	broken := "slatedb+s3://bucket/state?access_key_id=key&secret_access_key=hunter2%zz"
	require.Contains(t, RedactURLSecrets(broken), "secret_access_key=redacted")
	require.NotContains(t, RedactURLSecrets(broken), "hunter2")
	require.Equal(t, []string{"hunter2%zz"}, URLSecrets(broken))

	// A URL with nothing to hide is returned as it was, and one that will not
	// parse is not quoted back at all.
	plain := "slatedb+memory://tests/one"
	require.Equal(t, plain, RedactURLSecrets(plain))
	require.Equal(t, "unparseable database URL", RedactURLSecrets("http://[::1"))
	require.Empty(t, URLSecrets("http://[::1"))
}
