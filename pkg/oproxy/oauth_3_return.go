package oproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/xrpc"
	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"go.opentelemetry.io/otel"
)

func (o *OProxy) HandleOAuthReturn(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleOAuthReturn")
	defer span.End()
	code := c.QueryParam("code")
	iss := c.QueryParam("iss")
	state := c.QueryParam("state")
	errorCode := c.QueryParam("error")
	errorDescription := c.QueryParam("error_description")
	var httpError *echo.HTTPError
	var redirectURL string
	if errorCode != "" {
		httpError = echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("%s (%s)", errorDescription, errorCode))
	} else {
		redirectURL, httpError = o.Return(ctx, code, iss, state)
	}
	if httpError != nil {
		// we're a redirect; if we fail we need to send the user back
		jkt, _, err := parseState(state)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse URN: %s", err))
		}

		session, err := o.getOAuthSession(jkt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to load OAuth session jkt=%s: %s", jkt, err))
		}

		u, err := url.Parse(session.DownstreamRedirectURI)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse downstream redirect URI: %s", err))
		}
		q := u.Query()
		q.Set("error", "return_failed")
		q.Set("error_description", httpError.Error())
		u.RawQuery = q.Encode()
		return c.Redirect(http.StatusTemporaryRedirect, u.String())
	}
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (o *OProxy) Return(ctx context.Context, code string, iss string, state string) (string, *echo.HTTPError) {
	upstreamMeta := o.GetUpstreamMetadata()
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   o.upstreamJWK,
		ClientId:    upstreamMeta.ClientID,
		RedirectUri: upstreamMeta.RedirectURIs[0],
	})

	jkt, _, err := parseState(state)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse state: %s", err))
	}

	session, err := o.getOAuthSession(jkt)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to get OAuth session: %s", err))
	}
	if session == nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("no OAuth session found for state: %s", state))
	}

	if session.Status() != OAuthSessionStateUpstream {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("session is not in upstream state: %s", session.Status()))
	}

	if session.UpstreamState != state {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("state mismatch: %s != %s", session.UpstreamState, state))
	}

	if iss != session.UpstreamAuthServerIssuer {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("issuer mismatch: %s != %s", iss, session.UpstreamAuthServerIssuer))
	}

	key, err := jwk.ParseKey([]byte(session.UpstreamDPoPPrivateJWK))
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to parse DPoP private JWK: %s", err))
	}

	itResp, err := oclient.InitialTokenRequest(ctx, code, iss, session.UpstreamPKCEVerifier, session.UpstreamDPoPNonce, key)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to request initial token: %s", err))
	}
	now := time.Now()

	if itResp.Sub != session.DID {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("sub mismatch: %s != %s", itResp.Sub, session.DID))
	}

	if itResp.Scope != upstreamMeta.Scope {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("scope mismatch: %s != %s", itResp.Scope, upstreamMeta.Scope))
	}

	downstreamCode, err := generateAuthorizationCode()
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to generate downstream code: %s", err))
	}

	expiry := now.Add(time.Second * time.Duration(itResp.ExpiresIn)).UTC()
	session.UpstreamAccessToken = itResp.AccessToken
	session.UpstreamAccessTokenExp = &expiry
	session.UpstreamRefreshToken = itResp.RefreshToken
	session.DownstreamAuthorizationCode = downstreamCode

	authArgs := &oauth.XrpcAuthedRequestArgs{
		Did:            session.DID,
		AccessToken:    session.UpstreamAccessToken,
		PdsUrl:         session.PDSUrl,
		Issuer:         session.UpstreamAuthServerIssuer,
		DpopPdsNonce:   session.UpstreamDPoPNonce,
		DpopPrivateJwk: key,
	}

	xrpcClient := &oauth.XrpcClient{
		OnDpopPdsNonceChanged: func(did, newNonce string) {},
	}

	// brief check to make sure we can actually do stuff
	var out atproto.ServerCheckAccountStatus_Output
	if err := xrpcClient.Do(ctx, authArgs, xrpc.Query, "application/json", "com.atproto.server.checkAccountStatus", nil, nil, &out); err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to check account status: %s", err))
	}

	err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to update OAuth session: %s", err))
	}

	u, err := url.Parse(session.DownstreamRedirectURI)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse downstream redirect URI: %s", err))
	}
	q := u.Query()
	q.Set("iss", fmt.Sprintf("https://%s", o.host))
	q.Set("state", session.DownstreamState)
	q.Set("code", session.DownstreamAuthorizationCode)
	u.RawQuery = q.Encode()

	return u.String(), nil
}
