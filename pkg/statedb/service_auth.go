package statedb

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"stream.place/streamplace/pkg/log"
)

// EnsureServiceAuthKey ensures a shared symmetric key exists in the config table
// for intra-service JWT authentication. All nodes sharing the same database will
// use the same key, enabling mutual authentication within a station.
func (state *StatefulDB) EnsureServiceAuthKey(ctx context.Context) (jwk.Key, error) {
	conf, err := state.GetConfig("service-auth-key")
	if err != nil {
		return nil, fmt.Errorf("failed to get service auth key: %w", err)
	}

	if conf != nil {
		key, err := jwk.ParseKey(conf.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse service auth key: %w", err)
		}
		return key, nil
	}

	log.Warn(ctx, "no service auth key found, generating new one")

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	key, err := jwk.FromRaw(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to create symmetric key: %w", err)
	}

	b, err := json.Marshal(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service auth key: %w", err)
	}

	if err := state.PutConfig("service-auth-key", b); err != nil {
		return nil, fmt.Errorf("failed to save service auth key: %w", err)
	}

	return key, nil
}
