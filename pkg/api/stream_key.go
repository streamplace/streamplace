package api

import (
	"context"
	"crypto"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/mr-tron/base58"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

// parsedStreamKey is a decoded stream key, not yet checked against the keys
// its streamer registered.
type parsedStreamKey struct {
	signer crypto.Signer
	pub    atcrypto.PublicKey
	// did is the streamer the key names; empty for a bare key, which streams
	// as its own did:key.
	did string
}

func parseStreamKey(keyStr string) (*parsedStreamKey, error) {
	if len(keyStr) < 2 || keyStr[0] != 'z' {
		return nil, fmt.Errorf("invalid authorization key (not a multibase base58btc string)")
	}

	var addrBytes []byte
	var didBytes []byte
	priv, err := atcrypto.ParsePrivateMultibase(keyStr)
	if err == nil {
		addrBytes = priv.Bytes()
	} else {
		decoded, err := base58.Decode(keyStr[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid authorization key (not a base58btc string)")
		}
		if len(decoded) < 32 {
			return nil, fmt.Errorf("invalid authorization key (too short)")
		}
		addrBytes = decoded[:32]
		didBytes = decoded[32:]
		priv, err = atcrypto.ParsePrivateBytesK256(addrBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid authorization key (not valid atproto): %w", err)
		}
	}

	key, _ := secp256k1.PrivKeyFromBytes(addrBytes)
	if key == nil {
		return nil, fmt.Errorf("invalid authorization key (not valid secp256k1)")
	}
	pub, err := priv.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("invalid authorization key (could not parse as atproto): %w", err)
	}
	return &parsedStreamKey{signer: key.ToECDSA(), pub: pub, did: string(didBytes)}, nil
}

// bareKeyDID is the did:key a stream key without a streamer DID streams as.
func (k *parsedStreamKey) bareKeyDID() (string, error) {
	atkey, err := atproto.ParsePubKey(k.signer.Public())
	if err != nil {
		return "", fmt.Errorf("invalid authorization key (not valid secp256k1): %w", err)
	}
	return atkey.DIDKey(), nil
}

// StreamerForKey returns the DID of the streamer a stream key belongs to,
// checking that the streamer registered the key but not that they may stream:
// MakeMediaSigner decides that when the ingest itself starts. It syncs the
// streamer's repo only when the key isn't indexed yet (a key registered moments
// ago), as MakeMediaSigner would.
func (a *StreamplaceAPI) StreamerForKey(ctx context.Context, keyStr string) (string, error) {
	k, err := parseStreamKey(keyStr)
	if err != nil {
		return "", err
	}
	if k.did == "" {
		return k.bareKeyDID()
	}
	signingKey, err := a.Model.GetSigningKey(ctx, k.pub.DIDKey(), k.did)
	if err == nil && signingKey == nil {
		if _, err = a.ATSync.SyncBlueskyRepo(ctx, k.did, a.Model); err != nil {
			return "", fmt.Errorf("could not resolve streamplace key: %w", err)
		}
		signingKey, err = a.Model.GetSigningKey(ctx, k.pub.DIDKey(), k.did)
	}
	if err != nil {
		return "", fmt.Errorf("signing key not found: %w", err)
	}
	if signingKey == nil {
		return "", fmt.Errorf("signing key not found")
	}
	return k.did, nil
}

func (a *StreamplaceAPI) MakeMediaSigner(ctx context.Context, keyStr string) (media.MediaSigner, error) {
	k, err := parseStreamKey(keyStr)
	if err != nil {
		return nil, err
	}
	signer, pub, did := k.signer, k.pub, k.did

	if did != "" {
		repo, err := a.ATSync.SyncBlueskyRepo(ctx, did, a.Model)
		if err != nil {
			return nil, fmt.Errorf("could not resolve streamplace key: %w", err)
		}
		err = a.CLI.StreamIsAllowed(repo.DID)
		if err != nil {
			return nil, fmt.Errorf("user is not allowed to stream: %w", err)
		}
		signingKey, err := a.Model.GetSigningKey(ctx, pub.DIDKey(), repo.DID)
		if err != nil {
			return nil, fmt.Errorf("signing key not found: %w", err)
		}
		if signingKey == nil {
			return nil, fmt.Errorf("signing key not found")
		}
	} else {
		did, err = k.bareKeyDID()
		if err != nil {
			return nil, err
		}
		err = a.CLI.StreamIsAllowed(did)
		if err != nil {
			return nil, fmt.Errorf("user is not allowed to stream: %w", err)
		}
	}

	ctx = log.WithLogValues(ctx, "did", did)
	err = a.checkBanned(ctx, did)
	if err != nil {
		return nil, err
	}

	mediaSigner, err := media.MakeMediaSigner(ctx, a.CLI, did, signer, a.Model)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization key (not valid secp256k1): %w", err)
	}

	return mediaSigner, nil
}

func (a *StreamplaceAPI) checkBanned(ctx context.Context, did string) error {
	labels, err := a.Model.GetActiveLabels(did)
	if err != nil {
		return fmt.Errorf("failed to get active labels: %w", err)
	}
	if atproto.IsBanned(labels...) {
		log.Error(ctx, "user is banned", "did", did)
		return fmt.Errorf("user is banned")
	}
	return nil
}
