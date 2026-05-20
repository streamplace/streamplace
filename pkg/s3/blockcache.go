package s3

import (
	"container/list"
	"errors"
	"fmt"
	"io"
	"sync"
)

// CachingReaderAt wraps a backend io.ReaderAt with a bounded LRU cache of
// fixed-size blocks. It exists for the demuxer-over-S3 access pattern:
// qtdemux seeks all over a large MP4, and the bare S3 ReaderAt turns every
// non-sequential read into a fresh ranged GetObject. Serving reads out of
// cached 16 MB blocks collapses those thousands of round-trips into a
// handful of full-block fetches, while keeping memory bounded (maxBlocks *
// blockSize) so we never have to download a whole (potentially many-GB)
// upload up front.
//
// Reads are serialized by a single mutex, matching the bare ReaderAt and
// the single-threaded gstreamer streaming thread that drives it. The
// backend only ever sees aligned, full-block ReadAt calls.
type CachingReaderAt struct {
	backend   io.ReaderAt
	size      int64
	blockSize int64
	maxBlocks int

	mu          sync.Mutex
	blocks      map[int64]*list.Element // block index -> LRU element
	lru         *list.List              // front = most-recently-used
	everFetched map[int64]bool          // blocks fetched at least once (redownload accounting)
	stats       CacheStats
}

// Defaults chosen from replaying real demuxer traces against the
// simulation (see TestCachingReaderAtSim): 16 MB blocks minimize round
// trips (a full 1.4 GB read went from ~120k ranged GETs to ~91), and 10
// cached blocks (160 MB) leaves headroom for files whose tracks live in
// separate regions, where the demuxer keeps multiple read-fronts alive.
const (
	DefaultCacheBlockSize = 16 * 1024 * 1024
	DefaultCacheBlocks    = 10
)

type cacheEntry struct {
	index int64
	data  []byte
}

// CacheStats is a snapshot of the cache's behavior, used both for the
// offline simulation and (eventually) live metrics.
type CacheStats struct {
	Reads          int64 // ReadAt calls served
	BytesRequested int64 // sum of len(p) across ReadAt calls (clamped to size)
	BlockTouches   int64 // block-level accesses (a read may touch several)
	Hits           int64 // block touches served from cache
	Misses         int64 // block touches that required a backend fetch
	ColdMisses     int64 // misses for a block never fetched before
	Redownloads    int64 // misses for a block that was fetched then evicted
	Evictions      int64 // blocks dropped from the cache
	BackendReads   int64 // ReadAt calls issued to the backend (== Misses)
	BackendBytes   int64 // bytes pulled from the backend
}

// NewCachingReaderAt wraps backend (whose total length is size) with an LRU
// block cache of maxBlocks blocks of blockSize bytes each.
func NewCachingReaderAt(backend io.ReaderAt, size, blockSize int64, maxBlocks int) (*CachingReaderAt, error) {
	if blockSize <= 0 {
		return nil, fmt.Errorf("blockSize must be positive, got %d", blockSize)
	}
	if maxBlocks <= 0 {
		return nil, fmt.Errorf("maxBlocks must be positive, got %d", maxBlocks)
	}
	return &CachingReaderAt{
		backend:     backend,
		size:        size,
		blockSize:   blockSize,
		maxBlocks:   maxBlocks,
		blocks:      make(map[int64]*list.Element),
		lru:         list.New(),
		everFetched: make(map[int64]bool),
	}, nil
}

// ReadAt implements io.ReaderAt, serving from cached blocks and fetching
// (full, aligned) blocks from the backend on a miss. Returns io.EOF when a
// read runs past the end of the object, matching io.ReaderAt semantics.
func (c *CachingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset %d", off)
	}
	if off >= c.size {
		return 0, io.EOF
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats.Reads++
	want := len(p)
	if int64(off)+int64(want) > c.size {
		want = int(c.size - off)
	}
	c.stats.BytesRequested += int64(want)

	copied := 0
	for copied < want {
		readOff := off + int64(copied)
		idx := readOff / c.blockSize
		block, err := c.getBlock(idx)
		if err != nil {
			return copied, err
		}
		within := int(readOff - idx*c.blockSize)
		n := copy(p[copied:want], block[within:])
		copied += n
	}
	if copied < len(p) {
		// Caller asked for more than the object holds.
		return copied, io.EOF
	}
	return copied, nil
}

// getBlock returns block idx, fetching it from the backend on a miss and
// updating LRU/stats. Caller must hold c.mu.
func (c *CachingReaderAt) getBlock(idx int64) ([]byte, error) {
	c.stats.BlockTouches++
	if el, ok := c.blocks[idx]; ok {
		c.lru.MoveToFront(el)
		c.stats.Hits++
		return el.Value.(*cacheEntry).data, nil
	}

	c.stats.Misses++
	if c.everFetched[idx] {
		c.stats.Redownloads++
	} else {
		c.stats.ColdMisses++
		c.everFetched[idx] = true
	}

	start := idx * c.blockSize
	n := c.blockSize
	if start+n > c.size {
		n = c.size - start
	}
	buf := make([]byte, n)
	got, err := readAtFull(c.backend, buf, start)
	c.stats.BackendReads++
	c.stats.BackendBytes += int64(got)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("cache backend read block %d (offset %d): %w", idx, start, err)
	}
	buf = buf[:got]

	el := c.lru.PushFront(&cacheEntry{index: idx, data: buf})
	c.blocks[idx] = el
	if c.lru.Len() > c.maxBlocks {
		back := c.lru.Back()
		evicted := back.Value.(*cacheEntry)
		c.lru.Remove(back)
		delete(c.blocks, evicted.index)
		c.stats.Evictions++
	}
	return buf, nil
}

// Size returns the underlying object's size, satisfying blob.Reader.
func (c *CachingReaderAt) Size() int64 { return c.size }

// Close closes the backend if it owns resources (e.g. the S3 ReaderAt's
// open GetObject body), satisfying io.Closer / blob.Reader.
func (c *CachingReaderAt) Close() error {
	if closer, ok := c.backend.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Stats returns a copy of the current cache statistics.
func (c *CachingReaderAt) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// readAtFull reads len(buf) bytes via repeated ReadAt, tolerating short
// reads from backends that don't fill the buffer in one call. Returns the
// number of bytes read; io.EOF if the object ended first.
func readAtFull(r io.ReaderAt, buf []byte, off int64) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.ReadAt(buf[total:], off+int64(total))
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
