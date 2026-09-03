// Package acme obtains and renews the node's TLS certificates from an ACME
// CA (Let's Encrypt by default) with certmagic, keeping everything in
// statedb so a station's nodes share one set of certificates.
package acme

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
)

// Storage is certmagic.Storage on top of statedb. Values live in the
// certmagic_items table; locks are statedb named locks, which on Postgres
// are session-scoped advisory locks (released automatically if the holder
// dies, so there is no stale-lock bookkeeping) and on sqlite are in-process
// mutexes (a sqlite statedb is a single-node deployment).
type Storage struct {
	state *statedb.StatefulDB

	mu    sync.Mutex
	locks map[string]func()
}

var _ certmagic.Storage = (*Storage)(nil)

// NewStorage wraps state.
func NewStorage(state *statedb.StatefulDB) *Storage {
	return &Storage{state: state, locks: map[string]func(){}}
}

const lockPrefix = "certmagic/"

// lockRetryDelay is how long Lock waits between attempts once the statedb
// backoff has given up on a contended lock.
var lockRetryDelay = 2 * time.Second

// Lock implements certmagic.Locker. It blocks until the lock is held or ctx
// ends. A lock held by another node in the station (Postgres) or another
// goroutine (sqlite) makes the call wait, as certmagic expects.
func (s *Storage) Lock(ctx context.Context, name string) error {
	for {
		unlock, err := s.state.GetNamedLock(lockPrefix + name)
		if err == nil {
			s.mu.Lock()
			s.locks[name] = unlock
			s.mu.Unlock()
			return nil
		}
		if !errors.Is(err, statedb.ErrNoLock) {
			return fmt.Errorf("acme lock %q: %w", name, err)
		}
		log.Debug(ctx, "acme: lock held elsewhere, waiting", "name", name)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockRetryDelay):
		}
	}
}

// Unlock implements certmagic.Locker.
func (s *Storage) Unlock(ctx context.Context, name string) error {
	s.mu.Lock()
	unlock, ok := s.locks[name]
	delete(s.locks, name)
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("acme unlock %q: not held", name)
	}
	unlock()
	return nil
}

// Store implements certmagic.Storage.
func (s *Storage) Store(ctx context.Context, key string, value []byte) error {
	return s.state.CertmagicPut(ctx, key, value)
}

// Load implements certmagic.Storage.
func (s *Storage) Load(ctx context.Context, key string) ([]byte, error) {
	item, err := s.state.CertmagicGet(ctx, key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fs.ErrNotExist
	}
	return item.Value, nil
}

// Delete implements certmagic.Storage: key and everything under it.
func (s *Storage) Delete(ctx context.Context, key string) error {
	_, err := s.state.CertmagicDelete(ctx, key)
	return err
}

// Exists implements certmagic.Storage.
func (s *Storage) Exists(ctx context.Context, key string) bool {
	rows, err := s.state.CertmagicList(ctx, key)
	return err == nil && len(rows) > 0
}

// List implements certmagic.Storage with the same shape as certmagic's own
// FileStorage: every key below prefix when recursive (implicit directories
// included), else only the immediate children.
func (s *Storage) List(ctx context.Context, prefix string, recursive bool) ([]string, error) {
	rows, err := s.state.CertmagicList(ctx, prefix)
	if err != nil {
		return nil, err
	}
	base := prefix
	if base != "" {
		base += "/"
	}
	seen := map[string]struct{}{}
	var keys []string
	add := func(k string) {
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for _, row := range rows {
		if row.Key == prefix {
			continue
		}
		rel := strings.TrimPrefix(row.Key, base)
		parts := strings.Split(rel, "/")
		if !recursive {
			add(base + parts[0])
			continue
		}
		for i := range parts {
			add(base + strings.Join(parts[:i+1], "/"))
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// Stat implements certmagic.Storage.
func (s *Storage) Stat(ctx context.Context, key string) (certmagic.KeyInfo, error) {
	rows, err := s.state.CertmagicList(ctx, key)
	if err != nil {
		return certmagic.KeyInfo{}, err
	}
	if len(rows) == 0 {
		return certmagic.KeyInfo{}, fs.ErrNotExist
	}
	info := certmagic.KeyInfo{Key: key}
	for _, row := range rows {
		if row.Key == key {
			info.IsTerminal = true
			info.Size = row.Size
			info.Modified = row.UpdatedAt
			return info, nil
		}
		if row.UpdatedAt.After(info.Modified) {
			info.Modified = row.UpdatedAt
		}
	}
	return info, nil
}

// String makes storage identifiable in certmagic's logs.
func (s *Storage) String() string {
	return "statedb"
}
