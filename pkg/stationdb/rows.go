//go:build linux && cgo && stationslate

package stationdb

import (
	"database/sql/driver"
	"io"
	"reflect"

	"github.com/mattn/go-sqlite3"
)

type sqliteRows struct {
	*sqlite3.SQLiteRows
	stmt *sqlite3.SQLiteStmt
	conn *connection
}

func (r *sqliteRows) Close() error {
	if r.stmt == nil {
		return nil
	}
	err := r.SQLiteRows.Close()
	if closeErr := r.stmt.Close(); err == nil {
		err = closeErr
	}
	r.stmt = nil
	r.conn.readers--
	return err
}

type column struct {
	databaseType string
	scanType     reflect.Type
}

type bufferedRows struct {
	names  []string
	cols   []column
	values []driver.Value
}

func consume(rows *sqliteRows) (*bufferedRows, error) {
	names := rows.Columns()
	r := &bufferedRows{names: names, cols: make([]column, len(names))}
	for i := range r.cols {
		r.cols[i] = column{rows.ColumnTypeDatabaseTypeName(i), rows.ColumnTypeScanType(i)}
	}
	for {
		n := len(r.values)
		r.values = append(r.values, make([]driver.Value, len(names))...)
		err := rows.Next(r.values[n:])
		if err == nil {
			continue
		}
		r.values = r.values[:n]
		closeErr := rows.Close()
		if err != io.EOF {
			return nil, err
		}
		return r, closeErr
	}
}

func (r *bufferedRows) Columns() []string { return r.names }
func (r *bufferedRows) Close() error      { return nil }

func (r *bufferedRows) Next(dst []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	copy(dst, r.values[:len(r.names)])
	r.values = r.values[len(r.names):]
	return nil
}

func (r *bufferedRows) ColumnTypeDatabaseTypeName(i int) string { return r.cols[i].databaseType }
func (r *bufferedRows) ColumnTypeScanType(i int) reflect.Type   { return r.cols[i].scanType }
func (r *bufferedRows) ColumnTypeNullable(int) (bool, bool)     { return true, true }
