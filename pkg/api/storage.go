package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
)

type StreamplaceCertStorage struct {
	Path string

	// locks keeps track of current locks for this storage instance
	locks   map[string]*fileLock
	locksMu sync.Mutex
}

type fileLock struct {
	path     string
	lockFile *os.File
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewStreamplaceCertStorage(storagePath string) *StreamplaceCertStorage {
	return &StreamplaceCertStorage{
		Path:  storagePath,
		locks: make(map[string]*fileLock),
	}
}

func (s *StreamplaceCertStorage) Store(ctx context.Context, key string, value []byte) error {
	filePath := s.keyToPath(key)

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file atomically by writing to a temp file first
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, value, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Move temp file to final location
	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath) // Clean up temp file on error
		return fmt.Errorf("failed to move temp file to final location: %w", err)
	}

	return nil
}

func (s *StreamplaceCertStorage) Load(ctx context.Context, key string) ([]byte, error) {
	filePath := s.keyToPath(key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fs.ErrNotExist
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

func (s *StreamplaceCertStorage) Delete(ctx context.Context, key string) error {
	filePath := s.keyToPath(key)

	// Check if it's a directory (prefix of other keys)
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fs.ErrNotExist
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		err = os.RemoveAll(filePath)
	} else {
		err = os.Remove(filePath)
	}

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

func (s *StreamplaceCertStorage) Exists(ctx context.Context, key string) bool {
	filePath := s.keyToPath(key)
	_, err := os.Stat(filePath)
	return err == nil
}

func (s *StreamplaceCertStorage) List(ctx context.Context, prefix string, recursive bool) ([]string, error) {
	dirPath := s.keyToPath(prefix)

	var keys []string

	if recursive {
		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Convert back to key format
			if relPath, err := filepath.Rel(s.Path, path); err == nil {
				key := s.pathToKey(relPath)
				if key != prefix && strings.HasPrefix(key, prefix) {
					keys = append(keys, key)
				}
			}

			return nil
		})

		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				return keys, nil
			}
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			key := prefix
			if key != "" && !strings.HasSuffix(key, "/") {
				key += "/"
			}
			key += entry.Name()
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func (s *StreamplaceCertStorage) Stat(ctx context.Context, key string) (certmagic.KeyInfo, error) {
	filePath := s.keyToPath(key)

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return certmagic.KeyInfo{}, fs.ErrNotExist
		}
		return certmagic.KeyInfo{}, fmt.Errorf("failed to stat file: %w", err)
	}

	return certmagic.KeyInfo{
		Key:        key,
		Modified:   info.ModTime(),
		Size:       info.Size(),
		IsTerminal: !info.IsDir(),
	}, nil
}

func (s *StreamplaceCertStorage) Lock(ctx context.Context, name string) error {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()

	// Check if we already have this lock
	if _, exists := s.locks[name]; exists {
		return fmt.Errorf("lock %s already held by this process", name)
	}

	lockPath := s.keyToPath("locks/" + name + ".lock")

	// Ensure lock directory exists
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Try to create the lock file exclusively
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists, check if it's stale
			if s.isLockStale(lockPath) {
				// Remove stale lock and try again
				os.Remove(lockPath)
				lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
				if err != nil {
					return fmt.Errorf("failed to acquire lock after removing stale lock: %w", err)
				}
			} else {
				return fmt.Errorf("lock is held by another process")
			}
		} else {
			return fmt.Errorf("failed to create lock file: %w", err)
		}
	}

	// Write lock info with timestamp
	lockInfo := map[string]any{
		"pid":       os.Getpid(),
		"timestamp": time.Now().Unix(),
	}

	lockData, err := json.Marshal(lockInfo)
	if err != nil {
		lockFile.Close()
		os.Remove(lockPath)
		return fmt.Errorf("failed to marshal lock info: %w", err)
	}
	_, err = lockFile.Write(lockData)
	if err != nil {
		lockFile.Close()
		os.Remove(lockPath)
		return fmt.Errorf("failed to write lock info: %w", err)
	}

	lockFile.Close()

	// Create a file lock struct and start the renewal goroutine
	ctx, cancel := context.WithCancel(ctx)
	lock := &fileLock{
		path:   lockPath,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	s.locks[name] = lock

	// Start renewal goroutine to update timestamp periodically
	go s.renewLock(ctx, lock)

	return nil
}

func (s *StreamplaceCertStorage) Unlock(ctx context.Context, name string) error {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()

	lock, exists := s.locks[name]
	if !exists {
		return fmt.Errorf("lock %s not held by this process", name)
	}

	lock.cancel()

	<-lock.done

	err := os.Remove(lock.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	delete(s.locks, name)

	return nil
}

func (s *StreamplaceCertStorage) keyToPath(key string) string {
	return filepath.Join(s.Path, filepath.FromSlash(key))
}
func (s *StreamplaceCertStorage) pathToKey(path string) string {
	return filepath.ToSlash(path)
}
func (s *StreamplaceCertStorage) isLockStale(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return true
	}

	var lockInfo map[string]any
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return true
	}

	timestamp, ok := lockInfo["timestamp"].(float64)
	if !ok {
		return true
	}

	lockTime := time.Unix(int64(timestamp), 0)
	return time.Since(lockTime) > 30*time.Second
}
func (s *StreamplaceCertStorage) renewLock(ctx context.Context, lock *fileLock) {
	defer close(lock.done)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// update the lock file timestamp
			lockInfo := map[string]any{
				"pid":       os.Getpid(),
				"timestamp": time.Now().Unix(),
			}

			if lockData, err := json.Marshal(lockInfo); err == nil {
				err = os.WriteFile(lock.path, lockData, 0644)
				if err != nil {
					// lock is probably stale, remove it
					s.locksMu.Lock()
					delete(s.locks, lock.path)
					s.locksMu.Unlock()
					return
				}
			}
		}
	}
}
