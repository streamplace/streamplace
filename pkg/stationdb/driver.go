//go:build linux && cgo && stationslate

package stationdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-sqlite3"
	"stream.place/streamplace/pkg/s3"
)

// Open opens the bucket namespace and returns a database/sql SQLite pool.
// SQL writes that lose the head CAS must be retried by the caller as a whole.
func Open(ctx context.Context, cfg s3.Config, prefix string, key []byte) (*sql.DB, *Store, error) {
	if cfg.Bucket == "" {
		return nil, nil, errors.New("station database requires an S3 bucket")
	}
	prefix, err := validateOpenInputs(prefix, key)
	if err != nil {
		return nil, nil, err
	}
	backend := s3Objects{client: s3.NewClient(cfg), bucket: cfg.Bucket}
	if err := backend.qualifyConditional(ctx, prefix); err != nil {
		return nil, nil, err
	}
	s, err := openValidatedStore(ctx, backend, prefix, key)
	if err != nil {
		return nil, nil, err
	}
	return s.openSQL(), s, nil
}

func (s *Store) openSQL() *sql.DB {
	return sql.OpenDB(connector{s})
}

type connector struct{ store *Store }

func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &connection{store: c.store}, nil
}

func (c connector) Driver() driver.Driver { return c }

func (c connector) Open(string) (driver.Conn, error) {
	return c.Connect(context.Background())
}

var (
	_ driver.ConnBeginTx        = (*connection)(nil)
	_ driver.ConnPrepareContext = (*connection)(nil)
	_ driver.ExecerContext      = (*connection)(nil)
	_ driver.QueryerContext     = (*connection)(nil)
	_ driver.Pinger             = (*connection)(nil)
	_ driver.StmtExecContext    = (*statement)(nil)
	_ driver.StmtQueryContext   = (*statement)(nil)
)

// Each driver connection owns a private in-memory SQLite pager. A failed
// publication invalidates it; only an authenticated bucket snapshot can reopen it.
type connection struct {
	store              *Store
	db                 *sqlite3.SQLiteConn
	pager              *pager
	tx                 *transaction
	readers            int
	dirty              bool
	internal           bool
	captureForeignKeys bool
	foreignKeysPragma  string
}

func (c *connection) Close() error {
	c.pager = nil
	if c.db == nil {
		return nil
	}
	err := c.db.Close()
	c.db = nil
	return err
}

func (c *connection) refresh(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.tx != nil {
		if c.db.AutoCommit() {
			return errAborted
		}
		return nil
	}
	if c.readers != 0 {
		return errors.New("close station SQLite rows before reusing this connection")
	}
	base, err := c.store.load(ctx)
	if err != nil {
		return err
	}
	if c.pager != nil && c.pager.base.etag == base.etag {
		return nil
	}
	// SQLite's page and schema caches belong to the old file, not the new head.
	if err := c.Close(); err != nil {
		return err
	}
	db, p, err := openPager(ctx, base)
	if err != nil {
		return fmt.Errorf("open station SQLite pager: %w", err)
	}
	if c.foreignKeysPragma != "" {
		if _, err := db.ExecContext(ctx, c.foreignKeysPragma, nil); err != nil {
			_ = db.Close()
			return fmt.Errorf("restore station SQLite foreign_keys: %w", err)
		}
	}
	c.db, c.pager = db, p
	db.RegisterAuthorizer(c.authorize)
	db.RegisterCommitHook(func() int {
		c.dirty = true
		return 0
	})
	return nil
}

func (c *connection) Ping(ctx context.Context) error { return c.refresh(ctx) }

