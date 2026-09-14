package statedb

import (
	"context"
	"crypto/rand"
	"fmt"
)

// EnsureLiveTokenKey returns the HMAC key that signs pre-live playback
// tokens, generating and storing one on first use. Shared through statedb
// so every node in a station honours tokens minted by any other.
func (state *StatefulDB) EnsureLiveTokenKey(ctx context.Context) ([]byte, error) {
	const key = "live-token-key"
	conf, err := state.GetConfig(key)
	if err != nil {
		return nil, fmt.Errorf("get live token key: %w", err)
	}
	if conf != nil && len(conf.Value) >= 32 {
		return conf.Value, nil
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate live token key: %w", err)
	}
	if err := state.PutConfig(key, secret); err != nil {
		return nil, fmt.Errorf("store live token key: %w", err)
	}
	return secret, nil
}
