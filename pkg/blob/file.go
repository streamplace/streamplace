package blob

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// FileStore is a blob.Store backed by the local filesystem rooted at a
// configured directory. Writes go to a hidden staging area (root +
// stagingDir) and are atomically renamed to their final key on Complete.
type FileStore struct {
	root       string
	stagingDir string
}

// stagingDir is the per-store hidden subdir under root that holds
// in-progress writes. The leading dot keeps it out of plain blob key
// space — keys with a leading dot are technically allowed, but uncommon.
const stagingDir = ".staging"

// NewFileStore returns a FileStore rooted at root. The root and the
// staging directory inside it are created on construction if they
// don't already exist.
func NewFileStore(root string) (*FileStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("blob: abs root %q: %w", root, err)
	}
	staging := filepath.Join(abs, stagingDir)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, fmt.Errorf("blob: mkdir staging %q: %w", staging, err)
	}
	return &FileStore{root: abs, stagingDir: staging}, nil
}

// Root returns the absolute path of the FileStore's root, useful for
// callers that need to construct keys from absolute paths.
func (s *FileStore) Root() string { return s.root }

func (s *FileStore) URL(key string) string {
	return "file://" + s.absPath(key)
}

func (s *FileStore) absPath(key string) string {
	return filepath.Join(s.root, filepath.FromSlash(key))
}

func (s *FileStore) Open(ctx context.Context, key string) (Reader, error) {
	path := s.absPath(key)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("blob file open %s: %w", path, err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("blob file stat %s: %w", path, err)
	}
	return &fileReader{f: f, size: st.Size()}, nil
}

func (s *FileStore) NewWriter(ctx context.Context, key, contentType string) (Writer, error) {
	if err := s.ensureKeyDir(key); err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("blob file: generate staging id: %w", err)
	}
	staging := filepath.Join(s.stagingDir, id.String())
	f, err := os.OpenFile(staging, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("blob file: open staging %s: %w", staging, err)
	}
	return &fileWriter{
		f:        f,
		staging:  staging,
		finalKey: key,
		finalAbs: s.absPath(key),
	}, nil
}

// ensureKeyDir creates any parent directories implied by key so the
// final rename has somewhere to land.
func (s *FileStore) ensureKeyDir(key string) error {
	dir := filepath.Dir(s.absPath(key))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("blob file: mkdir %s: %w", dir, err)
	}
	return nil
}

func (s *FileStore) Move(ctx context.Context, srcKey, dstKey string) error {
	if err := s.ensureKeyDir(dstKey); err != nil {
		return err
	}
	src, dst := s.absPath(srcKey), s.absPath(dstKey)
	err := os.Rename(src, dst)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Idempotency: if src is gone, maybe a previous Move
			// already succeeded. Check whether the destination exists;
			// if it does, we're in the post-condition the caller wants.
			if _, statErr := os.Stat(dst); statErr == nil {
				return nil
			}
			return fmt.Errorf("%w: %s", ErrNotFound, src)
		}
		return fmt.Errorf("blob file rename %s -> %s: %w", src, dst, err)
	}
	return nil
}

func (s *FileStore) List(ctx context.Context, prefix string) ([]string, error) {
	// Walk anchored at the longest directory whose path is fully fixed
	// by prefix; the trailing component (which may be a partial
	// filename) becomes a string-prefix filter on every visited key.
	// This matches the S3 semantics: "foo/bar" matches both
	// "foo/bar.json" and "foo/bar/baz".
	slashIdx := strings.LastIndex(prefix, "/")
	walkRel := ""
	if slashIdx >= 0 {
		walkRel = prefix[:slashIdx]
	}
	walkRoot := s.absPath(walkRel)
	var out []string
	err := filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return filepath.SkipAll
			}
			return walkErr
		}
		if d.IsDir() {
			// Skip the per-store staging area; partial writes shouldn't
			// appear in callers' listings.
			if path == s.stagingDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if !strings.HasPrefix(key, prefix) {
			return nil
		}
		out = append(out, key)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("blob file list %s: %w", prefix, err)
	}
	return out, nil
}

func (s *FileStore) Delete(ctx context.Context, key string) error {
	path := s.absPath(key)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("blob file delete %s: %w", path, err)
	}
	return nil
}

// ParseLocation accepts either a plain absolute path under root or a
// file:// URL, and returns the corresponding key. Locations outside
// root are rejected with ok=false.
func (s *FileStore) ParseLocation(location string) (string, bool) {
	abs := location
	if strings.HasPrefix(location, "file://") {
		u, err := url.Parse(location)
		if err != nil {
			return "", false
		}
		abs = u.Path
	}
	if !filepath.IsAbs(abs) {
		// Treat as already-relative key; just sanity-check it doesn't
		// escape via "..".
		clean := filepath.Clean(abs)
		if strings.HasPrefix(clean, "..") {
			return "", false
		}
		return filepath.ToSlash(clean), true
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// fileReader implements blob.Reader over an open *os.File.
type fileReader struct {
	f    *os.File
	size int64
}

func (r *fileReader) ReadAt(p []byte, off int64) (int, error) { return r.f.ReadAt(p, off) }
func (r *fileReader) Close() error                            { return r.f.Close() }
func (r *fileReader) Size() int64                             { return r.size }

// fileWriter implements blob.Writer atop a staging temp file that gets
// renamed on Complete.
type fileWriter struct {
	f         *os.File
	staging   string
	finalKey  string
	finalAbs  string
	finalized bool
}

func (w *fileWriter) Write(p []byte) (int, error) {
	if w.finalized {
		return 0, fmt.Errorf("blob file: write after Complete/Abort on %s", w.finalKey)
	}
	return w.f.Write(p)
}

func (w *fileWriter) Complete() error {
	if w.finalized {
		return fmt.Errorf("blob file: Complete called twice on %s", w.finalKey)
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("blob file: fsync %s: %w", w.staging, err)
	}
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("blob file: close staging %s: %w", w.staging, err)
	}
	if err := os.Rename(w.staging, w.finalAbs); err != nil {
		// Best-effort cleanup so an aborted rename doesn't leave the
		// staging file lying around.
		_ = os.Remove(w.staging)
		return fmt.Errorf("blob file: rename staging -> %s: %w", w.finalAbs, err)
	}
	w.finalized = true
	return nil
}

func (w *fileWriter) Abort() error {
	if w.finalized {
		return nil
	}
	w.finalized = true
	_ = w.f.Close()
	if err := os.Remove(w.staging); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("blob file: remove staging %s: %w", w.staging, err)
	}
	return nil
}

func (w *fileWriter) Close() error { return w.Abort() }

// Compile-time interface checks.
var (
	_ Store  = (*FileStore)(nil)
	_ Reader = (*fileReader)(nil)
	_ Writer = (*fileWriter)(nil)
)
