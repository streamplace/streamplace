package media

import (
	"context"
	"crypto"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/mr-tron/base58"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

// AuthenticateStreamKey is shared by WHIP and pushCaptions. The latter requires
// a registered place.stream.key even for the legacy did:key-only encoding.
func AuthenticateStreamKey(ctx context.Context, cli *config.CLI, mod model.Model, syncer *atproto.ATProtoSynchronizer, keyStr string, requireRegistered bool) (string, crypto.Signer, error) {
	if len(keyStr) < 2 || keyStr[0] != 'z' {
		return "", nil, fmt.Errorf("invalid authorization key (not a multibase base58btc string)")
	}
	var addrBytes, didBytes []byte
	priv, err := atcrypto.ParsePrivateMultibase(keyStr)
	if err == nil {
		addrBytes = priv.Bytes()
	} else {
		decoded, err := base58.Decode(keyStr[1:])
		if err != nil {
			return "", nil, fmt.Errorf("invalid authorization key (not a base58btc string)")
		}
		if len(decoded) < 32 {
			return "", nil, fmt.Errorf("invalid authorization key (too short)")
		}
		addrBytes, didBytes = decoded[:32], decoded[32:]
		priv, err = atcrypto.ParsePrivateBytesK256(addrBytes)
		if err != nil {
			return "", nil, fmt.Errorf("invalid authorization key (not valid atproto): %w", err)
		}
	}
	key, _ := secp256k1.PrivKeyFromBytes(addrBytes)
	var signer crypto.Signer = key.ToECDSA()
	pub, err := priv.PublicKey()
	if err != nil {
		return "", nil, fmt.Errorf("invalid authorization key (could not parse as atproto): %w", err)
	}
	did := string(didBytes)
	if did != "" {
		repo, err := syncer.SyncBlueskyRepo(ctx, did, mod)
		if err != nil {
			return "", nil, fmt.Errorf("could not resolve streamplace key: %w", err)
		}
		did = repo.DID
	} else {
		did = pub.DIDKey()
	}
	if err := CheckStreamAllowed(cli, mod, did); err != nil {
		return "", nil, err
	}
	if len(didBytes) > 0 || requireRegistered {
		signingKey, err := mod.GetSigningKey(ctx, pub.DIDKey(), did)
		if err != nil {
			return "", nil, fmt.Errorf("signing key not found: %w", err)
		}
		if signingKey == nil {
			return "", nil, fmt.Errorf("signing key not found")
		}
	}
	return did, signer, nil
}

// CheckStreamAllowed applies the account policy shared by stream keys and OAuth caption pushes.
func CheckStreamAllowed(cli *config.CLI, mod model.Model, did string) error {
	if err := cli.StreamIsAllowed(did); err != nil {
		return fmt.Errorf("user is not allowed to stream: %w", err)
	}
	labels, err := mod.GetActiveLabels(did)
	if err != nil {
		return fmt.Errorf("failed to get active labels: %w", err)
	}
	if atproto.IsBanned(labels...) {
		return fmt.Errorf("user is banned")
	}
	return nil
}
