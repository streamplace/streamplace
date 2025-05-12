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

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	Sub          string `json:"sub"`
}

var OAuthTokenExpiry = time.Hour * 24

var dpopTimeWindow = time.Duration(30 * time.Second)

func (o *OProxy) Token(ctx context.Context, tokenRequest *TokenRequest, dpopHeader string) (*TokenResponse, error) {
	proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/token"}, dpop.ParseOptions{
		Nonce:      "",
		TimeWindow: &dpopTimeWindow,
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid DPoP proof")
	}

	jkt := proof.PublicKey()
	session, err := o.loadOAuthSession(jkt)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("could not get oauth session: %s", err))
	}

	if tokenRequest.GrantType == "authorization_code" {
		return o.AccessToken(ctx, tokenRequest, session)
	} else if tokenRequest.GrantType == "refresh_token" {
		return o.RefreshToken(ctx, tokenRequest, session)
	}
	return nil, echo.NewHTTPError(http.StatusBadRequest, "unsupported grant type")
}

func (o *OProxy) AccessToken(ctx context.Context, tokenRequest *TokenRequest, session *OAuthSession) (*TokenResponse, error) {
	if session.Status() != OAuthSessionStateDownstream {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("session is not in downstream state: %s", session.Status()))
	}

	// Hash the code verifier using SHA-256
	hasher := sha256.New()
	hasher.Write([]byte(tokenRequest.CodeVerifier))
	codeChallenge := hasher.Sum(nil)

	encodedChallenge := base64.RawURLEncoding.WithPadding(base64.NoPadding).EncodeToString(codeChallenge)

	if session.DownstreamCodeChallenge != encodedChallenge {
		return nil, fmt.Errorf("invalid code challenge")
	}

	if session.DownstreamAuthorizationCode != tokenRequest.Code {
		return nil, fmt.Errorf("invalid authorization code")
	}

	accessToken, err := o.generateJWT(session)
	if err != nil {
		return nil, fmt.Errorf("could not generate access token: %w", err)
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("could not generate refresh token: %w", err)
	}

	session.DownstreamAccessToken = accessToken
	session.DownstreamRefreshToken = refreshToken

	err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
	if err != nil {
		return nil, fmt.Errorf("could not update downstream session: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "DPoP",
		RefreshToken: refreshToken,
		Scope:        "atproto transition:generic",
		ExpiresIn:    int(OAuthTokenExpiry.Seconds()),
		Sub:          session.DID,
	}, nil
}

func (o *OProxy) RefreshToken(ctx context.Context, tokenRequest *TokenRequest, session *OAuthSession) (*TokenResponse, error) {

	if session.Status() != OAuthSessionStateReady {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "session is not in ready state")
	}

	if session.DownstreamRefreshToken != tokenRequest.RefreshToken {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid refresh token")
	}

	newJWT, err := o.generateJWT(session)
	if err != nil {
		return nil, fmt.Errorf("could not generate new access token: %w", err)
	}

	session.DownstreamAccessToken = newJWT
	err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
	if err != nil {
		return nil, fmt.Errorf("could not update downstream session: %w", err)
	}

	return &TokenResponse{
		AccessToken:  newJWT,
		TokenType:    "DPoP",
		RefreshToken: session.DownstreamRefreshToken,
		Scope:        "atproto transition:generic",
		ExpiresIn:    int(OAuthTokenExpiry.Seconds()),
		Sub:          session.DID,
	}, nil
}

func (o *OProxy) Revoke(ctx context.Context, dpopHeader string, revokeRequest *RevokeRequest) error {
	proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/revoke"}, dpop.ParseOptions{
		Nonce:      "",
		TimeWindow: &dpopTimeWindow,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid DPoP proof")
	}

	session, err := o.loadOAuthSession(proof.PublicKey())
	if err != nil {
		return fmt.Errorf("could not get downstream session: %w", err)
	}

	now := time.Now()
	session.RevokedAt = &now
	err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
	if err != nil {
		return fmt.Errorf("could not update downstream session: %w", err)
	}

	return nil
}

