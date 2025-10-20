package spkey

import (
	"crypto"
	"fmt"

	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/mr-tron/base58"
)

// returns private key, public key, error
func GenerateStreamKey() (*atcrypto.PrivateKeyK256, *atcrypto.PublicKeyK256, error) {
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		return nil, nil, err
	}
	pub, err := priv.PublicKey()
	if err != nil {
		return nil, nil, err
	}

	return priv, pub.(*atcrypto.PublicKeyK256), nil
}

func GenerateStreamKeyForDID(did string) (string, *atcrypto.PublicKeyK256, error) {
	priv, pub, err := GenerateStreamKey()
	if err != nil {
		return "", nil, err
	}
	didBytes := []byte(did)
	combinedBytes := append(priv.Bytes(), didBytes...)
	multibaseKey := base58.Encode(combinedBytes)
	return multibaseKey, pub, nil
}

func KeyToSigner(priv *atcrypto.PrivateKeyK256) (crypto.Signer, error) {
	addrBytes := priv.Bytes()
	key, _ := secp256k1.PrivKeyFromBytes(addrBytes)
	if key == nil {
		return nil, fmt.Errorf("invalid authorization key (not valid secp256k1)")
	}
	var signer crypto.Signer = key.ToECDSA()
	return signer, nil
}
