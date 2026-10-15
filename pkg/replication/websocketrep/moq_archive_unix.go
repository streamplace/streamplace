//go:build !windows

package websocketrep

import (
	"os"
	"path/filepath"
)

func commitMoqArchive(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(to))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
