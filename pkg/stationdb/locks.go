package stationdb

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
)

const (
	lockTimeout = 5 * time.Second
	lockPoll    = 50 * time.Millisecond
)

// GetNamedLock acquires a station-wide lock without an expiry. A crashed holder
// stays locked: only its acquired object version can be released automatically.
func (s *Store) GetNamedLock(name string) (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	return s.getNamedLock(ctx, name)
}

func (s *Store) getNamedLock(ctx context.Context, name string) (func(), error) {
	mac := hmac.New(sha256.New, s.lockKey[:])
	mac.Write([]byte("stationdb/v1/lock/" + name))
	key := s.prefix + "locks/" + hex.EncodeToString(mac.Sum(nil))
	var owner pageID
	rand.Read(owner[:])
	timer := time.NewTicker(lockPoll)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrLockBusy, err)
		}
		etag, free, err := s.lockState(ctx, key)
		if err != nil {
			return nil, err
		}
		if free {
			acquired, err := s.put(ctx, key, owner[:], etag)
			if err == nil {
				return sync.OnceFunc(func() { s.releaseLock(key, acquired) }), nil
			}
			if !errors.Is(err, errCAS) {
				return nil, fmt.Errorf("acquire station database lock: %w", err)
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrLockBusy, ctx.Err())
		case <-timer.C:
		}
	}
}

func (s *Store) lockState(ctx context.Context, key string) (string, bool, error) {
	body, etag, err := s.objects.get(ctx, key)
	if errors.Is(err, errMissing) {
		return "", true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read station database lock: %w", err)
	}
	plain, err := s.decrypt(key, body)
	if err != nil {
		return "", false, err
	}
	if etag == "" || (len(plain) != 0 && len(plain) != len(pageID{})) {
		return "", false, errors.New("invalid station database lock marker")
	}
	return etag, len(plain) == 0, nil
}

func (s *Store) releaseLock(key, acquired string) {
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	// A new GCM nonce gives each free marker a new object version, too.
	if _, err := s.put(ctx, key, nil, acquired); err != nil {
		log.Error(ctx, "could not release station database named lock", "error", err)
	}
}
