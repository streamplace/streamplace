package stationshard

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
)

type writer struct {
	mu         sync.Mutex // serializes a slot's read/modify/write operations
	db         database
	lease      Lease
	controlKey string
	until      time.Time
	timer      *time.Timer
}

// An expired, unused slot should not keep an embedded DB polling S3 forever.
// All handle use, including shutdown, runs under this writer's mutex.
func (w *writer) expireAt(until time.Time) {
	if w.timer != nil && w.until.Equal(until) {
		return
	}
	w.until = until
	if w.timer != nil {
		w.timer.Reset(time.Until(until))
		return
	}
	w.timer = time.AfterFunc(time.Until(until), func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.db != nil && !time.Now().Before(w.until) {
			if err := w.close(); err != nil {
				log.Warn(context.Background(), "closing expired station shard", "error", err)
			}
		}
	})
}

func (w *writer) close() error {
	if w.db == nil {
		return nil
	}
	err := w.db.Close()
	w.db = nil
	if errors.Is(err, ErrFenced) {
		return nil
	}
	return err
}

// Owner is a read-only routing hint. It never activates an unclaimed slot.
func (s *Store) Owner(ctx context.Context, did string) (string, error) {
	if err := s.begin(); err != nil {
		return "", err
	}
	defer s.active.Done()
	c, err := s.load(ctx, s.slotPath(did)+"/owner")
	if err != nil {
		return "", err
	}
	if c.Owner == "" || !time.Now().Before(c.Expires) {
		return "", ErrNotFound
	}
	return c.Owner, nil
}

// ensure returns the current local writer, or the remote routing hint plus
// ErrHeld. Caller holds w.mu. Never let a suspended opener unconditionally
// bump SlateDB's native epoch: sample its epoch, confirm our assignment by
// conditional PUT, and open *only* against that sampled epoch. A newer
// completed open means the stale opening attempt cannot fence the new writer.
func (s *Store) ensure(ctx context.Context, did string, w *writer) (control, error) {
	if did == "" {
		return control{}, errors.New("empty station shard identity")
	}
	root := s.slotPath(did)
	key := root + "/owner"
	c, err := s.claim(ctx, key, fmt.Sprintf("%02x", Slot(did)), "shard", true)
	if err != nil {
		return c, err
	}
	if w.db != nil && w.lease == c.Lease {
		if time.Until(c.Expires) < s.ttl/2 {
			c, err = s.change(ctx, key, c.Lease, nil, renew)
			if err != nil {
				return c, err
			}
		}
		w.expireAt(c.Expires)
		return c, nil
	}
	if err := w.close(); err != nil {
		return c, err
	}
	dbPath := root + "/db"
	for {
		expected, err := s.engine.Epoch(dbPath)
		if err != nil {
			return c, err
		}
		c, err = s.change(ctx, key, c.Lease, nil, confirmOpener)
		if err != nil {
			return c, err
		}
		db, err := s.engine.Open(dbPath, expected)
		if errors.Is(err, ErrFenced) {
			// A contender who sampled before us completed the native CAS first.
			// Retry only through the assignment check above.
			continue
		}
		if err != nil {
			return c, err
		}
		// Open fences the WAL and replays acknowledged mutations. If our
		// assignment changed in the meantime, never install this handle.
		c, err = s.change(ctx, key, c.Lease, nil, confirmOpener)
		if err != nil {
			return c, errors.Join(err, db.Close())
		}
		w.db, w.lease, w.controlKey = db, c.Lease, key
		w.expireAt(c.Expires)
		return c, nil
	}
}

// Route is for authenticated management operations. Never call it for
// untrusted arbitrary DIDs: an unassigned slot may be activated locally.
func (s *Store) Route(ctx context.Context, did string) (string, error) {
	if err := s.begin(); err != nil {
		return "", err
	}
	defer s.active.Done()
	w := &s.slots[Slot(did)]
	w.mu.Lock()
	defer w.mu.Unlock()
	c, err := s.ensure(ctx, did, w)
	if errors.Is(err, ErrHeld) && c.Owner != s.owner {
		return c.Owner, nil
	}
	if err != nil {
		return "", err
	}
	return s.owner, nil
}

func dbKey(did, key string) []byte {
	prefix := sha256.Sum256([]byte(did))
	return append(prefix[:], key...)
}

func (w *writer) get(key []byte) ([]byte, error) {
	value, err := w.db.Get(key)
	if err != nil && !errors.Is(err, ErrNotFound) {
		// Background WAL or object-store failures can close a SlateDB handle.
		// Never keep renewing the lease on a permanently unusable writer.
		return nil, errors.Join(err, w.close())
	}
	return value, err
}

func (s *Store) Get(ctx context.Context, did, key string) ([]byte, error) {
	if err := s.begin(); err != nil {
		return nil, err
	}
	defer s.active.Done()
	w := &s.slots[Slot(did)]
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := s.ensure(ctx, did, w); err != nil {
		return nil, err
	}
	return w.get(dbKey(did, key))
}

// Update applies exactly one callback under this slot's writer lock. It must
// not perform external side effects or reenter the slot. A nil return error
// means the new value is durable in the object store WAL. Transport failures
// can have unknown outcomes; callers use stable operation IDs when retrying.
func (s *Store) Update(ctx context.Context, did, key string, update func([]byte) ([]byte, error)) error {
	if update == nil {
		return errors.New("missing shard update")
	}
	if err := s.begin(); err != nil {
		return err
	}
	defer s.active.Done()
	w := &s.slots[Slot(did)]
	w.mu.Lock()
	defer w.mu.Unlock()
	c, err := s.ensure(ctx, did, w)
	if err != nil {
		return err
	}
	k := dbKey(did, key)
	prev, err := w.get(k)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	next, err := update(prev)
	if err != nil {
		return err
	}
	// A paused callback may return after a takeover. The conditional lease
	// renewal detects assignment changes; SlateDB's independent WAL fence
	// protects the window between this check and durable commit.
	c, err = s.change(ctx, w.controlKey, c.Lease, nil, renew)
	if err != nil {
		return err
	}
	w.expireAt(c.Expires)
	if err := w.db.Put(k, next); err != nil {
		return errors.Join(err, w.close())
	}
	return nil
}
