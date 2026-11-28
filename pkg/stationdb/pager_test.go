//go:build linux && cgo && stationslate

package stationdb

import (
	"database/sql/driver"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPagerTruncateAndRegrow(t *testing.T) {
	first := &pageData{pageSize - 3: 1}
	second := &pageData{0: 2, pageSize - 1: 3}
	third := &pageData{0: 4, 1: 5}
	base := &snapshot{pages: []storedPage{{data: first}, {data: second}, {data: third}}}
	p := newPager(base)

	n, err := p.WriteAt([]byte{6, 7, 8, 9}, pageSize-2)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	var buf [5]byte
	n, err = p.ReadAt(buf[:], pageSize-3)
	require.NoError(t, err)
	require.Equal(t, len(buf), n)
	require.Equal(t, [5]byte{1, 6, 7, 8, 9}, buf)

	require.NoError(t, p.Truncate(2*pageSize+1))
	size, err := p.FileSize()
	require.NoError(t, err)
	require.EqualValues(t, 2*pageSize+1, size)
	n, err = p.ReadAt(buf[:], 2*pageSize-1)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, 2, n)
	require.Equal(t, []byte{3, 4}, buf[:n])

	require.NoError(t, p.Truncate(3*pageSize))
	_, err = p.ReadAt(buf[:], 2*pageSize-1)
	require.NoError(t, err)
	require.Equal(t, [5]byte{3, 4, 0, 0, 0}, buf)

	require.NoError(t, p.Truncate(pageSize))
	_, err = p.WriteAt([]byte{10}, 2*pageSize+1)
	require.NoError(t, err)
	_, err = p.ReadAt(buf[:3], 2*pageSize-1)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 10}, buf[:3])
	_, err = newPager(base).ReadAt(buf[:], pageSize-3)
	require.NoError(t, err)
	require.Equal(t, [5]byte{1, 0, 0, 2, 0}, buf)
	_, err = newPager(base).ReadAt(buf[:2], 2*pageSize)
	require.NoError(t, err)
	require.Equal(t, []byte{4, 5}, buf[:2])
}

func TestPagerRemotePageReuse(t *testing.T) {
	ctx := t.Context()
	objects := newMemoryObjects()
	var pageReads, pageWrites int
	objects.getErr = func(key string) error {
		if strings.Contains(key, "/pages/") {
			pageReads++
		}
		return nil
	}
	objects.putErr = func(key string) error {
		if strings.Contains(key, "/pages/") {
			pageWrites++
		}
		return nil
	}
	writer, err := openStore(ctx, objects, "station", make([]byte, 32))
	require.NoError(t, err)
	base, err := writer.load(ctx)
	require.NoError(t, err)
	p := newPager(base)
	var header pageData
	copy(header[:], "SQLite format 3\x00")
	binary.BigEndian.PutUint16(header[16:18], pageSize)
	_, err = p.WriteAt(header[:], 0)
	require.NoError(t, err)
	_, err = p.WriteAt([]byte{1}, 3*pageSize-1)
	require.NoError(t, err)
	initial, err := writer.publish(ctx, p)
	require.NoError(t, err)
	p.reset(initial)

	reader, err := openStore(ctx, objects, "station", make([]byte, 32))
	require.NoError(t, err)
	before, err := reader.load(ctx)
	require.NoError(t, err)
	reads, writes := pageReads, pageWrites

	// One unchanged dirty page, one changed page, one untouched page.
	_, err = p.WriteAt(header[:], 0)
	require.NoError(t, err)
	_, err = p.WriteAt([]byte{2}, pageSize)
	require.NoError(t, err)
	next, err := writer.publish(ctx, p)
	require.NoError(t, err)
	require.Equal(t, writes+1, pageWrites)
	require.Same(t, initial.pages[0].data, next.pages[0].data)
	require.Same(t, initial.pages[2].data, next.pages[2].data)

	after, err := reader.load(ctx)
	require.NoError(t, err)
	require.Equal(t, reads+1, pageReads)
	require.Same(t, before.pages[0].data, after.pages[0].data)
	require.Same(t, before.pages[2].data, after.pages[2].data)
	require.EqualValues(t, 0, before.pages[1].data[0])
	require.EqualValues(t, 2, after.pages[1].data[0])

	p.reset(next)
	require.NoError(t, p.Truncate(pageSize))
	require.NoError(t, p.Truncate(3*pageSize))
	_, err = writer.publish(ctx, p)
	require.NoError(t, err)
	after, err = reader.load(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 0, after.pages[1].data[0])
	require.EqualValues(t, 0, after.pages[2].data[pageSize-1])
}

func TestPagerSQLCommitsAndRefreshes(t *testing.T) {
	ctx := t.Context()
	objects := newMemoryObjects()
	open := func() *connection {
		store, err := openStore(ctx, objects, "station", make([]byte, 32))
		require.NoError(t, err)
		c := &connection{store: store}
		t.Cleanup(func() { require.NoError(t, c.Close()) })
		return c
	}
	exec := func(c *connection, query string) {
		t.Helper()
		_, err := c.ExecContext(ctx, query, nil)
		require.NoError(t, err)
	}
	scalar := func(c *connection, query string) driver.Value {
		t.Helper()
		rows, err := c.QueryContext(ctx, query, nil)
		require.NoError(t, err)
		defer func() { require.NoError(t, rows.Close()) }()
		var values [1]driver.Value
		require.NoError(t, rows.Next(values[:]))
		return values[0]
	}

	writer, reader := open(), open()
	exec(writer, "CREATE TABLE amounts (n INTEGER)")
	local := writer.db
	require.EqualValues(t, 7, scalar(writer, "INSERT INTO amounts VALUES (7) RETURNING n"))
	require.Same(t, local, writer.db)
	require.EqualValues(t, 7, scalar(reader, "SELECT sum(n) FROM amounts"))
	require.Equal(t, "memory", scalar(writer, "PRAGMA journal_mode"))
	require.EqualValues(t, pageSize, scalar(writer, "PRAGMA page_size"))

	tx, err := reader.BeginTx(ctx, driver.TxOptions{})
	require.NoError(t, err)
	exec(reader, "INSERT INTO amounts VALUES (99)")
	exec(writer, "INSERT INTO amounts VALUES (11)")
	require.ErrorIs(t, tx.Commit(), ErrConflict)
	require.EqualValues(t, 18, scalar(reader, "SELECT sum(n) FROM amounts"))

	old := reader.db
	exec(writer, "INSERT INTO amounts VALUES (5)")
	require.EqualValues(t, 23, scalar(reader, "SELECT sum(n) FROM amounts"))
	require.NotSame(t, old, reader.db)

	tx, err = writer.BeginTx(ctx, driver.TxOptions{})
	require.NoError(t, err)
	exec(writer, "INSERT INTO amounts VALUES (100)")
	require.NoError(t, tx.Rollback())
	require.EqualValues(t, 23, scalar(writer, "SELECT sum(n) FROM amounts"))
	require.EqualValues(t, 23, scalar(reader, "SELECT sum(n) FROM amounts"))

	tx, err = writer.BeginTx(ctx, driver.TxOptions{})
	require.NoError(t, err)
	writer.db.RegisterCommitHook(nil)
	exec(writer, "INSERT INTO amounts VALUES (100)")
	require.ErrorContains(t, tx.Commit(), "without a commit hook")
	require.EqualValues(t, 23, scalar(reader, "SELECT sum(n) FROM amounts"))
}
