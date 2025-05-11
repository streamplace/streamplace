package oproxy

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/AxisCommunications/go-dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
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

func generateOAuthServerMetadata(host string) map[string]any {
	oauthServerMetadata := map[string]any{
		"issuer":                                         fmt.Sprintf("https://%s", host),
		"request_parameter_supported":                    true,
		"request_uri_parameter_supported":                true,
		"require_request_uri_registration":               true,
		"scopes_supported":                               []string{"atproto", "transition:generic", "transition:chat.bsky"},
		"subject_types_supported":                        []string{"public"},
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query", "fragment", "form_post"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"ui_locales_supported":                           []string{"en-US"},
		"display_values_supported":                       []string{"page", "popup", "touch"},
		"authorization_response_iss_parameter_supported": true,
		"request_object_encryption_alg_values_supported": []string{},
		"request_object_encryption_enc_values_supported": []string{},
		"jwks_uri":                              fmt.Sprintf("https://%s/oauth/jwks", host),
		"authorization_endpoint":                fmt.Sprintf("https://%s/oauth/authorize", host),
		"token_endpoint":                        fmt.Sprintf("https://%s/oauth/token", host),
		"token_endpoint_auth_methods_supported": []string{"none", "private_key_jwt"},
		"revocation_endpoint":                   fmt.Sprintf("https://%s/oauth/revoke", host),
		"introspection_endpoint":                fmt.Sprintf("https://%s/oauth/introspect", host),
		"pushed_authorization_request_endpoint": fmt.Sprintf("https://%s/oauth/par", host),
		"require_pushed_authorization_requests": true,
		"client_id_metadata_document_supported": true,
		"request_object_signing_alg_values_supported": []string{
			"RS256", "RS384", "RS512", "PS256", "PS384", "PS512",
			"ES256", "ES256K", "ES384", "ES512", "none",
		},
		"token_endpoint_auth_signing_alg_values_supported": []string{
			"RS256", "RS384", "RS512", "PS256", "PS384", "PS512",
			"ES256", "ES256K", "ES384", "ES512",
		},
		"dpop_signing_alg_values_supported": []string{
			"RS256", "RS384", "RS512", "PS256", "PS384", "PS512",
			"ES256", "ES256K", "ES384", "ES512",
		},
	}
	return oauthServerMetadata
}

func (o *OProxy) GetDownstreamMetadata() *OAuthClientMetadata {
	meta := &OAuthClientMetadata{
		ClientID:  fmt.Sprintf("https://%s/oauth/downstream/client-metadata.json", o.host),
		ClientURI: fmt.Sprintf("https://%s", o.host),
		// RedirectURIs:            []string{fmt.Sprintf("https://%s/login", host)},
		Scope:                   "atproto transition:generic",
		TokenEndpointAuthMethod: "none",
		ClientName:              "Streamplace",
		ResponseTypes:           []string{"code"},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		DPoPBoundAccessTokens:   boolPtr(true),
		RedirectURIs:            []string{fmt.Sprintf("https://%s/login", o.host)},
	}
	return meta
}

func (o *OProxy) NewPAR(ctx context.Context, par *PAR, dpopHeader string) (*PARResponse, error) {
	thirtySec := time.Duration(30 * time.Second)
	proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/api/oauth/par"}, dpop.ParseOptions{
		Nonce:      "",
		TimeWindow: &thirtySec,
	})
	// Check the error type to determine response
	if err != nil {
		// if ok := errors.Is(err, dpop.ErrInvalidProof); ok {
		// 	apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", nil)
		// 	return
		// }
		// apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", err)
		// return
		return nil, err
	}

	clientMetadata := o.GetDownstreamMetadata()
	if par.ClientID != clientMetadata.ClientID {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid client_id")
	}

	if !slices.Contains(clientMetadata.RedirectURIs, par.RedirectURI) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid redirect_uri")
	}

	if par.CodeChallengeMethod != "S256" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid code challenge method")
	}

	if par.ResponseMode != "query" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid response mode")
	}

	if par.ResponseType != "code" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid response type")
	}

	if par.Scope != o.scope {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid scope")
	}

	if par.LoginHint == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "login hint is required to find your PDS")
	}

	if par.State == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "state is required")
	}

	if par.Scope != o.scope {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid scope (expected %s, got %s)", o.scope, par.Scope))
	}

	// proof is valid, get public key to use as primary key of oauth session
	jkt := proof.PublicKey()

	urn := makeURN(jkt)

	err = o.createOAuthSession(jkt, &OAuthSession{
		DownstreamDPoPJKT:       jkt,
		DownstreamPARRequestURI: urn,
		DownstreamCodeChallenge: par.CodeChallenge,
		DownstreamState:         par.State,
		DID:                     par.LoginHint,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create oauth session: %w", err)
	}

	resp := &PARResponse{
		RequestURI: urn,
		ExpiresIn:  int(thirtySec.Seconds()),
	}

	return resp, nil
}
