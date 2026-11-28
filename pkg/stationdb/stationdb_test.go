//go:build linux && cgo && stationslate

package stationdb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type memoryObject struct {
	body []byte
	etag string
}

type memoryObjects struct {
	mu     sync.Mutex
	values map[string]memoryObject
	getErr func(key string) error
	putErr func(key string) error
	lost   func(key string) error
}

func (m *memoryObjects) get(ctx context.Context, key string) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		if err := m.getErr(key); err != nil {
			return nil, "", err
		}
	}
	v, ok := m.values[key]
	if !ok {
		return nil, "", errMissing
	}
	return bytes.Clone(v.body), v.etag, nil
}

func (m *memoryObjects) empty(ctx context.Context, prefix string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.values {
		if strings.HasPrefix(key, prefix) {
			return false, nil
		}
	}
	return true, nil
}

func (m *memoryObjects) put(ctx context.Context, key string, body []byte, match string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.putErr != nil {
		if err := m.putErr(key); err != nil {
			return "", err
		}
	}
	if old, ok := m.values[key]; (match == "" && ok) || (match != "" && (!ok || old.etag != match)) {
		return "", errCAS
	}
	digest := sha256.Sum256(body)
	etag := hex.EncodeToString(digest[:])
	m.values[key] = memoryObject{body: bytes.Clone(body), etag: etag}
	if m.lost != nil {
		if err := m.lost(key); err != nil {
			return "", err
		}
	}
	return etag, nil
}

func newMemoryObjects() *memoryObjects {
	return &memoryObjects{values: make(map[string]memoryObject)}
}

const testKey = "01234567890123456789012345678901"

func openTest(t *testing.T, objects *memoryObjects) (*Store, *sql.DB) {
	t.Helper()
	s, err := openStore(t.Context(), objects, "station", []byte(testKey))
	require.NoError(t, err)
	db := s.openSQL()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return s, db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(t.Context(), query, args...)
	require.NoError(t, err)
}

func readString(t *testing.T, db *sql.DB, q string) string {
	t.Helper()
	var value string
	require.NoError(t, db.QueryRowContext(t.Context(), q).Scan(&value))
	return value
}

func TestOpenStoreRejectsHeadlessNamespace(t *testing.T) {
	for _, key := range []string{"pages/10000000000000000000000000000000", "locks/session"} {
		t.Run(key, func(t *testing.T) {
			m := newMemoryObjects()
			name := "station/stationdb/v1/" + key
			object := memoryObject{body: []byte("surviving object"), etag: "original"}
			m.values[name] = object

			s, err := openStore(t.Context(), m, "station", []byte(testKey))
			require.ErrorContains(t, err, "namespace is corrupt or incomplete")
			require.Nil(t, s)
			require.Equal(t, map[string]memoryObject{name: object}, m.values)
		})
	}
}

type emptyCheckObjects struct {
	*memoryObjects
	beforeEmpty func(context.Context) error
}

func (m emptyCheckObjects) empty(ctx context.Context, prefix string) (bool, error) {
	if err := m.beforeEmpty(ctx); err != nil {
		return false, err
	}
	return m.memoryObjects.empty(ctx, prefix)
}

func TestOpenStoreReadsHeadCreatedBeforeEmptyCheck(t *testing.T) {
	m := newMemoryObjects()
	reader := emptyCheckObjects{memoryObjects: m, beforeEmpty: func(ctx context.Context) error {
		// Interleave another open between the reader's missing-head read and its list.
		_, err := openStore(ctx, m, "station", []byte(testKey))
		m.putErr = func(key string) error {
			t.Errorf("unexpected write after another opener initialized the namespace: %s", key)
			return errCAS
		}
		return err
	}}

	s, err := openStore(t.Context(), reader, "station", []byte(testKey))
	require.NoError(t, err)
	require.Equal(t, m.values["station/stationdb/v1/head"].etag, s.latest.Load().etag)
}

func TestOpenStoreDoesNotInitializeAfterEmptyCheckFailure(t *testing.T) {
	m := newMemoryObjects()
	listErr := errors.New("list denied")
	reader := emptyCheckObjects{memoryObjects: m, beforeEmpty: func(context.Context) error {
		return listErr
	}}

	s, err := openStore(t.Context(), reader, "station", []byte(testKey))
	require.ErrorIs(t, err, listErr)
	require.Nil(t, s)
	require.Empty(t, m.values)
}

