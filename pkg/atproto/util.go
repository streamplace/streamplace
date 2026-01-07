package atproto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func ParsePubKey(pub crypto.PublicKey) (*atcrypto.PublicKeyK256, error) {
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not an ECDSA public key")
	}

	publicKeyBytes := elliptic.Marshal(ethcrypto.S256(), ecdsaPub.X, ecdsaPub.Y) //nolint:all
	atkey, err := atcrypto.ParsePublicUncompressedBytesK256(publicKeyBytes)
	if err != nil {
		return nil, err
	}
	return atkey, nil
}
