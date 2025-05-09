package atproto

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/config"
)

// {
//   "jti": "tok-a29d8e13a3a0fc774f0b46a389627405",
//   "sub": "did:plc:dkh4rwafdcda4ko7lewe43ml",
//   "exp": 1746760220,
//   "iat": 1746756620,
//   "cnf": {
//     "jkt": "8zHiYKJXbQ_r3G9U4BR9d_95TF53cGmwVwh8ry8g3LM"
//   },
//   "aud": "did:web:milkcap.us-west.host.bsky.network",
//   "scope": "atproto transition:generic",
//   "client_id": "https://stream.place/api/atproto-oauth/web",
//   "iss": "https://bsky.social"
// }

func GenerateAccessJWT(cli *config.CLI) (string, error) {
	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"jti":       uu.String(),
		"iat":       time.Now().Unix(),
		"exp":       time.Now().Add(time.Hour * 24).Unix(),
		"iss":       "https://longos.iameli.link",
		"aud":       "did:web:longos.iameli.link",
		"scope":     "atproto transition:generic",
		"client_id": "https://longos.iameli.link/api/atproto-oauth/web",
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(cli.AccessJWK)

	var rawKey any
	if err := cli.AccessJWK.Raw(&rawKey); err != nil {
		return "", err
	}

	tokenString, err = token.SignedString(rawKey)

	if err != nil {
		return "", err
	}

	return tokenString, nil
}
