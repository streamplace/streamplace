package kvfs

import (
	"fmt"
	"io"
	"sort"

	"github.com/psanford/sqlite3vfs"
)

// file is one open SQLite file: its length, the chunks written since the last
// sync, and the lock it holds. A file is only ever touched while its VFS's
// mutex is held.
type file struct {
	vfs      *VFS
	name     string
	size     int64
	readOnly bool

	// storedSize is the length the store was last told about. SQLite reads a
	// file's length as soon as it writes, so the in-memory size leads this one
	// until the next sync catches up.
	storedSize int64

	// allocated is one past the highest chunk index that might exist in the
	// store. Chunks at or above it are holes — never written, or truncated
	// away — and read as zeros.
	allocated uint32

	// dirty is the set of chunk indexes written since the last sync. Their
	// current contents live in the VFS cache.
	dirty map[uint32]bool

	lock   sqlite3vfs.LockType
	closed bool
}

func (f *file) Close() error {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	// A closed file's chunks leave the cache, so anything written and not yet
	// stored has to go out now: SQLite may reopen the file in this process and
	// expect to find what it wrote, exactly as the OS would keep it.
	if !f.readOnly {
		if err := f.writeChunksLocked(); err != nil {
			v.removeFileLocked(f)
			v.cache.dropFile(f)
			return err
		}
	}
	v.removeFileLocked(f)
	v.cache.dropFile(f)
	return nil
}

func (f *file) ReadAt(p []byte, off int64) (int, error) {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return 0, sqlite3vfs.IOError
	}
	if off < 0 {
		return 0, v.ioError("read", fmt.Errorf("negative offset %d in %s", off, f.name))
	}
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		if pos >= f.size {
			break
		}
		ch, err := f.chunkLocked(uint32(pos / v.chunkSize))
		if err != nil {
			return n, err
		}
		// Chunks are stored whole: a read stops at the end of the file and at the
		// end of the chunk holding it, never past either, so the bytes a file
		// has already lost do not come back.
		start := pos % v.chunkSize
		end := min(int64(len(p)-n), f.size-pos, v.chunkSize-start)
		n += copy(p[n:n+int(end)], ch.data[start:start+end])
	}
	if n < len(p) {
		// Short read: SQLite wants EOF and zeroes the rest itself.
		return n, io.EOF
	}
	return n, nil
}

func (f *file) WriteAt(p []byte, off int64) (int, error) {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return 0, sqlite3vfs.IOError
	}
	if f.readOnly {
		return 0, sqlite3vfs.ReadOnlyError
	}
	if off < 0 {
		return 0, v.ioError("write", fmt.Errorf("negative offset %d in %s", off, f.name))
	}
	for n := 0; n < len(p); {
		pos := off + int64(n)
		index := uint32(pos / v.chunkSize)
		ch, err := f.chunkLocked(index)
		if err != nil {
			return n, err
		}
		n += copy(ch.data[pos%v.chunkSize:], p[n:])
		ch.dirty = true
		f.dirty[index] = true
		if index >= f.allocated {
			f.allocated = index + 1
		}
	}
	if end := off + int64(len(p)); end > f.size {
		f.size = end
	}
	return len(p), nil
}

func (f *file) Truncate(size int64) error {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return sqlite3vfs.IOError
	}
	if f.readOnly {
		return sqlite3vfs.ReadOnlyError
	}
	if size < 0 {
		return v.ioError("truncate", fmt.Errorf("negative size %d for %s", size, f.name))
	}
	if size >= f.size {
		// Growing writes zeros; the chunks nobody writes stay holes.
		f.size = size
		return nil
	}
	keep := chunksFor(size, v.chunkSize)
	for start := keep; start < f.allocated; {
		end := min(start+writeBatchLimit, f.allocated)
		deletes := make([]Mutation, 0, end-start)
		for i := start; i < end; i++ {
			if ch, ok := v.cache.get(cacheKey{f.name, i}); ok {
				// Whatever the cache holds for these chunks is gone; keeping it
				// would let a later write to the same index resurrect it.
				ch.dirty = false
				v.cache.remove(ch)
			}
			delete(f.dirty, i)
			deletes = append(deletes, Mutation{Key: chunkKey(f.name, i)})
		}
		if err := f.write(deletes); err != nil {
			return err
		}
		start = end
	}
	f.allocated = keep
	// The store keeps chunks whole, so the bytes the old length left in the
	// last surviving chunk have to go with the rest: a file that shrinks and
	// grows again reads zeros there, not what used to be there.
	if rem := size % v.chunkSize; rem != 0 && keep > 0 {
		index := uint32(keep - 1)
		ch, err := f.chunkLocked(index)
		if err != nil {
			return err
		}
		clear(ch.data[rem:])
		ch.dirty = true
		f.dirty[index] = true
	}
	f.size = size
	return nil
}

