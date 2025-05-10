package atproto

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	RedirectURI  string `json:"redirect_uri"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	ClientID     string `json:"client_id"`
}

// handle a request for a new downstream access token (must verify PKCE)
func HandleOAuthToken(ctx context.Context, cli *config.CLI, tokenRequest *TokenRequest, mod model.Model) (*model.OAuthSession, error) {

	// Hash the code verifier using SHA-256
	hasher := sha256.New()
	hasher.Write([]byte(tokenRequest.CodeVerifier))
	codeChallenge := hasher.Sum(nil)

	// Encode the hash in URL-safe base64 format
	// This removes padding and replaces + with - and / with _

	encodedChallenge := base64.RawURLEncoding.WithPadding(base64.NoPadding).EncodeToString(codeChallenge)

	// Look up the PAR using the code challenge
	par, err := mod.GetPARByCodeChallenge(encodedChallenge)
	if err != nil {
		return nil, fmt.Errorf("could not get par: %w", err)
	}

	if par.ExpiresAt.Before(time.Now()) {
		// todo: clean up the half-created session at this point?
		return nil, fmt.Errorf("par expired")
	}

	// get the session for this par
	session, err := mod.GetOAuthSessionByDownstreamPARID(par.ID)
	if err != nil {
		return nil, fmt.Errorf("could not get oauth session: %w", err)
	}

	accessToken, err := generateJWT(cli)
	if err != nil {
		return nil, fmt.Errorf("could not generate access token: %w", err)
	}

	refreshToken, err := generateRefreshToken(cli)
	if err != nil {
		return nil, fmt.Errorf("could not generate refresh token: %w", err)
	}

	session.DownstreamAccessToken = accessToken
	session.DownstreamRefreshToken = refreshToken

	err = mod.UpdateOAuthSession(session)
	if err != nil {
		return nil, fmt.Errorf("could not update downstream session: %w", err)
	}

	return session, nil

}

func generateJWT(cli *config.CLI) (string, error) {
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

func generateRefreshToken(cli *config.CLI) (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("refresh-%s", uu.String()), nil
}
