package upload

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/stationdb"
)

const stationTestPrefix = "stations/test"

func TestStationUploadFencesWriterBlockedInClose(t *testing.T) {
	dsn, disk := stationTestStorage(t)
	stalled := &stationCloseGate{Store: disk, entered: make(chan struct{}), release: make(chan struct{})}
	a := newStationTestPeer(t, dsn, stalled)
	b := newStationTestPeer(t, dsn, disk)
	t.Cleanup(stalled.unblock)
	up := a.create(t, 7)

	loser := make(chan stationTestResponse, 1)
	go func() {
		loser <- a.request(t.Context(), up, http.MethodPatch, 0, strings.NewReader("BAD"))
	}()
	stationAwait(t, stalled.entered)
	b.head(t, up, 0, 7)
	b.patch(t, up, 0, "abc", 3)
	b.patch(t, up, 3, "de", 5)

	// No state in the winning process is needed to resume the upload.
	b.server.Close()
	require.NoError(t, b.pool.Close())
	c := newStationTestPeer(t, dsn, disk)
	c.head(t, up, 5, 7)
	c.patch(t, up, 5, "fg", 7)

	stalled.unblock()
	stationAssertStatus(t, stationAwait(t, loser), http.StatusConflict)
	c.head(t, up, 7, 7)
	c.requireDelivery(t, disk, up, "abcdefg")
}

func TestStationUploadCompletionCommitRecovery(t *testing.T) {
	dsn, disk := stationTestStorage(t)
	a := newStationTestPeer(t, dsn, disk)
	b := newStationTestPeer(t, dsn, disk)
	up := a.create(t, 6)
	a.pool.fail.Store(stationFailAfterCommit)
	lost := a.request(t.Context(), up, http.MethodPatch, 0, strings.NewReader("abc"))
	stationAssertStatus(t, lost, http.StatusServiceUnavailable)
	require.Contains(t, lost.body, "check HEAD before resubmitting")
	b.head(t, up, 3, 6) // The lost reply must not discard this referenced part.
	taskKey := "vod-process:" + up.UploadID

	// The assembled candidate exists, but none of the SQL writes survive.
	a.pool.fail.Store(stationFailBeforeCommit)
	stationAssertStatus(t, a.request(t.Context(), up, http.MethodPatch, 3, strings.NewReader("BAD")), http.StatusServiceUnavailable)
	a.server.Close()
	require.NoError(t, a.pool.Close())
	b.head(t, up, 3, 6)
	task, err := b.state.GetTaskByKey(t.Context(), taskKey)
	require.NoError(t, err)
	require.Nil(t, task)
	row, err := b.state.GetUpload(t.Context(), up.UploadID)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Nil(t, row.CompletedAt)
	require.Empty(t, row.Location)

	// The retry's SQL commits, but the writer sees an error. It must not
	// remove the winning object or re-enqueue the processing task.
	b.pool.fail.Store(stationFailAfterCommit)
	stationAssertStatus(t, b.request(t.Context(), up, http.MethodPatch, 3, strings.NewReader("def")), http.StatusServiceUnavailable)
	b.server.Close()
	require.NoError(t, b.pool.Close())
	c := newStationTestPeer(t, dsn, disk)
	c.head(t, up, 6, 6)
	stationAssertStatus(t, c.request(t.Context(), up, http.MethodPatch, 3, strings.NewReader("BAD")), http.StatusConflict)
	c.patch(t, up, 6, "", 6)
	c.requireDelivery(t, disk, up, "abcdef")

	stationAssertStatus(t, c.request(t.Context(), up, http.MethodDelete, 0, nil), http.StatusNoContent)
	stationAssertStatus(t, c.request(t.Context(), up, http.MethodHead, 0, nil), http.StatusNotFound)
	c.requireDelivery(t, disk, up, "abcdef") // A queued worker still owns the final object.
}

