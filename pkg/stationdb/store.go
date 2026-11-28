// Package stationdb stores a private SQLite database in an S3 CAS namespace.
package stationdb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	// ErrConflict means that another node published a write based on the same snapshot.
	ErrConflict = errors.New("station database write conflict")
	// ErrCommitUnknown means a head PUT failed without proving whether it took effect.
	ErrCommitUnknown = errors.New("station database commit outcome unknown")
	// ErrLockBusy means the lock stayed held throughout the acquisition budget.
	ErrLockBusy = errors.New("station database named lock busy")

	errMissing     = errors.New("station database object missing")
	errMissingHead = errors.New("station database head missing")
	errCAS         = errors.New("station database object precondition failed")
)

const (
	pageSize      = 4096
	maxPages      = math.MaxInt / pageSize
	objectTimeout = 30 * time.Second
	pageWorkers   = 8
)

// A missing match means create-if-absent. Nonempty matches are opaque object ETags.
// Reads, prefix emptiness checks, and single-key conditional writes must be strongly consistent.
type objects interface {
	get(ctx context.Context, key string) (body []byte, etag string, err error)
	empty(ctx context.Context, prefix string) (bool, error)
	put(ctx context.Context, key string, body []byte, match string) (etag string, err error)
}

type pageID [16]byte

type pageData [pageSize]byte

type storedPage struct {
	id   pageID
	data *pageData
}

type snapshot struct {
	etag  string
	pages []storedPage
}

// Store owns a station namespace. Cached snapshots are immutable and are never
// served without first reading the current head from the bucket.
type Store struct {
	objects objects
	prefix  string
	aead    cipher.AEAD
	lockKey [32]byte
	latest  atomic.Pointer[snapshot]
}

func openStore(ctx context.Context, objects objects, prefix string, key []byte) (*Store, error) {
	prefix, err := validateOpenInputs(prefix, key)
	if err != nil {
		return nil, err
	}
	return openValidatedStore(ctx, objects, prefix, key)
}

func validateOpenInputs(prefix string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", errors.New("station database requires a 32-byte encryption key")
	}
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "", errors.New("station database requires a station prefix")
	}
	return prefix, nil
}

func openValidatedStore(ctx context.Context, objects objects, prefix string, key []byte) (*Store, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	s := &Store{objects: objects, prefix: prefix + "/stationdb/v1/", aead: aead, lockKey: [32]byte(key)}
	ctx, cancel := context.WithTimeout(ctx, objectTimeout)
	defer cancel()

	if _, err := s.load(ctx); !errors.Is(err, errMissingHead) {
		if err != nil {
			return nil, err
		}
		return s, nil
	}
	empty, err := objects.empty(ctx, s.prefix)
	if err != nil {
		return nil, fmt.Errorf("check station database namespace: %w", err)
	}
	if !empty {
		// Another opener may have initialized the head after our first read.
		if _, err := s.load(ctx); err != nil {
			if errors.Is(err, errMissingHead) {
				return nil, fmt.Errorf("station database namespace is corrupt or incomplete: %w", err)
			}
			return nil, err
		}
		return s, nil
	}

	name := s.prefix + "head"
	etag, err := s.put(ctx, name, nil, "")
	if errors.Is(err, errCAS) {
		if _, err := s.load(ctx); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("initialize station database: %w", err)
	}
	s.latest.Store(&snapshot{etag: etag})
	return s, nil
}

func (s *Store) put(ctx context.Context, name string, plain []byte, match string) (string, error) {
	etag, err := s.objects.put(ctx, name, s.aead.Seal(nil, nil, plain, []byte(name)), match)
	if err == nil && etag == "" {
		return "", errors.New("station database PUT response has no ETag")
	}
	return etag, err
}

func (s *Store) decrypt(name string, body []byte) ([]byte, error) {
	plain, err := s.aead.Open(body[:0], nil, body, []byte(name))
	if err != nil {
		return nil, fmt.Errorf("authenticate station database object: %w", err)
	}
	return plain, nil
}

func (s *Store) load(ctx context.Context) (*snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, objectTimeout)
	defer cancel()
	cached := s.latest.Load()
	name := s.prefix + "head"
	body, etag, err := s.objects.get(ctx, name)
	if errors.Is(err, errMissing) {
		return nil, errMissingHead
	}
	if err != nil {
		return nil, fmt.Errorf("read station database head: %w", err)
	}
	if etag == "" {
		return nil, errors.New("station database head has no ETag")
	}
	if cached != nil && cached.etag == etag {
		return cached, nil
	}
	plain, err := s.decrypt(name, body)
	if err != nil {
		return nil, err
	}
	const idSize = len(pageID{})
	if len(plain)%idSize != 0 || len(plain)/idSize > maxPages {
		return nil, errors.New("invalid station database page index")
	}
	n := len(plain) / idSize
	next := &snapshot{etag: etag, pages: make([]storedPage, n)}
	group, fetchCtx := errgroup.WithContext(ctx)
	group.SetLimit(pageWorkers)
	for i := range next.pages {
		id := pageID(plain[i*idSize : (i+1)*idSize])
		if cached != nil && i < len(cached.pages) && cached.pages[i].id == id {
			next.pages[i] = cached.pages[i]
			continue
		}
		if fetchCtx.Err() != nil {
			break
		}
		group.Go(func() error {
			data, err := s.readPage(fetchCtx, id)
			next.pages[i] = storedPage{id: id, data: data}
			return err
		})
	}
	if err := group.Wait(); err != nil {
		// A dangling page is corruption, not permission to create an empty head.
		return nil, fmt.Errorf("read station database pages: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if n != 0 && !validHeader(next.pages[0].data) {
		return nil, errors.New("invalid station SQLite image")
	}
	s.latest.CompareAndSwap(cached, next)
	return next, nil
}

func (s *Store) readPage(ctx context.Context, id pageID) (*pageData, error) {
	name := s.prefix + "pages/" + hex.EncodeToString(id[:])
	body, _, err := s.objects.get(ctx, name)
	if err != nil {
		return nil, err
	}
	plain, err := s.decrypt(name, body)
	if err != nil {
		return nil, err
	}
	if len(plain) != pageSize {
		return nil, errors.New("invalid station database page length")
	}
	return (*pageData)(plain), nil
}

func validHeader(data *pageData) bool {
	return bytes.HasPrefix(data[:], []byte("SQLite format 3\x00")) &&
		binary.BigEndian.Uint16(data[16:18]) == pageSize
}
