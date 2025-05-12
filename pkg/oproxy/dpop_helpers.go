package oproxy

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/AxisCommunications/go-dpop"
	"github.com/golang-jwt/jwt/v5"
)

// all of this code borrowed from https://github.com/AxisCommunications/go-dpop
// MIT license
func keyFunc(t *jwt.Token) (interface{}, error) {
	// Return the required jwkHeader header. See https://datatracker.ietf.org/doc/html/rfc9449#section-4.2
	// Used to validate the signature of the DPoP proof.
	jwkHeader := t.Header["jwk"]
	if jwkHeader == nil {
		return nil, dpop.ErrMissingJWK
	}

	jwkMap, ok := jwkHeader.(map[string]interface{})
	if !ok {
		return nil, dpop.ErrMissingJWK
	}

	return parseJwk(jwkMap)
}

// Parses a JWK and inherently strips it of optional fields
func parseJwk(jwkMap map[string]interface{}) (interface{}, error) {
	// Ensure that JWK kty is present and is a string.
	kty, ok := jwkMap["kty"].(string)
	if !ok {
		return nil, dpop.ErrInvalidProof
	}
	switch kty {
	case "EC":
		// Ensure that the required fields are present and are strings.
		x, ok := jwkMap["x"].(string)
		if !ok {
			return nil, dpop.ErrInvalidProof
		}
		y, ok := jwkMap["y"].(string)
		if !ok {
			return nil, dpop.ErrInvalidProof
		}
		crv, ok := jwkMap["crv"].(string)
		if !ok {
			return nil, dpop.ErrInvalidProof
		}

		// Decode the coordinates from Base64.
		//
		// According to RFC 7518, they are Base64 URL unsigned integers.
		// https://tools.ietf.org/html/rfc7518#section-6.3
		xCoordinate, err := base64urlTrailingPadding(x)
		if err != nil {
			return nil, err
		}
		yCoordinate, err := base64urlTrailingPadding(y)
		if err != nil {
			return nil, err
		}

		// Read the specified curve of the key.
		var curve elliptic.Curve
		switch crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, dpop.ErrUnsupportedCurve
		}

		return &ecdsa.PublicKey{
			X:     big.NewInt(0).SetBytes(xCoordinate),
			Y:     big.NewInt(0).SetBytes(yCoordinate),
			Curve: curve,
		}, nil
	case "RSA":
		// Ensure that the required fields are present and are strings.
		e, ok := jwkMap["e"].(string)
		if !ok {
			return nil, dpop.ErrInvalidProof
		}
		n, ok := jwkMap["n"].(string)
		if !ok {
			return nil, dpop.ErrInvalidProof
		}

		// Decode the exponent and modulus from Base64.
		//
		// According to RFC 7518, they are Base64 URL unsigned integers.
		// https://tools.ietf.org/html/rfc7518#section-6.3
		exponent, err := base64urlTrailingPadding(e)
		if err != nil {
			return nil, err
		}
		modulus, err := base64urlTrailingPadding(n)
		if err != nil {
			return nil, err
		}
		return &rsa.PublicKey{
			N: big.NewInt(0).SetBytes(modulus),
			E: int(big.NewInt(0).SetBytes(exponent).Uint64()),
		}, nil
	case "OKP":
		// Ensure that the required fields are present and are strings.
		x, ok := jwkMap["x"].(string)
		if !ok {
			return nil, dpop.ErrInvalidProof
		}

		publicKey, err := base64urlTrailingPadding(x)
		if err != nil {
			return nil, err
		}

		return ed25519.PublicKey(publicKey), nil
	case "OCT":
		return nil, dpop.ErrUnsupportedKeyAlgorithm
	default:
		return nil, dpop.ErrUnsupportedKeyAlgorithm
	}
}

// Borrowed from MicahParks/keyfunc See: https://github.com/MicahParks/keyfunc/blob/master/keyfunc.go#L56
//
// base64urlTrailingPadding removes trailing padding before decoding a string from base64url. Some non-RFC compliant
// JWKS contain padding at the end values for base64url encoded public keys.
//
// Trailing padding is required to be removed from base64url encoded keys.
// RFC 7517 Section 1.1 defines base64url the same as RFC 7515 Section 2:
// https://datatracker.ietf.org/doc/html/rfc7517#section-1.1
// https://datatracker.ietf.org/doc/html/rfc7515#section-2
func base64urlTrailingPadding(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	return base64.RawURLEncoding.DecodeString(s)
}

// Strips eventual optional members of a JWK in order to be able to compute the thumbprint of it
// https://datatracker.ietf.org/doc/html/rfc7638#section-3.2
func getThumbprintableJwkJSONbytes(jwk map[string]interface{}) ([]byte, error) {
	minimalJwk, err := parseJwk(jwk)
	if err != nil {
		return nil, err
	}
	jwkHeaderJSONBytes, err := getKeyStringRepresentation(minimalJwk)
	if err != nil {
		return nil, err
	}
	return jwkHeaderJSONBytes, nil
}

// Returns the string representation of a key in JSON format.
func getKeyStringRepresentation(key interface{}) ([]byte, error) {
	var keyParts interface{}
	switch key := key.(type) {
	case *ecdsa.PublicKey:
		// Calculate the size of the byte array representation of an elliptic curve coordinate
		// and ensure that the byte array representation of the key is padded correctly.
		bits := key.Curve.Params().BitSize
		keyCurveBytesSize := bits/8 + bits%8

		keyParts = map[string]interface{}{
			"kty": "EC",
			"crv": key.Curve.Params().Name,
			"x":   base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, keyCurveBytesSize))),
			"y":   base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, keyCurveBytesSize))),
		}
	case *rsa.PublicKey:
		keyParts = map[string]interface{}{
			"kty": "RSA",
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		}
	case ed25519.PublicKey:
		keyParts = map[string]interface{}{
			"kty": "OKP",
			"crv": "Ed25519",
			"x":   base64.RawURLEncoding.EncodeToString(key),
		}
	default:
		return nil, dpop.ErrUnsupportedKeyAlgorithm
	}

	return json.Marshal(keyParts)
}