func TestRecoveryAndPageDelta(t *testing.T) {
	m := newMemoryObjects()
	s, a := openTest(t, m)
	_, b := openTest(t, m)
	mustExec(t, a, "CREATE TABLE data (id INTEGER PRIMARY KEY, label TEXT, payload BLOB)")
	mustExec(t, a, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<24)
		INSERT INTO data SELECT i, 'station secret', zeroblob(4096) FROM n`)
	before := s.latest.Load()
	objectsBefore := len(m.values)
	require.Equal(t, "station secret", readString(t, b, "SELECT label FROM data WHERE id=1"))
	mustExec(t, a, "UPDATE data SET label='changed secret' WHERE id=1")
	after := s.latest.Load()
	require.Len(t, after.pages, len(before.pages))
	changed := 0
	for i, page := range after.pages {
		if page.id != before.pages[i].id {
			changed++
		}
	}
	require.Positive(t, changed)
	require.Less(t, changed, len(after.pages))
	require.Len(t, m.values, objectsBefore+changed)
	for name, object := range m.values {
		require.NotContains(t, string(object.body), "SQLite format 3", name)
		require.NotContains(t, string(object.body), "station secret", name)
		require.NotContains(t, string(object.body), "changed secret", name)
	}
	require.Equal(t, "changed secret", readString(t, b, "SELECT label FROM data WHERE id=1"))
	_, restarted := openTest(t, m)
	require.Equal(t, "changed secret", readString(t, restarted, "SELECT label FROM data WHERE id=1"))
	wrongKey := bytes.Repeat([]byte{1}, 32)
	_, err := openStore(t.Context(), m, "station", wrongKey)
	require.ErrorContains(t, err, "authenticate")

	// A missing referenced page is not a missing database.
	head := m.values[s.prefix+"head"].etag
	page := after.pages[0].id
	delete(m.values, s.prefix+"pages/"+hex.EncodeToString(page[:]))
	_, err = openStore(t.Context(), m, "station", []byte(testKey))
	require.ErrorIs(t, err, errMissing)
	require.Equal(t, head, m.values[s.prefix+"head"].etag)
}

func TestPreparedReturningAndTransactions(t *testing.T) {
	m := newMemoryObjects()
	_, a := openTest(t, m)
	_, b := openTest(t, m)
	stmt, err := a.PrepareContext(t.Context(), "INSERT INTO data(label) VALUES(?) RETURNING id, label")
	require.NoError(t, err)
	defer stmt.Close()
	mustExec(t, b, "CREATE TABLE data (id INTEGER PRIMARY KEY, label TEXT UNIQUE)")
	var id int
	var label string
	require.NoError(t, stmt.QueryRowContext(t.Context(), "first").Scan(&id, &label))
	require.Equal(t, "first", label)
	require.Equal(t, "first", readString(t, b, "SELECT label FROM data WHERE id=1"))

	result, err := a.ExecContext(t.Context(), "INSERT INTO data(label) VALUES('second'), ('third') RETURNING id")
	require.NoError(t, err)
	count, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 2, count)

	tx, err := a.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	require.NoError(t, err)
	defer tx.Rollback()
	mustExec(t, b, "UPDATE data SET label='remote' WHERE id=1")
	require.NoError(t, tx.QueryRowContext(t.Context(), "SELECT label FROM data WHERE id=1").Scan(&label))
	require.Equal(t, "first", label)
	_, err = tx.ExecContext(t.Context(), "DELETE FROM data")
	require.Error(t, err)
	require.NoError(t, tx.Commit())
	require.Equal(t, "remote", readString(t, a, "SELECT label FROM data WHERE id=1"))

	tx, err = a.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.ExecContext(t.Context(), "SAVEPOINT keep")
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), "UPDATE data SET label='discarded' WHERE id=1")
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), "ROLLBACK TO keep")
	require.NoError(t, err)
	require.NoError(t, tx.StmtContext(t.Context(), stmt).QueryRowContext(t.Context(), "fourth").Scan(&id, &label))
	require.Equal(t, "fourth", label)
	var n int
	require.NoError(t, b.QueryRowContext(t.Context(), "SELECT count(*) FROM data").Scan(&n))
	require.Equal(t, 3, n)
	require.NoError(t, tx.Commit())
	require.Equal(t, "remote", readString(t, b, "SELECT label FROM data WHERE id=1"))
	require.Equal(t, "fourth", readString(t, b, "SELECT label FROM data WHERE id=4"))

	tx, err = a.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.ExecContext(t.Context(), "INSERT OR ROLLBACK INTO data(label) VALUES('fourth')")
	require.Error(t, err)
	_, err = tx.ExecContext(t.Context(), "DELETE FROM data")
	require.ErrorIs(t, err, errAborted)
	require.ErrorIs(t, tx.Commit(), errAborted)
	require.Equal(t, "remote", readString(t, a, "SELECT label FROM data WHERE id=1"))
}

func TestConcurrentCommits(t *testing.T) {
	m := newMemoryObjects()
	_, a := openTest(t, m)
	_, b := openTest(t, m)
	mustExec(t, a, "CREATE TABLE data (label TEXT)")

	txA, err := a.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer txA.Rollback()
	txB, err := b.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer txB.Rollback()
	_, err = txA.ExecContext(t.Context(), "INSERT INTO data VALUES ('a')")
	require.NoError(t, err)
	_, err = txB.ExecContext(t.Context(), "INSERT INTO data VALUES ('b')")
	require.NoError(t, err)

	type commit struct {
		label string
		err   error
	}
	start := make(chan struct{})
	done := make(chan commit, 2)
	go func() { <-start; done <- commit{"a", txA.Commit()} }()
	go func() { <-start; done <- commit{"b", txB.Commit()} }()
	close(start)
	winner, loser := <-done, <-done
	if winner.err != nil {
		winner, loser = loser, winner
	}
	require.NoError(t, winner.err)
	require.ErrorIs(t, loser.err, ErrConflict)
	require.Equal(t, winner.label, readString(t, a, "SELECT group_concat(label) FROM data"))
	require.Equal(t, winner.label, readString(t, b, "SELECT group_concat(label) FROM data"))
}

func TestDurabilityFailures(t *testing.T) {
	m := newMemoryObjects()
	s, a := openTest(t, m)
	_, b := openTest(t, m)
	mustExec(t, a, "CREATE TABLE data (label TEXT)")
	mustExec(t, a, "INSERT INTO data VALUES ('durable')")
	require.Equal(t, "durable", readString(t, b, "SELECT label FROM data"))
	fault := errors.New("injected object failure")
	m.putErr = func(key string) error {
		if strings.Contains(key, "/pages/") {
			return fault
		}
		return nil
	}
	label := "not set"
	err := a.QueryRowContext(t.Context(), "UPDATE data SET label='unpublished' RETURNING label").Scan(&label)
	require.ErrorIs(t, err, fault)
	require.Equal(t, "not set", label)
	m.putErr = nil
	require.Equal(t, "durable", readString(t, a, "SELECT label FROM data"))

	m.getErr = func(key string) error {
		if key == s.prefix+"head" {
			return fault
		}
		return nil
	}
	require.ErrorIs(t, b.QueryRowContext(t.Context(), "SELECT label FROM data").Scan(&label), fault)
	m.getErr = nil

	m.lost = func(key string) error {
		if key == s.prefix+"head" {
			return fault
		}
		return nil
	}
	_, err = a.ExecContext(t.Context(), "UPDATE data SET label='committed but unacknowledged'")
	require.ErrorIs(t, err, ErrCommitUnknown)
	require.ErrorIs(t, err, fault)
	m.lost = nil
	require.Equal(t, "committed but unacknowledged", readString(t, b, "SELECT label FROM data"))
	require.Equal(t, "committed but unacknowledged", readString(t, a, "SELECT label FROM data"))
}

func TestLateSQLiteConstraintDoesNotPublish(t *testing.T) {
	m := newMemoryObjects()
	_, a := openTest(t, m)
	a.SetMaxOpenConns(1)
	_, b := openTest(t, m)
	mustExec(t, a, "PRAGMA foreign_keys=ON")
	mustExec(t, a, "CREATE TABLE parent (id INTEGER PRIMARY KEY)")
	mustExec(t, a, "CREATE TABLE child (id INTEGER PRIMARY KEY REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED)")
	mustExec(t, b, "INSERT INTO parent VALUES(2)")
	_, err := a.ExecContext(t.Context(), "INSERT INTO child VALUES(1) RETURNING id")
	require.Error(t, err)
	var id int
	require.Error(t, a.QueryRowContext(t.Context(), "INSERT INTO child VALUES(3) RETURNING id").Scan(&id))
	var count int
	require.NoError(t, b.QueryRowContext(t.Context(), "SELECT count(*) FROM child").Scan(&count))
	require.Zero(t, count)
}

func TestNamedLockCAS(t *testing.T) {
	m := newMemoryObjects()
	a, _ := openTest(t, m)
	first, err := a.GetNamedLock("migrate")
	require.NoError(t, err)
	var key string
	for name := range m.values {
		if strings.HasPrefix(name, a.prefix+"locks/") {
			key = name
		}
	}
	require.NotEmpty(t, key)
	oldETag := m.values[key].etag

	// Simulate restart and a waiter that reaches its deadline after seeing the held marker.
	b, _ := openTest(t, m)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m.getErr = func(name string) error {
		if name == key {
			cancel()
		}
		return nil
	}
	_, err = b.getNamedLock(ctx, "migrate")
	require.ErrorIs(t, err, ErrLockBusy)
	m.getErr = nil
	require.Equal(t, oldETag, m.values[key].etag)

	first()
	freeETag := m.values[key].etag
	second, err := b.GetNamedLock("migrate")
	require.NoError(t, err)
	current := m.values[key].etag
	a.releaseLock(key, oldETag)
	require.Equal(t, current, m.values[key].etag)
	second()
	require.NotEqual(t, freeETag, m.values[key].etag)
}

func TestGORMMigrationAndCreate(t *testing.T) {
	m := newMemoryObjects()
	_, a := openTest(t, m)
	_, b := openTest(t, m)
	type widget struct {
		ID   uint
		Name string
	}
	writer, err := gorm.Open(sqlite.New(sqlite.Config{Conn: a}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	reader, err := gorm.Open(sqlite.New(sqlite.Config{Conn: b}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	require.NoError(t, writer.WithContext(t.Context()).AutoMigrate(&widget{}))
	require.NoError(t, reader.WithContext(t.Context()).AutoMigrate(&widget{}))
	want := widget{Name: "durable"}
	require.NoError(t, writer.WithContext(t.Context()).Create(&want).Error)
	require.NotZero(t, want.ID)
	var got widget
	require.NoError(t, reader.WithContext(t.Context()).First(&got, want.ID).Error)
	require.Equal(t, want, got)
}