func (c *connection) authorize(op int, arg, value, _ string) int {
	if c.internal {
		return sqlite3.SQLITE_OK
	}
	switch op {
	case sqlite3.SQLITE_ATTACH, sqlite3.SQLITE_DETACH, sqlite3.SQLITE_TRANSACTION,
		sqlite3.SQLITE_CREATE_TEMP_INDEX, sqlite3.SQLITE_CREATE_TEMP_TABLE,
		sqlite3.SQLITE_CREATE_TEMP_TRIGGER, sqlite3.SQLITE_CREATE_TEMP_VIEW:
		return sqlite3.SQLITE_DENY
	case sqlite3.SQLITE_SAVEPOINT:
		if c.tx == nil {
			return sqlite3.SQLITE_DENY
		}
	case sqlite3.SQLITE_PRAGMA:
		switch strings.ToLower(arg) {
		case "foreign_keys":
			// SQLite ignores changes to foreign_keys inside a transaction.
			c.captureForeignKeys = c.tx == nil
		case "application_id", "user_version", "defer_foreign_keys",
			"table_info", "table_xinfo", "table_list", "index_info", "index_xinfo", "index_list",
			"foreign_key_list", "foreign_key_check", "integrity_check", "quick_check":
		case "database_list", "collation_list", "compile_options", "page_size", "journal_mode",
			"busy_timeout", "schema_version", "read_uncommitted", "query_only":
			if value != "" {
				return sqlite3.SQLITE_DENY
			}
		default:
			return sqlite3.SQLITE_DENY
		}
	}
	return sqlite3.SQLITE_OK
}

func (c *connection) control(ctx context.Context, query string) error {
	c.internal = true
	defer func() { c.internal = false }()
	_, err := c.db.ExecContext(ctx, query, nil)
	return err
}

func (c *connection) flush(ctx context.Context) error {
	if !c.dirty {
		if c.pager.modified() {
			_ = c.Close()
			return errors.New("station SQLite pager modified without a commit hook")
		}
		return nil
	}
	next, err := c.store.publish(ctx, c.pager)
	if err != nil {
		_ = c.Close()
		return err
	}
	// Only a successful SQL commit and head publication can transfer dirty pages.
	c.pager.reset(next)
	return nil
}

func (c *connection) failed() {
	if c.tx == nil {
		// An error (e.g. OR FAIL or late RETURNING failure) must not leave
		// private edits around to be published by a subsequent operation.
		_ = c.Close()
	}
}

func (c *connection) prepare(ctx context.Context, query string, args []driver.NamedValue) (*sqlite3.SQLiteStmt, error) {
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}
	c.dirty = false
	if query == "" {
		if len(args) != 0 {
			return nil, errors.New("arguments supplied for an empty SQLite statement")
		}
		return nil, nil
	}
	c.captureForeignKeys = false
	stmt, err := c.db.PrepareContext(ctx, query)
	if err != nil {
		c.failed()
		return nil, err
	}
	if n := stmt.NumInput(); n != len(args) {
		_ = stmt.Close()
		c.failed()
		return nil, fmt.Errorf("station SQLite expected %d arguments, got %d", n, len(args))
	}
	if c.captureForeignKeys {
		// SQLite applies this PRAGMA during prepare. Read the effective value
		// rather than interpreting the caller's SQL or boolean spelling.
		enabled, err := c.scalarInt(ctx, "PRAGMA foreign_keys")
		if err != nil {
			_ = stmt.Close()
			c.failed()
			return nil, err
		}
		c.foreignKeysPragma = "PRAGMA foreign_keys=OFF"
		if enabled != 0 {
			c.foreignKeysPragma = "PRAGMA foreign_keys=ON"
		}
	}
	return stmt.(*sqlite3.SQLiteStmt), nil
}

func (c *connection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	query, err := singleStatement(query)
	if err != nil {
		return nil, err
	}
	return c.exec(ctx, query, args)
}

func (c *connection) exec(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	stmt, err := c.prepare(ctx, query, args)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return driver.RowsAffected(0), nil
	}
	returning := !stmt.Readonly() && containsReturning(query)
	result, err := stmt.ExecContext(ctx, args)
	if closeErr := stmt.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		c.failed()
		return nil, err
	}
	if returning {
		// mattn captures changes() at the first RETURNING row, before
		// SQLite finalizes the statement and updates the count.
		changes, err := c.scalarInt(ctx, "SELECT changes()")
		if err != nil {
			c.failed()
			return nil, err
		}
		result = changesResult{Result: result, changes: changes}
	}
	if c.tx == nil {
		if err := c.flush(ctx); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func containsReturning(query string) bool {
	const keyword = "returning"
	for i := 0; i+len(keyword) <= len(query); i++ {
		if query[i]|0x20 == 'r' && strings.EqualFold(query[i:i+len(keyword)], keyword) {
			return true
		}
	}
	return false
}

func (c *connection) scalarInt(ctx context.Context, query string) (int64, error) {
	rows, err := c.db.QueryContext(ctx, query, nil)
	if err != nil {
		return 0, err
	}
	var value [1]driver.Value
	err = rows.Next(value[:])
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		if err == io.EOF {
			return 0, fmt.Errorf("station SQLite %s returned no row", query)
		}
		return 0, err
	}
	return value[0].(int64), nil
}