func TestStationUploadRejectsOverflowAndFencesTerminatedUpload(t *testing.T) {
	dsn, disk := stationTestStorage(t)
	stalled := &stationCloseGate{Store: disk, entered: make(chan struct{}), release: make(chan struct{})}
	a := newStationTestPeer(t, dsn, stalled)
	b := newStationTestPeer(t, dsn, disk)
	t.Cleanup(stalled.unblock)
	up := b.create(t, 3)

	wrong := *up
	wrong.UploadID = b.create(t, 3).UploadID // Token is bound to its original ID.
	stationAssertStatus(t, a.request(t.Context(), &wrong, http.MethodHead, 0, nil), http.StatusUnauthorized)
	overflow := io.NopCloser(strings.NewReader("1234")) // Chunked HTTP, no Content-Length to reject upfront.
	stationAssertStatus(t, b.request(t.Context(), up, http.MethodPatch, 0, overflow), http.StatusRequestEntityTooLarge)
	b.head(t, up, 0, 3)

	pending := make(chan stationTestResponse, 1)
	go func() {
		pending <- a.request(t.Context(), up, http.MethodPatch, 0, strings.NewReader("abc"))
	}()
	stationAwait(t, stalled.entered)
	stationAssertStatus(t, b.request(t.Context(), up, http.MethodDelete, 0, nil), http.StatusNoContent)
	stalled.unblock()
	stationAssertStatus(t, stationAwait(t, pending), http.StatusNotFound)
	stationAssertStatus(t, b.request(t.Context(), up, http.MethodHead, 0, nil), http.StatusNotFound)
	task, err := b.state.GetTaskByKey(t.Context(), "vod-process:"+up.UploadID)
	require.NoError(t, err)
	require.Nil(t, task)
}

func TestStationUploadConcurrentAuthKeyCreation(t *testing.T) {
	dsn, disk := stationTestStorage(t)
	a, _ := openStationTestState(t, dsn)
	b, _ := openStationTestState(t, dsn)
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	// Both constructors must observe an absent signing key before either inserts.
	for _, state := range []*statedb.StatefulDB{a, b} {
		require.NoError(t, state.DB.Callback().Create().Before("gorm:create").Register("test:hold_auth_insert", func(tx *gorm.DB) {
			if tx.Statement.Table == "configs" {
				arrived <- struct{}{}
				<-release
			}
		}))
	}
	type started struct {
		m   *Manager
		err error
	}
	peers := make(chan started, 2)
	for _, state := range []*statedb.StatefulDB{a, b} {
		go func() {
			m, err := newStationManager(t.Context(), state, disk, BackendFile, stationTestPrefix)
			peers <- started{m: m, err: err}
		}()
	}
	stationAwait(t, arrived)
	stationAwait(t, arrived)
	unblock()
	first, second := stationAwait(t, peers), stationAwait(t, peers)
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	pa, pb := startStationTestPeer(t, first.m, nil), startStationTestPeer(t, second.m, nil)
	up := pa.create(t, 1)
	pb.head(t, up, 0, 1)
	pa.head(t, pb.create(t, 1), 0, 1)
}

func stationTestStorage(t *testing.T) (string, blob.Store) {
	t.Helper()
	root := t.TempDir()
	dsn := filepath.Join(root, "station.db") + "?_journal_mode=WAL&_busy_timeout=3000"
	state, _ := openStationTestState(t, dsn)
	require.NoError(t, state.DB.AutoMigrate(&statedb.Config{}, &statedb.Upload{}, &statedb.DraftVideo{}, &statedb.AppTask{}))
	disk, err := blob.NewFileStore(filepath.Join(root, "objects"))
	require.NoError(t, err)
	return dsn, disk
}

func openStationTestState(t *testing.T, dsn string) (*statedb.StatefulDB, *stationTestPool) {
	t.Helper()
	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	pool := &stationTestPool{DB: db}
	gdb, err := gorm.Open(sqlite.Dialector{Conn: pool}, &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	return &statedb.StatefulDB{DB: gdb, Type: statedb.DBTypeSQLite}, pool
}

type stationTestPeer struct {
	m      *Manager
	state  *statedb.StatefulDB
	pool   *stationTestPool
	server *httptest.Server
}

func newStationTestPeer(t *testing.T, dsn string, disk blob.Store) *stationTestPeer {
	t.Helper()
	state, pool := openStationTestState(t, dsn)
	m, err := newStationManager(t.Context(), state, disk, BackendFile, stationTestPrefix)
	require.NoError(t, err)
	return startStationTestPeer(t, m, pool)
}

func startStationTestPeer(t *testing.T, m *Manager, pool *stationTestPool) *stationTestPeer {
	s := httptest.NewServer(m)
	s.Client().Timeout = 5 * time.Second
	t.Cleanup(s.Close)
	return &stationTestPeer{m: m, state: m.state, pool: pool, server: s}
}

func (p *stationTestPeer) create(t *testing.T, size int64) *CreateResult {
	t.Helper()
	up, err := p.m.Create(t.Context(), "did:plc:station", "video/mp4", "take.mp4", size, p.server.URL)
	require.NoError(t, err)
	return up
}

func (p *stationTestPeer) head(t *testing.T, up *CreateResult, offset, size int64) {
	t.Helper()
	header := stationAssertStatus(t, p.request(t.Context(), up, http.MethodHead, 0, nil), http.StatusOK)
	require.Equal(t, strconv.FormatInt(offset, 10), header.Get("Upload-Offset"))
	require.Equal(t, strconv.FormatInt(size, 10), header.Get("Upload-Length"))
	require.Equal(t, "no-store", header.Get("Cache-Control"))
}

func (p *stationTestPeer) patch(t *testing.T, up *CreateResult, offset int64, body string, want int64) {
	t.Helper()
	header := stationAssertStatus(t, p.request(t.Context(), up, http.MethodPatch, offset, strings.NewReader(body)), http.StatusNoContent)
	require.Equal(t, strconv.FormatInt(want, 10), header.Get("Upload-Offset"))
}

func (p *stationTestPeer) requireDelivery(t *testing.T, disk blob.Store, up *CreateResult, want string) {
	t.Helper()
	row, err := p.state.GetUpload(t.Context(), up.UploadID)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.NotNil(t, row.CompletedAt)
	tasks, err := p.state.ListTasks(t.Context(), statedb.TaskFilters{Type: statedb.TaskVODProcess, TaskKey: "vod-process:" + up.UploadID})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	var task statedb.VODProcessTask
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &task))
	require.Equal(t, up.UploadID, task.UploadID)
	require.Equal(t, row.Location, task.Location)
	key, ok := disk.ParseLocation(task.Location)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(key, stationTestPrefix+"/uploads/"), "object key outside station namespace: %s", key)
	r, err := disk.Open(t.Context(), key)
	require.NoError(t, err)
	defer r.Close()
	data, err := io.ReadAll(io.NewSectionReader(r, 0, r.Size()))
	require.NoError(t, err)
	require.Equal(t, want, string(data))
}

