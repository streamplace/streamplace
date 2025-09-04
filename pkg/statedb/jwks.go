package statedb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lestrrat-go/jwx/v2/jwk"
	oauth_helpers "github.com/streamplace/atproto-oauth-golang/helpers"
	"stream.place/streamplace/pkg/log"
)

func (state *StatefulDB) EnsureJWK(ctx context.Context, name string) (jwk.Key, error) {
	var key jwk.Key

	conf, err := state.GetConfig(name)
	if err != nil {
		return nil, err
	}

	// happy path: we found the jwk in the database, use that
	if conf != nil {
		key, err = jwk.ParseKey(conf.Value)
		if err != nil {
			return nil, err
		}
		return key, nil
	}

	// migration path: maybe we have an old one on disk.
	key, _ = state.getOldJWK(ctx, name)

	// new path: found neither, generate a new one
	if key == nil {
		log.Warn(ctx, "no JWK found, generating new one", "name", name)
		key, err = oauth_helpers.GenerateKey(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to generate JWK: %w", err)
		}
	}

	b, err := json.Marshal(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JWK: %w", err)
	}
	err = state.PutConfig(name, b)
	if err != nil {
		return nil, fmt.Errorf("failed to save JWK: %w", err)
	}

	return key, nil
}

// migration for the old one we stored on disk
func (state *StatefulDB) getOldJWK(ctx context.Context, name string) (jwk.Key, error) {
	var key jwk.Key
	jwkPath := state.CLI.DataFilePath([]string{name + ".json"})
	_, err := os.Stat(jwkPath)
	if err == nil {
		b, err := os.ReadFile(jwkPath)
		if err != nil {
			return nil, err
		}
		key, err = jwk.ParseKey(b)
		if err != nil {
			return nil, err
		}
		log.Warn(ctx, "found old JWK on disk, migrating to stateful database", "path", jwkPath)
		return key, nil
	}
	return nil, nil
}