type changesResult struct {
	driver.Result
	changes int64
}

func (r changesResult) RowsAffected() (int64, error) { return r.changes, nil }

func (c *connection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	query, err := singleStatement(query)
	if err != nil {
		return nil, err
	}
	return c.query(ctx, query, args)
}

func (c *connection) query(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	stmt, err := c.prepare(ctx, query, args)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return &bufferedRows{}, nil
	}
	rows, err := stmt.QueryContext(ctx, args)
	if err != nil {
		_ = stmt.Close()
		c.failed()
		return nil, err
	}
	c.readers++
	r := &sqliteRows{SQLiteRows: rows.(*sqlite3.SQLiteRows), stmt: stmt, conn: c}
	if stmt.Readonly() && !c.dirty {
		return r, nil
	}
	// RETURNING rows must not escape before the statement and head CAS finish.
	result, err := consume(r)
	if err != nil {
		c.failed()
		return nil, err
	}
	if c.tx == nil {
		if err := c.flush(ctx); err != nil {
			return nil, err
		}
	}
	return result, nil
}

var errAborted = errors.New("station SQLite transaction has been rolled back")

func (c *connection) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if opts.Isolation != driver.IsolationLevel(sql.LevelDefault) && opts.Isolation != driver.IsolationLevel(sql.LevelSerializable) {
		return nil, errors.New("station SQLite supports only serializable transactions")
	}
	if c.tx != nil {
		return nil, errors.New("station SQLite transaction already open")
	}
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}
	if opts.ReadOnly {
		if err := c.control(ctx, "PRAGMA query_only=ON"); err != nil {
			_ = c.Close()
			return nil, err
		}
	}
	if err := c.control(ctx, "BEGIN"); err != nil {
		_ = c.Close()
		return nil, err
	}
	tx := &transaction{conn: c, ctx: ctx, readOnly: opts.ReadOnly}
	c.tx = tx
	return tx, nil
}

type transaction struct {
	conn     *connection
	ctx      context.Context
	readOnly bool
}

func (tx *transaction) done() {
	c := tx.conn
	c.tx = nil
	if tx.readOnly && c.db != nil {
		if err := c.control(context.Background(), "PRAGMA query_only=OFF"); err != nil {
			_ = c.Close()
		}
	}
}

func (tx *transaction) Commit() error {
	c := tx.conn
	defer tx.done()
	if c.db.AutoCommit() {
		_ = c.Close()
		return errAborted
	}
	c.dirty = false
	if err := c.control(tx.ctx, "COMMIT"); err != nil {
		_ = c.Close()
		return err
	}
	return c.flush(tx.ctx)
}

func (tx *transaction) Rollback() error {
	c := tx.conn
	defer tx.done()
	if !c.db.AutoCommit() {
		if err := c.control(context.Background(), "ROLLBACK"); err != nil {
			_ = c.Close()
			return err
		}
	}
	if c.pager.modified() {
		return c.Close()
	}
	return nil
}

func (c *connection) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *connection) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query, err := singleStatement(query)
	if err != nil {
		return nil, err
	}
	// Compile against the execution snapshot, never a retired local pager.
	return &statement{conn: c, query: query}, nil
}

type statement struct {
	conn  *connection
	query string
}

func (s *statement) Close() error  { return nil }
func (s *statement) NumInput() int { return -1 }

func (s *statement) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), named(args))
}

func (s *statement) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), named(args))
}

func (s *statement) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.conn.exec(ctx, s.query, args)
}

func (s *statement) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.conn.query(ctx, s.query, args)
}

func named(args []driver.Value) []driver.NamedValue {
	values := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		values[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return values
}
