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
	RefreshToken string `json:"refresh_token"`
}

type RevokeRequest struct {
	Token    string `json:"token"`
	ClientID string `json:"client_id"`
}

var OAuthTokenExpiry = time.Hour * 24

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

	if session.DownstreamAuthorizationCode != tokenRequest.Code {
		return nil, fmt.Errorf("invalid authorization code")
	}

	accessToken, err := generateJWT(cli, par.JKT, session.RepoDID)
	if err != nil {
		return nil, fmt.Errorf("could not generate access token: %w", err)
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("could not generate refresh token: %w", err)
	}

	session.DownstreamAccessToken = accessToken
	session.DownstreamRefreshToken = refreshToken
	session.DownstreamDPoPJKT = par.JKT

	err = mod.UpdateOAuthSession(session)
	if err != nil {
		return nil, fmt.Errorf("could not update downstream session: %w", err)
	}

	return session, nil
}

func HandleOAuthRefreshToken(ctx context.Context, cli *config.CLI, tokenRequest *TokenRequest, mod model.Model) (*model.OAuthSession, error) {
	session, err := mod.GetOAuthSessionByDownstreamRefreshToken(tokenRequest.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("could not get downstream session: %w", err)
	}

	if session == nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	newJWT, err := generateJWT(cli, session.DownstreamDPoPJKT, session.RepoDID)
	if err != nil {
		return nil, fmt.Errorf("could not generate new access token: %w", err)
	}

	session.DownstreamAccessToken = newJWT
	err = mod.UpdateOAuthSession(session)
	if err != nil {
		return nil, fmt.Errorf("could not update downstream session: %w", err)
	}

	return session, nil
}

func HandleOAuthRevoke(ctx context.Context, cli *config.CLI, revokeRequest *RevokeRequest, mod model.Model) error {
	session, err := mod.GetOAuthSessionByDownstreamAccessToken(revokeRequest.Token)
	if err != nil {
		return fmt.Errorf("could not get downstream session: %w", err)
	}

	now := time.Now()
	session.RevokedAt = &now
	err = mod.UpdateOAuthSession(session)
	if err != nil {
		return fmt.Errorf("could not update downstream session: %w", err)
	}

	return nil
}

func generateJWT(cli *config.CLI, jkt string, did string) (string, error) {
	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"jti": uu.String(),
		"sub": did,
		"exp": now.Add(OAuthTokenExpiry).Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"cnf": map[string]any{
			"jkt": jkt,
		},
		"aud":       "did:web:longos.iameli.link",
		"scope":     "atproto transition:generic",
		"client_id": "https://longos.iameli.link/api/atproto-oauth/web",
		"iss":       "https://longos.iameli.link",
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

func generateRefreshToken() (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("refresh-%s", uu.String()), nil
}

func generateAuthorizationCode() (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("code-%s", uu.String()), nil
}