func (f *file) Sync(flag sqlite3vfs.SyncType) error {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return sqlite3vfs.IOError
	}
	if err := f.writeChunksLocked(); err != nil {
		return err
	}
	// The barrier: everything this file (and every other file written before
	// it) has handed to the store is durable once this returns. This is the
	// point SQLite is entitled to assume; SQLITE_SYNC_DATAONLY asks for less
	// than that, and getting more is never a problem.
	if err := v.store.Flush(); err != nil {
		return v.ioError("flush after sync of "+f.name, err)
	}
	return nil
}

func (f *file) FileSize() (int64, error) {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return 0, sqlite3vfs.IOError
	}
	return f.size, nil
}

func (f *file) Lock(want sqlite3vfs.LockType) error {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return sqlite3vfs.IOError
	}
	if want <= f.lock {
		// Locking only ever tightens, so this is already held.
		return nil
	}
	if v.lockHeld(f, want) {
		// SQLite retries a busy lock until its busy timeout runs out, which is
		// how two files on one name in one process serialize.
		return sqlite3vfs.BusyError
	}
	f.lock = want
	return nil
}

func (f *file) Unlock(level sqlite3vfs.LockType) error {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return sqlite3vfs.IOError
	}
	if level != sqlite3vfs.LockNone && level != sqlite3vfs.LockShared {
		return v.ioError("unlock", fmt.Errorf("unlock to %v in %s", level, f.name))
	}
	if level < f.lock {
		f.lock = level
	}
	return nil
}

func (f *file) CheckReservedLock() (bool, error) {
	v := f.vfs
	v.mu.Lock()
	defer v.mu.Unlock()
	if f.closed {
		return false, sqlite3vfs.IOError
	}
	return v.reservedElsewhere(f), nil
}

// SectorSize reports 0: the store makes no promises about tearing, so SQLite
// should assume the worst and journal everything it wants protected.
func (f *file) SectorSize() int64 { return 0 }

// DeviceCharacteristics reports no special powers, for the same reason.
func (f *file) DeviceCharacteristics() sqlite3vfs.DeviceCharacteristic { return 0 }

// writeChunksLocked sends every chunk written since the last call to the store
// as one batch, along with the file's length when that changed too.
func (f *file) writeChunksLocked() error {
	v := f.vfs
	if f.readOnly {
		return nil
	}
	indexes := make([]uint32, 0, len(f.dirty))
	for index := range f.dirty {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })

	mutations := make([]Mutation, 0, len(indexes)+1)
	for _, index := range indexes {
		ch, ok := v.cache.get(cacheKey{f.name, index})
		if !ok {
			// A dirty chunk is always cached; losing one means a bug here, and
			// writing what is left would leave a half-updated file behind.
			return v.ioError("sync of "+f.name, fmt.Errorf("chunk %d of %s is not in the cache", index, f.name))
		}
		mutations = append(mutations, Mutation{Key: chunkKey(f.name, index), Value: ch.data})
	}
	length := f.size != f.storedSize
	if length {
		mutations = append(mutations, Mutation{Key: lengthKey(f.name), Value: encodeLength(f.size)})
	}
	if len(mutations) == 0 {
		return nil
	}
	if err := f.write(mutations); err != nil {
		return err
	}
	for _, index := range indexes {
		if ch, ok := v.cache.get(cacheKey{f.name, index}); ok {
			ch.dirty = false
		}
		delete(f.dirty, index)
	}
	if length {
		f.storedSize = f.size
	}
	return nil
}

// chunkLocked returns the chunk at index, reading it from the store (or
// synthesizing a hole) and caching it when it is not in memory already.
func (f *file) chunkLocked(index uint32) (*chunk, error) {
	v := f.vfs
	key := cacheKey{f.name, index}
	if ch, ok := v.cache.get(key); ok {
		return ch, nil
	}
	data := make([]byte, v.chunkSize)
	if index < f.allocated {
		value, ok, err := v.store.Get(chunkKey(f.name, index))
		if err != nil {
			return nil, v.ioError(fmt.Sprintf("reading chunk %d of %s", index, f.name), err)
		}
		if ok {
			if int64(len(value)) != v.chunkSize {
				return nil, v.ioError(fmt.Sprintf("reading chunk %d of %s", index, f.name),
					fmt.Errorf("stored chunk is %d bytes, want %d", len(value), v.chunkSize))
			}
			copy(data, value)
		}
	}
	ch := &chunk{key: key, data: data, file: f}
	if err := v.cache.put(ch); err != nil {
		return nil, v.ioError(fmt.Sprintf("caching chunk %d of %s", index, f.name), err)
	}
	return ch, nil
}

func (f *file) write(mutations []Mutation) error {
	if len(mutations) == 0 {
		return nil
	}
	if err := f.vfs.store.Write(mutations); err != nil {
		return f.vfs.ioError("writing "+f.name, err)
	}
	return nil
}

// writeBatchLimit caps one store batch. A file being truncated writes its
// surviving chunks in further batches.
const writeBatchLimit = 4096
