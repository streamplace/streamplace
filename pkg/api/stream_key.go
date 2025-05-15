package api

import (
	"context"
	"crypto"
	"fmt"

	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/mr-tron/base58"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func (a *StreamplaceAPI) MakeMediaSigner(ctx context.Context, keyStr string) (media.MediaSigner, error) {
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
	var signer crypto.Signer = key.ToECDSA()
	pub, err := priv.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("invalid authorization key (could not parse as atproto): %w", err)
	}

	did := string(didBytes)

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
		atkey, err := atproto.ParsePubKey(signer.Public())
		if err != nil {
			return nil, fmt.Errorf("invalid authorization key (not valid secp256k1): %w", err)
		}
		did = atkey.DIDKey()
		err = a.CLI.StreamIsAllowed(did)
		if err != nil {
			return nil, fmt.Errorf("user is not allowed to stream: %w", err)
		}
	}

	ctx = log.WithLogValues(ctx, "did", did)

	var mediaSigner media.MediaSigner
	if a.CLI.ExternalSigning {
		mediaSigner, err = media.MakeMediaSignerExt(ctx, a.CLI, did, addrBytes)
	} else {
		mediaSigner, err = media.MakeMediaSigner(ctx, a.CLI, did, signer)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid authorization key (not valid secp256k1): %w", err)
	}

	return mediaSigner, nil
}
