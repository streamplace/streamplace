package statedb

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

// Rows travel through the Go models so each engine uses its ordinary type mapping.

// CopyReport is one table's outcome.
type CopyReport struct {
	Table    string
	Source   int64 // rows in the source (soft-deleted included)
	Inserted int64 // rows this run wrote (inserted, or reconciled on a later pass)
	Target   int64 // rows in the target afterwards
}

// dialectorFor parses a state database URL the way MakeDB does.
func dialectorFor(dbURL string) (gorm.Dialector, DBType, error) {
	switch {
	case dbURL == ":memory:":
		return sqlite.Open(":memory:"), DBTypeSQLite, nil
	case strings.HasPrefix(dbURL, "sqlite://"):
		return sqlite.Open(dbURL[len("sqlite://"):]), DBTypeSQLite, nil
	case strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://"):
		return postgres.Open(dbURL), DBTypePostgres, nil
	}
	return nil, "", fmt.Errorf("unsupported database URL (most start with sqlite:// or postgresql://): %s", redactDBURL(dbURL))
}

// openSource opens a state database to read from: no AutoMigrate, nothing
// written.
func openSource(dbURL string) (*gorm.DB, DBType, error) {
	dial, dbType, err := dialectorFor(dbURL)
	if err != nil {
		return nil, "", err
	}
	db, err := openDB(dial)
	if err != nil {
		return nil, "", fmt.Errorf("error opening source database: %w", err)
	}
	if dbType == DBTypeSQLite {
		if err := sqlitePragmas(db); err != nil {
			return nil, "", err
		}
	}
	return db, dbType, nil
}

// CopyState copies relational state. A station cutover requires a stopped source
// and commits all tables together into an empty destination.
func CopyState(ctx context.Context, fromURL string, dstConfig *config.CLI, batch int) ([]CopyReport, error) {
	if batch <= 0 {
		batch = 500
	}
	src, srcType, err := openSource(fromURL)
	if err != nil {
		return nil, err
	}
	log.Log(ctx, "copy: source open", "url", redactDBURL(fromURL), "type", srcType)
	srcPool, err := src.DB()
	if err != nil {
		return nil, err
	}
	defer srcPool.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dst, err := MakeDB(ctx, dstConfig, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error opening target database: %w", err)
	}
	log.Log(ctx, "copy: target open", "url", redactDBURL(dstConfig.DBURL), "type", dst.Type)
	dstPool, err := dst.DB.DB()
	if err != nil {
		return nil, err
	}
	defer dstPool.Close()

	if dst.stationStore != nil {
		var count int64
		if err := src.WithContext(ctx).Model(&Upload{}).Where("completed_at IS NULL").Count(&count).Error; err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, fmt.Errorf("source has %d unfinished legacy uploads; finish or cancel them before cutover", count)
		}
		if err := src.WithContext(ctx).Model(&AppTask{}).Where("status = ?", TaskStatusProcessing).Count(&count).Error; err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, fmt.Errorf("source has %d claimed tasks; reconcile outstanding side effects before cutover", count)
		}
		var reports []CopyReport
		err := dst.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, m := range StatefulDBModels {
				var existing int64
				if err := tx.Unscoped().Model(m).Count(&existing).Error; err != nil {
					return err
				}
				if existing != 0 {
					return fmt.Errorf("station migration requires an empty namespace; %T already contains rows", m)
				}
			}
			var err error
			reports, err = copyTables(ctx, src, tx, dst.Type, batch)
			return err
		})
		if err != nil {
			return nil, err
		}
		return reports, nil
	}
	return copyTables(ctx, src, dst.DB, dst.Type, batch)
}

func copyTables(ctx context.Context, src, dst *gorm.DB, dstType DBType, batch int) ([]CopyReport, error) {
	var reports []CopyReport
	var problems []string
	models, err := copyStateSourceModels(ctx, src)
	if err != nil {
		return nil, err
	}
	for _, m := range models {
		r, err := copyTable(ctx, src, dst, dstType, m, batch)
		if err != nil {
			return reports, err
		}
		reports = append(reports, r)
		if r.Target < r.Source {
			problems = append(problems, fmt.Sprintf("%s: source has %d rows, target has %d", r.Table, r.Source, r.Target))
		}
	}
	if len(problems) > 0 {
		return reports, fmt.Errorf("copy finished but some tables are short:\n  %s", strings.Join(problems, "\n  "))
	}
	return reports, nil
}

