package s3

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// patternBackend is a fake io.ReaderAt whose byte at offset o is o%251, so
// any misalignment in the cache surfaces as a content mismatch.
type patternBackend struct{ size int64 }

func (p *patternBackend) ReadAt(b []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset %d", off)
	}
	if off >= p.size {
		return 0, io.EOF
	}
	n := len(b)
	if int64(off)+int64(n) > p.size {
		n = int(p.size - off)
	}
	for i := 0; i < n; i++ {
		b[i] = byte((off + int64(i)) % 251)
	}
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

// TestCachingReaderAt checks the cache returns exactly what the backend
// would, across block boundaries, re-reads (post-eviction), and the tail —
// with a deliberately tiny cache (4 × 64 KB) to force eviction/redownload.
func TestCachingReaderAt(t *testing.T) {
	const size = 10*1024*1024 + 12345
	backend := &patternBackend{size: size}
	c, err := NewCachingReaderAt(backend, size, 64*1024, 4)
	require.NoError(t, err)

	check := func(off, n int64) {
		t.Helper()
		got := make([]byte, n)
		gn, gerr := c.ReadAt(got, off)
		want := make([]byte, n)
		wn, werr := readAtFull(backend, want, off)
		require.Equal(t, wn, gn, "read count mismatch at off=%d n=%d", off, n)
		require.Equal(t, want[:wn], got[:gn], "bytes mismatch at off=%d", off)
		require.Equal(t, errors.Is(werr, io.EOF), errors.Is(gerr, io.EOF), "EOF parity at off=%d", off)
	}

	check(0, 100*1024)          // sequential across many blocks
	check(5*1024*1024, 64*1024) // jump
	check(123, 3)               // tiny read
	check(64*1024-10, 40)       // spans a block boundary
	check(0, 100*1024)          // re-read early region (likely evicted)
	check(size-10, 100)         // read past the tail

	// Reading entirely past EOF yields (0, io.EOF).
	n, err := c.ReadAt(make([]byte, 16), size)
	require.Equal(t, 0, n)
	require.ErrorIs(t, err, io.EOF)

	// A whole-file read through the cache equals a direct backend read.
	full := make([]byte, size)
	fn, ferr := readAtFull(c, full, 0)
	require.NoError(t, ferr)
	require.Equal(t, size, fn)
	direct := make([]byte, size)
	_, _ = readAtFull(backend, direct, 0)
	require.Equal(t, direct, full)
}

type readOp struct{ pos, size int64 }

func parseTrace(path string) ([]readOp, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var ops []readOp
	var size int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var pos, sz int64
		if _, err := fmt.Sscanf(strings.TrimSpace(sc.Text()), "readAt pos=%d size=%d", &pos, &sz); err != nil {
			continue
		}
		ops = append(ops, readOp{pos, sz})
		if pos+sz > size {
			size = pos + sz
		}
	}
	return ops, size, sc.Err()
}

// baselineGets counts what today's ReaderAt would do: a fresh ranged
// GetObject for the first read and every subsequent non-sequential read.
func baselineGets(ops []readOp) int64 {
	var gets, expected int64
	first := true
	for _, op := range ops {
		if first || op.pos != expected {
			gets++
		}
		expected = op.pos + op.size
		first = false
	}
	return gets
}

// TestCachingReaderAtSim replays a real RandomAccessSrcBin read trace
// (captured as "readAt pos=N size=M" lines) through the cache against a
// fake backend and reports cache behavior for a few configs. Set
// SEEK_TRACE=/path/to/seek-example to run it.
func TestCachingReaderAtSim(t *testing.T) {
	path := os.Getenv("SEEK_TRACE")
	if path == "" {
		t.Skip("set SEEK_TRACE=/path/to/trace to run the cache simulation")
	}
	ops, size, err := parseTrace(path)
	require.NoError(t, err)
	require.NotEmpty(t, ops, "no 'readAt pos=.. size=..' lines parsed from %s", path)

	const MB = 1024 * 1024
	var requested int64
	for _, op := range ops {
		requested += op.size
	}
	t.Logf("trace: %d reads, %.2f MB requested, object size %.2f GB",
		len(ops), float64(requested)/MB, float64(size)/1e9)
	t.Logf("baseline (current ReaderAt): %d GetObject round-trips", baselineGets(ops))
	t.Logf("--- LRU block cache ---")

	configs := []struct {
		blockSize int64
		maxBlocks int
	}{
		{16 * MB, 4}, {16 * MB, 10}, {16 * MB, 20},
		{8 * MB, 10}, {4 * MB, 10}, {1 * MB, 16},
	}
	for _, cfg := range configs {
		backend := &patternBackend{size: size}
		c, err := NewCachingReaderAt(backend, size, cfg.blockSize, cfg.maxBlocks)
		require.NoError(t, err)
		for _, op := range ops {
			_, _ = c.ReadAt(make([]byte, op.size), op.pos)
		}
		s := c.Stats()
		hitRate := 100 * float64(s.Hits) / float64(s.BlockTouches)
		amp := float64(s.BackendBytes) / float64(s.BytesRequested)
		t.Logf("block=%-4s cache=%-2d (%4d MB) -> backendGETs=%-5d backendMB=%-8.1f hit=%.1f%% cold=%-3d redownloads=%-4d evictions=%-4d amp=%.1fx",
			human(cfg.blockSize), cfg.maxBlocks, int(cfg.blockSize/MB)*cfg.maxBlocks,
			s.BackendReads, float64(s.BackendBytes)/MB, hitRate, s.ColdMisses, s.Redownloads, s.Evictions, amp)
	}
}

func human(b int64) string {
	const MB = 1024 * 1024
	if b%MB == 0 {
		return fmt.Sprintf("%dMB", b/MB)
	}
	return fmt.Sprintf("%dKB", b/1024)
}