type stationTestResponse struct {
	status int
	header http.Header
	body   string
	err    error
}

func (p *stationTestPeer) request(ctx context.Context, up *CreateResult, method string, offset int64, body io.Reader) stationTestResponse {
	r, err := http.NewRequestWithContext(ctx, method, p.server.URL+BasePath+up.UploadID, body)
	if err != nil {
		return stationTestResponse{err: err}
	}
	r.Header.Set("Tus-Resumable", "1.0.0")
	r.Header.Set("Authorization", "Bearer "+up.UploadToken)
	if method == http.MethodPatch {
		r.Header.Set("Content-Type", "application/offset+octet-stream")
		r.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))
	}
	res, err := p.server.Client().Do(r)
	if err != nil {
		return stationTestResponse{err: err}
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	return stationTestResponse{status: res.StatusCode, header: res.Header, body: string(data), err: err}
}

func stationAssertStatus(t *testing.T, r stationTestResponse, status int) http.Header {
	t.Helper()
	require.NoError(t, r.err)
	require.Equal(t, status, r.status, "%s", r.body)
	if status != http.StatusUnauthorized {
		require.Equal(t, "1.0.0", r.header.Get("Tus-Resumable"))
	}
	return r.header
}

func stationAwait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case val := <-ch:
		return val
	case <-time.After(5 * time.Second):
		t.Fatal("station test rendezvous timed out")
		var zero T
		return zero
	}
}

type stationCloseGate struct {
	blob.Store
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *stationCloseGate) unblock() { s.once.Do(func() { close(s.release) }) }

func (s *stationCloseGate) NewWriter(ctx context.Context, key, contentType string) (blob.Writer, error) {
	w, err := s.Store.NewWriter(ctx, key, contentType)
	if err != nil {
		return nil, err
	}
	return &stationGatedWriter{Writer: w, gate: s}, nil
}

type stationGatedWriter struct {
	blob.Writer
	gate *stationCloseGate
}

func (w *stationGatedWriter) Close() error {
	close(w.gate.entered)
	<-w.gate.release
	return w.Writer.Close()
}

const (
	stationFailBeforeCommit int32 = iota + 1
	stationFailAfterCommit
)

type stationTestPool struct {
	*sql.DB
	fail atomic.Int32
}

func (p *stationTestPool) GetDBConn() (*sql.DB, error) { return p.DB, nil }

func (p *stationTestPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &stationTestTx{Tx: tx, fail: p.fail.Swap(0)}, nil
}

type stationTestTx struct {
	*sql.Tx
	fail int32
}

func (tx *stationTestTx) Commit() error {
	if tx.fail == stationFailBeforeCommit {
		if err := tx.Rollback(); err != nil {
			return err
		}
		return fmt.Errorf("injected head CAS loss: %w", stationdb.ErrConflict)
	}
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	if tx.fail == stationFailAfterCommit {
		return fmt.Errorf("injected loss of COMMIT response: %w", stationdb.ErrCommitUnknown)
	}
	return nil
}
