package oproxy

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/AxisCommunications/go-dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func boolPtr(b bool) *bool {
	return &b
}

func codeUUID(prefix string) string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%s", prefix, uu.String())
}

var urnPrefix = "urn:ietf:params:oauth:request_uri:"

const UUID_LENGTH = 37

func makeURN(jkt string) string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s%s-%s", urnPrefix, uu.String(), jkt)
}

// urn --> jkt, uu
func parseURN(urn string) (string, string, error) {
	if !strings.HasPrefix(urn, urnPrefix) {
		return "", "", fmt.Errorf("invalid URN: %s", urn)
	}
	withoutPrefix := urn[len(urnPrefix):]
	uu := withoutPrefix[:UUID_LENGTH]
	suffix := withoutPrefix[UUID_LENGTH:]
	return suffix, uu, nil
}

func makeState(jkt string) string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%s", uu.String(), jkt)
}

func parseState(state string) (string, string, error) {
	if len(state) < UUID_LENGTH {
		return "", "", fmt.Errorf("invalid state: %s", state)
	}
	uu := state[:UUID_LENGTH]
	suffix := state[UUID_LENGTH:]
	return suffix, uu, nil
}

func makeNoncePad() string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("noncepad-%s", uu.String())
}

// returns jkt, nonce, error
func getJKT(dpopJWT string) (string, *dpop.ProofTokenClaims, error) {
	var claims dpop.ProofTokenClaims
	token, err := jwt.ParseWithClaims(dpopJWT, &claims, keyFunc)
	if err != nil {
		return "", nil, err
	}
	jwk, ok := token.Header["jwk"].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("missing jwk in DPoP JWT header")
	}
	jwkJSONbytes, err := getThumbprintableJwkJSONbytes(jwk)
	if err != nil {
		// keyFunc used with parseWithClaims should ensure that this can not happen but better safe than sorry.
		return "", nil, errors.Join(dpop.ErrInvalidProof, err)
	}
	h := sha256.New()
	_, err = h.Write(jwkJSONbytes)
	if err != nil {
		return "", nil, errors.Join(dpop.ErrInvalidProof, err)
	}
	b64URLjwkHash := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return b64URLjwkHash, &claims, nil
}
