package branding

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// A brand directory is a bundle unzipped: branding.yaml beside the files it
// names. It is what an operator keeps in git and what the app build reads
// (SP_BRAND_DIR; see brand/README.md), so the same directory brands the
// built apps and, imported, the running node.

// IsDir reports whether path is a brand directory rather than a zip.
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// ZipDir packs a brand directory as a bundle. Only the directory's own
// regular files are included (no subdirectories: brand/custom/ lives inside
// the default brand), and only if the whole stays under MaxBundleSize.
func ZipDir(dir string) ([]byte, error) {
	if _, err := os.Stat(filepath.Join(dir, yamlName)); err != nil {
		return nil, fmt.Errorf("%s has no %s", dir, yamlName)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	total := 0
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		bs, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		total += len(bs)
		if total > MaxBundleSize {
			return nil, fmt.Errorf("%s is larger than a bundle may be (%d bytes)", dir, MaxBundleSize)
		}
		w, err := zw.Create(e.Name())
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(bs); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReadBundle reads a bundle from a zip file or a brand directory.
func ReadBundle(path string) ([]byte, error) {
	if IsDir(path) {
		return ZipDir(path)
	}
	return os.ReadFile(path)
}

// UnzipDir writes a bundle out as a brand directory, creating dir. Only
// flat file names are written; anything else in the zip is ignored.
func UnzipDir(zipBytes []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("not a zip file: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		name := filepath.Base(filepath.Clean(f.Name))
		if f.FileInfo().IsDir() || name != f.Name || strings.HasPrefix(name, ".") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		bs, err := io.ReadAll(io.LimitReader(rc, MaxBundleSize+1))
		rc.Close()
		if err != nil {
			return err
		}
		if len(bs) > MaxBundleSize {
			return fmt.Errorf("%s is too large", name)
		}
		if err := os.WriteFile(filepath.Join(dir, name), bs, 0o644); err != nil {
			return err
		}
	}
	return nil
}
