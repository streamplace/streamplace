package statedb

import (
	"context"
	"encoding/json"
	"fmt"

	webpush "github.com/SherClockHolmes/webpush-go"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/log"
)

// vapidConfigKey is the Config-table key under which the VAPID keypair is
// stored, mirroring how EnsureJWK persists JWKs by name.
const vapidConfigKey = "vapid-keys"

// EnsureVAPIDKeys returns the Web Push VAPID keypair, generating and
// persisting it on first use. It follows the same pattern as EnsureJWK:
// look the key up in the Config table; if present, use it; otherwise
// generate a fresh P-256 keypair and store it so it survives restarts.
//
// VAPID keys must stay stable — rotating them invalidates every existing
// browser subscription, so we never regenerate once a key exists.
func (state *StatefulDB) EnsureVAPIDKeys(ctx context.Context) (notificationpkg.VAPIDKeys, error) {
	conf, err := state.GetConfig(vapidConfigKey)
	if err != nil {
		return notificationpkg.VAPIDKeys{}, fmt.Errorf("error loading vapid keys: %w", err)
	}

	// happy path: we found the keys in the database, use that
	if conf != nil {
		var keys notificationpkg.VAPIDKeys
		if err := json.Unmarshal(conf.Value, &keys); err != nil {
			return notificationpkg.VAPIDKeys{}, fmt.Errorf("error parsing stored vapid keys: %w", err)
		}
		return keys, nil
	}

	// new path: no keys yet, generate a fresh pair
	log.Warn(ctx, "no VAPID keys found, generating new ones")
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return notificationpkg.VAPIDKeys{}, fmt.Errorf("failed to generate vapid keys: %w", err)
	}
	keys := notificationpkg.VAPIDKeys{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}

	b, err := json.Marshal(keys)
	if err != nil {
		return notificationpkg.VAPIDKeys{}, fmt.Errorf("failed to marshal vapid keys: %w", err)
	}
	if err := state.PutConfig(vapidConfigKey, b); err != nil {
		return notificationpkg.VAPIDKeys{}, fmt.Errorf("failed to save vapid keys: %w", err)
	}

	return keys, nil
}
