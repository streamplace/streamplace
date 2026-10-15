package kvfs

import (
	"io"

	"github.com/psanford/sqlite3vfs"
)

// memFile is what SQLite gets when it asks for a temporary file: scratch space
// that is thrown away with the connection, and that has no business in object
// storage. SQLite only ever hands these to the connection that created them, so
// no locking beyond the level SQLite itself tracks is needed.
type memFile struct {
	data []byte
	lock sqlite3vfs.LockType
}

func newMemFile() *memFile { return &memFile{} }

func (m *memFile) Close() error { return nil }

func (m *memFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(m.data)) {
		return 0, io.EOF
	}
	if off < 0 {
		return 0, sqlite3vfs.IOError
	}
	n := copy(p, m.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (m *memFile) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, sqlite3vfs.IOError
	}
	if end := off + int64(len(p)); end > int64(len(m.data)) {
		grown := make([]byte, end)
		copy(grown, m.data)
		m.data = grown
	}
	copy(m.data[off:], p)
	return len(p), nil
}

func (m *memFile) Truncate(size int64) error {
	if size < 0 {
		return sqlite3vfs.IOError
	}
	if size <= int64(len(m.data)) {
		m.data = m.data[:size]
		return nil
	}
	grown := make([]byte, size)
	copy(grown, m.data)
	m.data = grown
	return nil
}

func (m *memFile) Sync(sqlite3vfs.SyncType) error { return nil }

func (m *memFile) FileSize() (int64, error) { return int64(len(m.data)), nil }

func (m *memFile) Lock(want sqlite3vfs.LockType) error {
	if want > m.lock {
		m.lock = want
	}
	return nil
}

func (m *memFile) Unlock(level sqlite3vfs.LockType) error {
	if level < m.lock {
		m.lock = level
	}
	return nil
}

func (m *memFile) CheckReservedLock() (bool, error) { return false, nil }

func (m *memFile) SectorSize() int64 { return 0 }

func (m *memFile) DeviceCharacteristics() sqlite3vfs.DeviceCharacteristic { return 0 }

var _ sqlite3vfs.File = (*memFile)(nil)
