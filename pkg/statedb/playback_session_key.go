package statedb

import (
	"context"
	"crypto/rand"
	"fmt"
)

// EnsurePlaybackSessionKey returns the HMAC key that signs playback sessions
// (pkg/psession), generating and storing one on first use. Shared through
// statedb so every node in a station honours sessions minted by any other.
func (state *StatefulDB) EnsurePlaybackSessionKey(ctx context.Context) ([]byte, error) {
	const key = "playback-session-key"
	conf, err := state.GetConfig(key)
	if err != nil {
		return nil, fmt.Errorf("get playback session key: %w", err)
	}
	if conf != nil && len(conf.Value) >= 32 {
		return conf.Value, nil
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate playback session key: %w", err)
	}
	if err := state.PutConfig(key, secret); err != nil {
		return nil, fmt.Errorf("store playback session key: %w", err)
	}
	return secret, nil
}
