//go:build linux && cgo && stationslate

package stationdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/mattn/go-sqlite3"
	"github.com/psanford/sqlite3vfs"
	"golang.org/x/sync/errgroup"
)

const (
	pagerVFSName = "streamplace-stationdb"
	maxPagerSize = int64(maxPages) * pageSize
)

var (
	zeroPage         pageData
	stationVFS       pagerVFS
	registerPagerVFS = sync.OnceValue(func() error {
		return sqlite3vfs.RegisterVFS(pagerVFSName, &stationVFS)
	})
)

// pending holds only files that have not yet been claimed by SQLite's xOpen.
type pagerVFS struct {
	sequence atomic.Uint64
	pending  sync.Map
}

func (v *pagerVFS) Open(name string, flags sqlite3vfs.OpenFlag) (sqlite3vfs.File, sqlite3vfs.OpenFlag, error) {
	const main = sqlite3vfs.OpenMainDB | sqlite3vfs.OpenReadWrite
	if flags&main == main {
		if file, ok := v.pending.LoadAndDelete(name); ok {
			return file.(*pager), sqlite3vfs.OpenReadWrite, nil
		}
	}
	// No fallback to the OS VFS, even for auxiliary files.
	return nil, 0, sqlite3vfs.CantOpenError
}

func (*pagerVFS) FullPathname(name string) string                    { return name }
func (*pagerVFS) Access(string, sqlite3vfs.AccessFlag) (bool, error) { return false, nil }
func (*pagerVFS) Delete(string, bool) error                          { return sqlite3vfs.IOError }

func openPager(ctx context.Context, base *snapshot) (*sqlite3.SQLiteConn, *pager, error) {
	if err := registerPagerVFS(); err != nil {
		return nil, nil, err
	}
	p := newPager(base)
	name := "stationdb-" + strconv.FormatUint(stationVFS.sequence.Add(1), 10)
	stationVFS.pending.Store(name, p)
	defer stationVFS.pending.Delete(name)

	raw, err := (&sqlite3.SQLiteDriver{}).Open("file:" + name + "?vfs=" + pagerVFSName + "&mode=rwc&cache=private&_journal_mode=MEMORY")
	if err != nil {
		return nil, nil, err
	}
	db := raw.(*sqlite3.SQLiteConn)
	if _, err := db.ExecContext(ctx, "PRAGMA temp_store=MEMORY; PRAGMA page_size="+strconv.Itoa(pageSize), nil); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return db, p, nil
}

// A pager is owned by one SQLite connection. Only dirty holds mutable pages.
type pager struct {
	base      *snapshot
	dirty     map[int]*pageData
	size      int64
	basePages int // Full base pages not discarded by a truncate.
}

func newPager(base *snapshot) *pager {
	p := &pager{}
	p.reset(base)
	return p
}

func (p *pager) reset(base *snapshot) {
	*p = pager{base: base, basePages: len(base.pages), size: int64(len(base.pages)) * pageSize}
}

func (p *pager) modified() bool {
	return len(p.dirty) != 0 || p.basePages != len(p.base.pages) || p.size != int64(len(p.base.pages))*pageSize
}

func (p *pager) page(i int) *pageData {
	if data := p.dirty[i]; data != nil {
		return data
	}
	if i < p.basePages {
		return p.base.pages[i].data
	}
	return &zeroPage
}

func (p *pager) writable(i int, overwrite bool) *pageData {
	if data := p.dirty[i]; data != nil {
		return data
	}
	data := new(pageData)
	if !overwrite {
		*data = *p.page(i)
	}
	if p.dirty == nil {
		p.dirty = make(map[int]*pageData)
	}
	p.dirty[i] = data
	return data
}

func (p *pager) ReadAt(dst []byte, off int64) (int, error) {
	if off < 0 {
		return 0, sqlite3vfs.IOErrorRead
	}
	limit := int(min(int64(len(dst)), max(0, p.size-off)))
	for n := 0; n < limit; {
		i, start := int(off/pageSize), int(off%pageSize)
		count := copy(dst[n:limit], p.page(i)[start:])
		n += count
		off += int64(count)
	}
	if limit < len(dst) {
		return limit, io.EOF
	}
	return limit, nil
}

