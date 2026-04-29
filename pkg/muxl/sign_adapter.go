package muxl

import (
	"crypto"
	"crypto/sha256"
	"encoding/asn1"
	"math/big"
)

// cryptoSigner is the crypto.Signer interface, aliased so SignerToCallback
// reads tidily without dragging the crypto import into muxl.go.
type cryptoSigner = crypto.Signer

// cryptoSHA256 is crypto.SHA256 as a SignerOpts value.
var cryptoSHA256 crypto.SignerOpts = crypto.SHA256

func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// derECDSAToRaw converts a DER-encoded ECDSA signature (the SEQUENCE { r,
// s } shape Go's crypto.Signer returns for ECDSA keys) to the fixed-width
// r||s format c2pa expects from CallbackSigner closures.
//
// byteLen is the curve's coordinate width: 32 for P-256/secp256k1, 48 for
// P-384, 66 for P-521. r and s are left-padded with zeros to that width.
//
// Returns ok=false if the input doesn't look like ASN.1 DER ECDSA — in
// that case the caller should pass the bytes through unmodified (e.g.
// RSA-PSS signatures aren't DER-shaped).
func derECDSAToRaw(der []byte, byteLen int) ([]byte, bool) {
	var sig struct{ R, S *big.Int }
	rest, err := asn1.Unmarshal(der, &sig)
	if err != nil || len(rest) != 0 || sig.R == nil || sig.S == nil {
		return nil, false
	}
	out := make([]byte, 2*byteLen)
	sig.R.FillBytes(out[:byteLen])
	sig.S.FillBytes(out[byteLen:])
	return out, true
}
