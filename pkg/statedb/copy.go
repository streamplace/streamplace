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

// Copying a state database from one engine to another (the sqlite → Postgres
// move when a node grows up). Rows travel through the Go models, so every
// difference between the two schemas — bytea vs blob, jsonb vs text,
// timestamptz vs datetime text, bool vs int — is GORM's type mapping doing
// what it already does on every read and write; nothing is hand-mapped.
//
// The copy is idempotent (insert on conflict do nothing, keyed by each
// table's primary key), so it can be run once while the old node is still up
// to prove the target out, and again after stopping it to catch the delta.

// CopyReport is one table's outcome.
type CopyReport struct {
	Table    string
	Source   int64 // rows in the source (soft-deleted included)
	Inserted int64 // rows this run inserted
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

// CopyState copies every table in StatefulDBModels from fromURL into toURL.
// The target is opened exactly as a node would open it (created if missing,
// AutoMigrated to the current schema). batch is rows per INSERT.
func CopyState(ctx context.Context, fromURL, toURL string, batch int) ([]CopyReport, error) {
	if batch <= 0 {
		batch = 500
	}
	src, srcType, err := openSource(fromURL)
	if err != nil {
		return nil, err
	}
	log.Log(ctx, "copy: source open", "url", redactDBURL(fromURL), "type", srcType)
	dst, err := MakeDB(ctx, &config.CLI{DBURL: toURL}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error opening target database: %w", err)
	}
	log.Log(ctx, "copy: target open", "url", redactDBURL(toURL), "type", dst.Type)

	var reports []CopyReport
	var problems []string
	for _, m := range StatefulDBModels {
		r, err := copyTable(ctx, src, dst, m, batch)
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

func copyTable(ctx context.Context, src *gorm.DB, dst *StatefulDB, m any, batch int) (CopyReport, error) {
	s, err := schema.Parse(m, &sync.Map{}, dst.DB.NamingStrategy)
	if err != nil {
		return CopyReport{}, err
	}
	r := CopyReport{Table: s.Table}
	if err := src.Unscoped().Model(m).Count(&r.Source).Error; err != nil {
		return r, fmt.Errorf("%s: counting source rows: %w", s.Table, err)
	}
	started := time.Now()
	// A pointer to a []Model, built by reflection since the model list is
	// []any; FindInBatches walks the source in primary-key order.
	rows := reflect.New(reflect.SliceOf(reflect.TypeOf(m)))
	// Hooks off: rows are copied as they are, not re-created. Unscoped on
	// the target too, so a soft-deleted row is inserted with its deleted_at.
	writer := dst.DB.Session(&gorm.Session{SkipHooks: true, Context: ctx}).Unscoped()
	res := src.WithContext(ctx).Unscoped().Model(m).FindInBatches(rows.Interface(), batch, func(tx *gorm.DB, n int) error {
		normalizeJSON(rows.Elem())
		ins := writer.Clauses(clause.OnConflict{DoNothing: true}).Create(rows.Interface())
		if ins.Error != nil {
			return ins.Error
		}
		r.Inserted += ins.RowsAffected
		return nil
	})
	if res.Error != nil {
		return r, fmt.Errorf("%s: %w", s.Table, res.Error)
	}
	if dst.Type == DBTypePostgres {
		if err := bumpSequence(ctx, dst.DB, s); err != nil {
			return r, err
		}
	}
	if err := dst.DB.Unscoped().Model(m).Count(&r.Target).Error; err != nil {
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
