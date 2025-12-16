package statedb

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"stream.place/streamplace/pkg/log"
)

func (state *StatefulDB) EnsurePublisherKey(ctx context.Context) (*atcrypto.PrivateKeyK256, *atcrypto.PublicKeyK256, error) {
	var priv *atcrypto.PrivateKeyK256

	keyBs, err := state.GetConfig("publisher-key")
	if err != nil {
		return nil, nil, err
	}

	if keyBs != nil {
		priv, err = atcrypto.ParsePrivateBytesK256(keyBs.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse publisher key: %w", err)
		}
	} else {
		log.Warn(ctx, "no publisher key found, generating new one")
		priv, err = atcrypto.GeneratePrivateKeyK256()
		if err != nil {
			return nil, nil, err
		}

		bs := priv.Bytes()
		err = state.PutConfig("publisher-key", bs)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to save publisher key: %w", err)
		}
	}

	pub, err := priv.PublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get public key: %w", err)
	}

	return priv, pub.(*atcrypto.PublicKeyK256), nil
}
