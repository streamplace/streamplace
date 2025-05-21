package oproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
)

func (o *OProxy) HandleOAuthAuthorize(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleOAuthAuthorize")
	defer span.End()
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	requestURI := c.QueryParam("request_uri")
	if requestURI == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "request_uri is required")
	}
	clientID := c.QueryParam("client_id")
	if clientID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "client_id is required")
	}
	redirectURL, redirectErr := o.Authorize(ctx, requestURI, clientID)
	if redirectErr != nil {
		// we're a redirect; if we fail we need to send the user back
		jkt, _, err := parseURN(requestURI)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse URN: %s", err))
		}

		session, err := o.getOAuthSession(jkt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to load OAuth session jkt=%s: %s", jkt, err))
		}

		if session == nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("no session found for jkt=%s", jkt))
		}

		u, err := url.Parse(session.DownstreamRedirectURI)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse downstream redirect URI: %s", err))
		}
		q := u.Query()
		q.Set("error", "authorize_failed")
		q.Set("error_description", redirectErr.Error())
		u.RawQuery = q.Encode()
		return c.Redirect(http.StatusTemporaryRedirect, u.String())
	}
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// downstream --> upstream transition; attempt to send user to the upstream auth server
func (o *OProxy) Authorize(ctx context.Context, requestURI, clientID string) (string, *echo.HTTPError) {
	downstreamMeta, err := o.GetDownstreamMetadata("")
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to get downstream metadata: %s", err))
	}
	if downstreamMeta.ClientID != clientID {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("client ID mismatch: %s != %s", downstreamMeta.ClientID, clientID))
	}

	jkt, _, err := parseURN(requestURI)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse URN: %s", err))
	}

	session, err := o.getOAuthSession(jkt)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to load OAuth session jkt=%s: %s", jkt, err))
	}

	if session == nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("no session found for jkt=%s", jkt))
	}

	if session.Status() != OAuthSessionStatePARCreated {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("session is not in par-created state: %s", session.Status()))
	}

	if session.DownstreamPARRequestURI != requestURI {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("request URI mismatch: %s != %s", session.DownstreamPARRequestURI, requestURI))
	}

	now := time.Now()
	session.DownstreamPARUsedAt = &now
	err = o.updateOAuthSession(jkt, session)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to update OAuth session: %s", err))
	}

	upstreamMeta := o.GetUpstreamMetadata()
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   o.upstreamJWK,
		ClientId:    upstreamMeta.ClientID,
		RedirectUri: upstreamMeta.RedirectURIs[0],
	})
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create OAuth client: %s", err))
	}

	did, err := ResolveHandle(ctx, session.Handle)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to resolve handle '%s': %s", session.DID, err))
	}

	service, err := ResolveService(ctx, did)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to resolve service for DID '%s': %s", did, err))
	}

	authserver, err := oclient.ResolvePdsAuthServer(ctx, service)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to resolve PDS auth server for service '%s': %s", service, err))
	}

	authmeta, err := oclient.FetchAuthServerMetadata(ctx, authserver)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to fetch auth server metadata from '%s': %s", authserver, err))
	}

	k, err := helpers.GenerateKey(nil)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to generate DPoP key: %s", err))
	}

	state := makeState(jkt)

	opts := oauth.ParAuthRequestOpts{
		State: state,
	}
	parResp, err := oclient.SendParAuthRequest(ctx, authserver, authmeta, session.Handle, upstreamMeta.Scope, k, opts)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to send PAR auth request to '%s': %s", authserver, err))
	}

	jwkJSON, err := json.Marshal(k)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to marshal DPoP key to JSON: %s", err))
	}

	u, err := url.Parse(authmeta.AuthorizationEndpoint)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse auth server metadata: %s", err))
	}
	u.RawQuery = fmt.Sprintf("client_id=%s&request_uri=%s", url.QueryEscape(upstreamMeta.ClientID), parResp.RequestUri)
	str := u.String()

	session.DID = did
	session.PDSUrl = service
	session.UpstreamState = parResp.State
	session.UpstreamAuthServerIssuer = authserver
	session.UpstreamPKCEVerifier = parResp.PkceVerifier
	session.UpstreamDPoPNonce = parResp.DpopAuthserverNonce
	session.UpstreamDPoPPrivateJWK = string(jwkJSON)

	err = o.updateOAuthSession(jkt, session)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to update OAuth session: %s", err))
	}

	return str, nil
}
