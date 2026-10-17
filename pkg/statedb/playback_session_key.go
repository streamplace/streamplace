package statedb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

// EnsurePlaybackSessionKey returns the HMAC key that signs playback sessions
// (pkg/psession), generating and storing one on first use. Shared through
// statedb so every node in a station honours sessions minted by any other.
func (state *StatefulDB) EnsurePlaybackSessionKey(ctx context.Context) ([]byte, error) {
	return state.ensureSharedSecret(ctx, "playback-session-key", 32)
}

// ensureSharedSecret returns the secret stored under key, generating one of
// n bytes on first use. The generate-and-store runs under the station's
// named lock, and the value is read again once the lock is held: two nodes
// starting together must both end up with the one secret the database
// keeps, or sessions minted on one node would fail verification on the
// other.
func (state *StatefulDB) ensureSharedSecret(ctx context.Context, key string, n int) ([]byte, error) {
	conf, err := state.GetConfig(key)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	if conf != nil && len(conf.Value) >= n {
		return conf.Value, nil
	}
	unlock, err := state.WaitForNamedLock(ctx, "ensure-secret:"+key)
	if err != nil {
		return nil, fmt.Errorf("lock for %s: %w", key, err)
	}
	defer unlock()
	// Another node may have won the race while we waited.
	if conf, err = state.GetConfig(key); err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	if conf != nil && len(conf.Value) >= n {
		return conf.Value, nil
	}
	secret := make([]byte, n)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate %s: %w", key, err)
	}
	if err := state.PutConfig(key, secret); err != nil {
		return nil, fmt.Errorf("store %s: %w", key, err)
	}
	return secret, nil
}

// WaitForNamedLock takes the named lock, waiting out another holder until
// ctx ends. GetNamedLock itself can block on the node-local mutex before
// the database is even asked, so the attempt runs on its own goroutine and
// a lock that arrives after the caller gave up is released at once.
func (state *StatefulDB) WaitForNamedLock(ctx context.Context, name string) (func(), error) {
	type got struct {
		unlock func()
		err    error
	}
	for {
		ch := make(chan got, 1)
		go func() {
			unlock, err := state.GetNamedLock(name)
			ch <- got{unlock, err}
		}()
		var g got
		select {
		case g = <-ch:
		case <-ctx.Done():
			go func() {
				if g := <-ch; g.err == nil {
					g.unlock()
				}
			}()
			return nil, ctx.Err()
		}
		if g.err == nil {
			return g.unlock, nil
		}
		if !errors.Is(g.err, ErrNoLock) {
			return nil, g.err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
