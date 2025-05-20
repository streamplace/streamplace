package oproxy

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/AxisCommunications/go-dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
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

func (o *OProxy) HandleOAuthToken(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleOAuthToken")
	defer span.End()
	var tokenRequest TokenRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&tokenRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", err))
	}

	dpopHeader := c.Request().Header.Get("DPoP")
	if dpopHeader == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "DPoP header is required")
	}

	res, err := o.Token(ctx, &tokenRequest, dpopHeader)
	if err != nil {
		return err
	}
	jkt, _, err := getJKT(dpopHeader)
	if err != nil {
		return err
	}
	sess, err := o.getOAuthSession(jkt)
	if err != nil {
		return err
	}
	if sess == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "session not found")
	}
	nonces := generateValidNonces(sess.DownstreamDPoPNoncePad, time.Now())
	c.Response().Header().Set("DPoP-Nonce", nonces[0])

	return c.JSON(http.StatusOK, res)
}

func (o *OProxy) Token(ctx context.Context, tokenRequest *TokenRequest, dpopHeader string) (*TokenResponse, error) {
	proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/token"}, dpop.ParseOptions{
		Nonce:      "",
		TimeWindow: &dpopTimeWindow,
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid DPoP proof")
	}

	jkt := proof.PublicKey()
	session, err := o.getOAuthSession(jkt)
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

func (o *OProxy) generateJWT(session *OAuthSession) (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	downstreamMeta, err := o.GetDownstreamMetadata("")
	if err != nil {
		return "", err
	}
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
