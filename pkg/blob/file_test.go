package blob

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileStore_WriteCompleteReadDeleteMove(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)

	const key = "vod/abc/def.fmp4"
	payload := []byte("hello blob storage world")

	w, err := store.NewWriter(ctx, key, "video/mp4")
	require.NoError(t, err)
	n, err := w.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	// Before Complete, the final key is not visible.
	_, err = store.Open(ctx, key)
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, w.Complete())
	// Double-complete is rejected.
	require.Error(t, w.Complete())
	// Close after Complete is a no-op.
	require.NoError(t, w.Close())

	// Random-access read.
	r, err := store.Open(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(len(payload)), r.Size())

	buf := make([]byte, 5)
	got, err := r.ReadAt(buf, 6)
	require.NoError(t, err)
	require.Equal(t, 5, got)
	require.Equal(t, "blob ", string(buf))
	require.NoError(t, r.Close())

	// Move atomically renames; the source key disappears.
	const dstKey = "vod/xyz/moved.fmp4"
	require.NoError(t, store.Move(ctx, key, dstKey))
	_, err = store.Open(ctx, key)
	require.ErrorIs(t, err, ErrNotFound)
	r2, err := store.Open(ctx, dstKey)
	require.NoError(t, err)
	require.NoError(t, r2.Close())

	// Move is idempotent: re-running with src already gone returns nil
	// as long as the destination still exists.
	require.NoError(t, store.Move(ctx, key, dstKey))

	// Delete removes; Open afterwards is ErrNotFound.
	require.NoError(t, store.Delete(ctx, dstKey))
	_, err = store.Open(ctx, dstKey)
	require.ErrorIs(t, err, ErrNotFound)
	// Delete of missing blob is also nil.
	require.NoError(t, store.Delete(ctx, dstKey))
}

func TestFileStore_AbortLeavesNoFinalBlob(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)

	const key = "vod/aborted.fmp4"
	w, err := store.NewWriter(ctx, key, "")
	require.NoError(t, err)
	_, err = w.Write([]byte("incoming bytes"))
	require.NoError(t, err)
	require.NoError(t, w.Abort())

	_, err = store.Open(ctx, key)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestFileStore_CloseWithoutCompleteAborts(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)

	const key = "vod/forgotten.fmp4"
	w, err := store.NewWriter(ctx, key, "")
	require.NoError(t, err)
	_, err = w.Write([]byte("oops never finalized"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	_, err = store.Open(ctx, key)
	require.ErrorIs(t, err, ErrNotFound)

	// And the staging directory should have no residue.
	stagingEntries, err := os.ReadDir(filepath.Join(store.Root(), stagingDir))
	require.NoError(t, err)
	require.Empty(t, stagingEntries, "staging dir should be empty after Close-without-Complete")
}

func TestFileStore_ParseLocation(t *testing.T) {
	root := t.TempDir()
	store, err := NewFileStore(root)
	require.NoError(t, err)

	t.Run("absolute path inside root", func(t *testing.T) {
		key, ok := store.ParseLocation(filepath.Join(root, "uploads/abc"))
		require.True(t, ok)
		require.Equal(t, "uploads/abc", key)
	})
	t.Run("file:// URL inside root", func(t *testing.T) {
		key, ok := store.ParseLocation("file://" + filepath.Join(root, "uploads/abc"))
		require.True(t, ok)
		require.Equal(t, "uploads/abc", key)
	})
	t.Run("relative key", func(t *testing.T) {
		key, ok := store.ParseLocation("uploads/abc")
		require.True(t, ok)
		require.Equal(t, "uploads/abc", key)
	})
	t.Run("absolute path outside root", func(t *testing.T) {
		_, ok := store.ParseLocation("/etc/passwd")
		require.False(t, ok)
	})
	t.Run("relative path attempting escape", func(t *testing.T) {
		_, ok := store.ParseLocation("../etc/passwd")
		require.False(t, ok)
	})
}

func TestFileStore_URL(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)
	url := store.URL("vod/abc.fmp4")
	require.Contains(t, url, "file://")
	require.Contains(t, url, "vod/abc.fmp4")
}

func TestFileStore_ConcurrentReads(t *testing.T) {
	// Multiple Readers against the same blob should be able to operate
	// independently, since each Open returns a fresh *os.File.
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)

	const key = "concurrent.bin"
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	w, err := store.NewWriter(ctx, key, "")
	require.NoError(t, err)
	_, err = w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Complete())

	r1, err := store.Open(ctx, key)
	require.NoError(t, err)
	defer r1.Close()
	r2, err := store.Open(ctx, key)
	require.NoError(t, err)
	defer r2.Close()

	buf1 := make([]byte, 1024)
	buf2 := make([]byte, 512)

	n1, err := r1.ReadAt(buf1, 100)
	require.NoError(t, err)
	require.Equal(t, 1024, n1)

	n2, err := r2.ReadAt(buf2, 3000)
	require.NoError(t, err)
	require.Equal(t, 512, n2)

	require.Equal(t, payload[100:1124], buf1)
	require.Equal(t, payload[3000:3512], buf2)
}

func TestFileStore_OpenMissingReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)

	_, err = store.Open(ctx, "does/not/exist")
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestFileStore_WriterCloseAndAbortIdempotent(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	require.NoError(t, err)

	w, err := store.NewWriter(ctx, "x", "")
	require.NoError(t, err)
	_, err = io.WriteString(w, "hi")
	require.NoError(t, err)
	require.NoError(t, w.Abort())
	// Second Abort is a no-op (returns nil).
	require.NoError(t, w.Abort())
	require.NoError(t, w.Close())
}
