package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// oatproxy checks every OAuth request's DPoP proof with golang-jwt's stock
// ES256 and an *ecdsa.PublicKey. golang-jwt keeps one process-wide table of
// signing methods, so any package in the binary can replace "ES256" from its
// init: indigo's atproto/auth does, with a method that only takes atcrypto
// keys, and every sign-in then fails with "key is of invalid type". This
// package's tests link nearly everything the node does, so a dependency that
// takes over the table again shows up here.
func TestJWTES256IsStock(t *testing.T) {
	_, ok := jwt.GetSigningMethod("ES256").(*jwt.SigningMethodECDSA)
	require.True(t, ok, "golang-jwt's ES256 was replaced by %T", jwt.GetSigningMethod("ES256"))

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	signed, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{"htm": "POST"}).SignedString(key)
	require.NoError(t, err)
	_, err = jwt.Parse(signed, func(*jwt.Token) (any, error) { return &key.PublicKey, nil })
	require.NoError(t, err, "a DPoP-style ES256 proof must verify with an *ecdsa.PublicKey")
}