func (o *OProxy) generateJWT(session *OAuthSession) (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	downstreamMeta := o.GetDownstreamMetadata()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"jti": uu.String(),
		"sub": session.DID,
		"exp": now.Add(OAuthTokenExpiry).Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"cnf": map[string]any{
			"jkt": session.DownstreamDPoPJKT,
		},
		"aud":       fmt.Sprintf("did:web:%s", o.host),
		"scope":     downstreamMeta.Scope,
		"client_id": downstreamMeta.ClientID,
		"iss":       fmt.Sprintf("https://%s", o.host),
	})

	var rawKey any
	if err := o.downstreamJWK.Raw(&rawKey); err != nil {
		return "", err
	}

	tokenString, err := token.SignedString(rawKey)

	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (o *OProxy) DPoPNonceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		dpopHeader := c.Request().Header.Get("DPoP")
		if dpopHeader == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "missing DPoP header")
		}

		jkt, _, err := getJKT(dpopHeader)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		session, err := o.loadOAuthSession(jkt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		c.Set("session", session)
		return next(c)
	}
}

func (o *OProxy) ErrorHandlingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err == nil {
			return nil
		}
		_, ok := err.(*echo.HTTPError)
		if ok {
			return err
		}
		o.slog.Error("unhandled error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
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
		ApplicationType:         "web",
	}
	return meta
}

var ErrFirstNonce = echo.NewHTTPError(http.StatusBadRequest, "first time seeing this key, come back with a nonce")

func (o *OProxy) NewPAR(ctx context.Context, c echo.Context, par *PAR, dpopHeader string) (*PARResponse, error) {
	jkt, nonce, err := getJKT(dpopHeader)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to get JKT from DPoP header: %s", err))
	}
	session, err := o.loadOAuthSession(jkt)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to load OAuth session: %s", err))
	}
	// special case - if this is the first request, we need to send it back for a new nonce
	if session == nil {
		_, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/par"}, dpop.ParseOptions{
			Nonce:      nonce, // normally this would be bad! but on the first request we're revalidating nonce anyway
			TimeWindow: &dpopTimeWindow,
		})
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse DPoP header: %s", err))
		}
		newNonce := makeNonce()
		err = o.createOAuthSession(jkt, &OAuthSession{
			DownstreamDPoPJKT:   jkt,
			DownstreamDPoPNonce: newNonce,
		})
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create OAuth session: %s", err))
		}
		// come back later, nerd
		c.Response().Header().Set("DPoP-Nonce", newNonce)
		return nil, ErrFirstNonce
	}
	if session.DownstreamDPoPNonce != nonce {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid nonce")
	}
	proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/par"}, dpop.ParseOptions{
		Nonce:      session.DownstreamDPoPNonce,
		TimeWindow: &dpopTimeWindow,
	})
	// Check the error type to determine response
	if err != nil {
		// if ok := errors.Is(err, dpop.ErrInvalidProof); ok {
		// 	apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", nil)
		// 	return
		// }
		// apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", err)
		// return
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid DPoP proof: %s", err))
	}
	if proof.PublicKey() != jkt {
		panic("invalid code path: parsed DPoP proof twice and got different keys?!")
	}

	clientMetadata := o.GetDownstreamMetadata()
	if par.ClientID != clientMetadata.ClientID {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid client_id: expected %s, got %s", clientMetadata.ClientID, par.ClientID))
	}

	if !slices.Contains(clientMetadata.RedirectURIs, par.RedirectURI) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid redirect_uri: %s not in allowed URIs", par.RedirectURI))
	}

	if par.CodeChallengeMethod != "S256" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid code challenge method: expected S256, got %s", par.CodeChallengeMethod))
	}

	if par.ResponseMode != "query" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid response mode: expected query, got %s", par.ResponseMode))
	}

	if par.ResponseType != "code" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid response type: expected code, got %s", par.ResponseType))
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

	urn := makeURN(jkt)

	newNonce := makeNonce()

	err = o.updateOAuthSession(jkt, &OAuthSession{
		DownstreamDPoPJKT:       jkt,
		DownstreamDPoPNonce:     newNonce,
		DownstreamPARRequestURI: urn,
		DownstreamCodeChallenge: par.CodeChallenge,
		DownstreamState:         par.State,
		DID:                     par.LoginHint,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create oauth session: %w", err)
	}
	c.Response().Header().Set("DPoP-Nonce", newNonce)

	resp := &PARResponse{
		RequestURI: urn,
		ExpiresIn:  int(dpopTimeWindow.Seconds()),
	}

	return resp, nil
}