func (p *pager) WriteAt(src []byte, off int64) (int, error) {
	if off < 0 || off > maxPagerSize || int64(len(src)) > maxPagerSize-off {
		return 0, sqlite3vfs.IOErrorWrite
	}
	if len(src) == 0 {
		return 0, nil
	}
	for n := 0; n < len(src); {
		i, start := int(off/pageSize), int(off%pageSize)
		count := min(len(src)-n, pageSize-start)
		data := p.writable(i, count == pageSize)
		copy(data[start:], src[n:n+count])
		n += count
		off += int64(count)
	}
	p.size = max(p.size, off)
	return len(src), nil
}

func (p *pager) Truncate(size int64) error {
	if size < 0 || size > maxPagerSize {
		return sqlite3vfs.IOError
	}
	if size < p.size {
		i, end := int(size/pageSize), int(size%pageSize)
		// Preserve the prefix of the last partial page; never resurrect its tail.
		if end != 0 && (i < p.basePages || p.dirty[i] != nil) {
			clear(p.writable(i, false)[end:])
		}
		p.basePages = min(p.basePages, i)
		if end != 0 {
			i++
		}
		for j := range p.dirty {
			if j >= i {
				delete(p.dirty, j)
			}
		}
	}
	p.size = size
	return nil
}

func (p *pager) FileSize() (int64, error) { return p.size, nil }
func (*pager) Close() error               { return nil }

// SQLite handles rollback in memory; only the SQL driver may publish a commit.
func (*pager) Sync(sqlite3vfs.SyncType) error { return nil }

// This file has one SQLite owner; inter-connection arbitration is the head CAS.
func (*pager) Lock(sqlite3vfs.LockType) error                         { return nil }
func (*pager) Unlock(sqlite3vfs.LockType) error                       { return nil }
func (*pager) CheckReservedLock() (bool, error)                       { return false, nil }
func (*pager) SectorSize() int64                                      { return pageSize }
func (*pager) DeviceCharacteristics() sqlite3vfs.DeviceCharacteristic { return 0 }

func (s *Store) publish(ctx context.Context, p *pager) (*snapshot, error) {
	base := p.base
	if !p.modified() {
		return base, nil
	}
	n := int(p.size / pageSize)
	if p.size%pageSize != 0 || (n != 0 && !validHeader(p.page(0))) {
		return nil, errors.New("invalid station SQLite image")
	}
	ctx, cancel := context.WithTimeout(ctx, objectTimeout)
	defer cancel()
	var next *snapshot
	newSnapshot := func() *snapshot {
		pages := make([]storedPage, n)
		copy(pages, base.pages)
		return &snapshot{pages: pages}
	}
	if n != len(base.pages) {
		next = newSnapshot()
	}
	group, uploadCtx := errgroup.WithContext(ctx)
	group.SetLimit(pageWorkers)
	upload := func(i int, data *pageData) {
		if uploadCtx.Err() != nil {
			return
		}
		if i < len(base.pages) {
			old := base.pages[i].data
			if data == old || *data == *old {
				return
			}
		}
		if next == nil {
			next = newSnapshot()
		}
		var id pageID
		rand.Read(id[:])
		next.pages[i] = storedPage{id: id, data: data}
		name := s.prefix + "pages/" + hex.EncodeToString(id[:])
		group.Go(func() error {
			_, err := s.put(uploadCtx, name, data[:], "")
			return err
		})
	}
	for i, data := range p.dirty {
		upload(i, data)
	}
	// Holes past the retained base are zero, including truncated pages that regrew.
	for i := p.basePages; i < n && uploadCtx.Err() == nil; i++ {
		if p.dirty[i] == nil {
			upload(i, &zeroPage)
		}
	}
	if err := group.Wait(); err != nil {
		return nil, fmt.Errorf("upload station database pages: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if next == nil {
		return base, nil
	}

	const idSize = len(pageID{})
	index := make([]byte, n*idSize)
	for i := range next.pages {
		copy(index[i*idSize:], next.pages[i].id[:])
	}

	etag, err := s.put(ctx, s.prefix+"head", index, base.etag)
	if errors.Is(err, errCAS) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommitUnknown, err)
	}
	next.etag = etag
	s.latest.CompareAndSwap(base, next)
	return next, nil
}