func copyTable(ctx context.Context, src, dst *gorm.DB, dstType DBType, m any, batch int) (CopyReport, error) {
	s, err := schema.Parse(m, &sync.Map{}, dst.NamingStrategy)
	if err != nil {
		return CopyReport{}, err
	}
	r := CopyReport{Table: s.Table}
	if err := src.Unscoped().Model(m).Count(&r.Source).Error; err != nil {
		return r, fmt.Errorf("%s: counting source rows: %w", s.Table, err)
	}
	started := time.Now()
	// Walk the entire primary key. GORM's FindInBatches cursor uses one field
	// and cannot advance through a table such as (streamer_did, livestream_uri).
	rows := reflect.New(reflect.SliceOf(reflect.TypeOf(m)))
	// Hooks off: rows are copied as they are, not re-created. Unscoped on
	// the target too, so a soft-deleted row is inserted with its deleted_at.
	writer := dst.Session(&gorm.Session{SkipHooks: true, Context: ctx}).Unscoped()
	if len(s.PrimaryFields) == 0 {
		return r, fmt.Errorf("%s: no primary key for migration", s.Table)
	}
	columns := make([]string, len(s.PrimaryFields))
	cursor := make([]any, len(columns))
	q := src.WithContext(ctx).Unscoped().Model(m).Limit(batch)
	for i, field := range s.PrimaryFields {
		columns[i] = `"` + field.DBName + `"`
		q = q.Order(clause.OrderByColumn{Column: clause.Column{Name: field.DBName}})
	}
	after := "(" + strings.Join(columns, ",") + ") > (" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"
	page := q.Session(&gorm.Session{})
	for {
		if err := page.Find(rows.Interface()).Error; err != nil {
			return r, fmt.Errorf("read %s: %w", s.Table, err)
		}
		if rows.Elem().Len() == 0 {
			break
		}
		normalizeJSON(rows.Elem())
		// Upsert, not insert-or-skip: the documented second (delta) pass
		// after the cutover has to carry rows that changed since the first
		// pass (tasks, configs, branding, soft deletions), which keep their
		// primary keys and would otherwise be left at their old values.
		ins := writer.Clauses(clause.OnConflict{UpdateAll: true}).Create(rows.Interface())
		if ins.Error != nil {
			return r, fmt.Errorf("copy %s: %w", s.Table, ins.Error)
		}
		r.Inserted += ins.RowsAffected
		if rows.Elem().Len() < batch {
			break
		}
		last := rows.Elem().Index(rows.Elem().Len() - 1)
		for i, field := range s.PrimaryFields {
			cursor[i], _ = field.ValueOf(ctx, last)
		}
		page = q.Session(&gorm.Session{}).Where(after, cursor...)
	}
	if dstType == DBTypePostgres {
		if err := bumpSequence(ctx, dst, s); err != nil {
			return r, err
		}
	}
	if err := dst.Unscoped().Model(m).Count(&r.Target).Error; err != nil {
		return r, fmt.Errorf("%s: counting target rows: %w", s.Table, err)
	}
	log.Log(ctx, "copy: table done", "table", s.Table, "source", r.Source, "inserted", r.Inserted, "target", r.Target, "took", time.Since(started).Round(time.Millisecond))
	return r, nil
}

// bumpSequence moves an autoincrement table's sequence past the ids that were
// copied in with explicit values, so the node's first insert doesn't collide.
// Tables whose key isn't a serial have no sequence (pg_get_serial_sequence is
// NULL) and setval of NULL is a no-op.
func bumpSequence(ctx context.Context, db *gorm.DB, s *schema.Schema) error {
	pk := s.PrioritizedPrimaryField
	if pk == nil || !pk.AutoIncrement || (pk.DataType != schema.Int && pk.DataType != schema.Uint) {
		return nil
	}
	q := fmt.Sprintf(
		`SELECT setval(pg_get_serial_sequence('%s', '%s'), COALESCE((SELECT MAX(%s) FROM %s), 0) + 1, false)`,
		s.Table, pk.DBName, pk.DBName, s.Table,
	)
	if err := db.WithContext(ctx).Exec(q).Error; err != nil {
		return fmt.Errorf("%s: resetting sequence: %w", s.Table, err)
	}
	return nil
}

var rawMessageType = reflect.TypeOf(json.RawMessage{})

// normalizeJSON turns an empty or invalid json.RawMessage into NULL in every
// row of a batch: sqlite stored whatever it was handed, but a jsonb column
// rejects "" on the way in.
func normalizeJSON(rows reflect.Value) {
	if rows.Len() == 0 {
		return
	}
	var fields []int
	t := rows.Index(0).Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type == rawMessageType {
			fields = append(fields, i)
		}
	}
	if len(fields) == 0 {
		return
	}
	for i := 0; i < rows.Len(); i++ {
		row := rows.Index(i)
		for _, f := range fields {
			v := row.Field(f)
			if b := v.Bytes(); len(b) > 0 && !json.Valid(b) || len(b) == 0 && !v.IsNil() {
				v.Set(reflect.Zero(rawMessageType))
			}
		}
	}
}
