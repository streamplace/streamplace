package oproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

func (o *OProxy) Handler() http.Handler {
	o.e.GET("/.well-known/oauth-authorization-server", o.HandleOAuthAuthorizationServer)
	o.e.GET("/.well-known/oauth-protected-resource", o.HandleOAuthProtectedResource)
	o.e.POST("/oauth/par", o.HandleOAuthPAR)
	o.e.GET("/oauth/authorize", o.HandleOAuthAuthorize)
	o.e.GET("/oauth/return", o.HandleOAuthReturn)
	o.e.POST("/oauth/token", o.HandleOAuthToken)
	o.e.POST("/oauth/revoke", o.HandleOAuthRevoke)
	o.e.GET("/oauth/upstream/client-metadata.json", o.HandleClientMetadataUpstream)
	o.e.GET("/oauth/downstream/client-metadata.json", o.HandleClientMetadataDownstream)
	// prefer to handle this by returning in the metadata blob:
	// apiRouter.GET("/api/atproto-oauth/jwks.json", a.HandleJWKPublic(ctx))
	return o.e
}

func (o *OProxy) HandleOAuthAuthorizationServer(c echo.Context) error {
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set("Content-Type", "application/json")
	c.Response().WriteHeader(200)
	json.NewEncoder(c.Response().Writer).Encode(generateOAuthServerMetadata("longos.iameli.link"))
	return nil
}

func (o *OProxy) HandleClientMetadataUpstream(c echo.Context) error {
	meta := o.GetUpstreamMetadata()
	return c.JSON(200, meta)
}

func (o *OProxy) HandleClientMetadataDownstream(c echo.Context) error {
	meta := o.GetDownstreamMetadata()
	return c.JSON(200, meta)
}

func (o *OProxy) HandleOAuthProtectedResource(c echo.Context) error {
	return c.JSON(200, map[string]interface{}{
		"resource": fmt.Sprintf("https://%s", o.host),
		"authorization_servers": []string{
			fmt.Sprintf("https://%s", o.host),
		},
		"scopes_supported": []string{},
		"bearer_methods_supported": []string{
			"header",
		},
		"resource_documentation": "https://atproto.com",
	})
}

func (o *OProxy) HandleOAuthPAR(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleOAuthPAR")
	defer span.End()
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	var par PAR
	if err := json.NewDecoder(c.Request().Body).Decode(&par); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	dpopHeader := c.Request().Header.Get("DPoP")
	if dpopHeader == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "DPoP header is required")
	}

	resp, err := o.NewPAR(ctx, &par, dpopHeader)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusCreated, resp)
}

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
	redirectURL, err := o.Authorize(ctx, clientID, requestURI)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (o *OProxy) HandleOAuthReturn(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleOAuthReturn")
	defer span.End()
	code := c.QueryParam("code")
	iss := c.QueryParam("iss")
	state := c.QueryParam("state")
	upstreamSession, err := atproto.HandleOauthReturn(ctx, a.CLI, code, iss, state, a.Model)
	if err != nil {
		apierrors.WriteHTTPInternalServerError(w, "could not handle oauth return", err)
		return
	}
	if upstreamSession == nil {
		log.Error(ctx, "no upstream session found", "upstreamSession", upstreamSession)
		apierrors.WriteHTTPBadRequest(w, "no upstream session found", nil)
		return
	}
	if upstreamSession.DownstreamPAR == nil {
		log.Error(ctx, "no downstream par found", "upstreamSession", upstreamSession)
		apierrors.WriteHTTPBadRequest(w, "no downstream par found", nil)
		return
	}

	u, err := url.Parse("https://longos.iameli.link/login")
	if err != nil {
		apierrors.WriteHTTPInternalServerError(w, "could not parse redirect url", err)
		return
	}
	q := u.Query()
	q.Set("iss", "https://longos.iameli.link")
	q.Set("state", upstreamSession.DownstreamPAR.State)
	q.Set("code", upstreamSession.DownstreamAuthorizationCode)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}

// TokenRequest represents the structure of an OAuth token request

func (o *OProxy) HandleOAuthToken(c echo.Context) error {
	var tokenRequest atproto.TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&tokenRequest); err != nil {
		apierrors.WriteHTTPBadRequest(w, "invalid request", err)
		return
	}

	// Verify the token request parameters
	if tokenRequest.GrantType == "authorization_code" {
		a.handleAuthToken(w, r, tokenRequest)
		return
	} else if tokenRequest.GrantType == "refresh_token" {
		a.handleRefreshToken(w, r, tokenRequest)
		return
	}

	apierrors.WriteHTTPBadRequest(w, "unsupported grant type", nil)
	return nil
}

func (o *OProxy) HandleOAuthRevoke(c echo.Context) error {
	var revokeRequest atproto.RevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&revokeRequest); err != nil {
		apierrors.WriteHTTPBadRequest(w, "invalid request", err)
		return
	}
	err := atproto.HandleOAuthRevoke(r.Context(), a.CLI, &revokeRequest, a.Model)
	if err != nil {
		apierrors.WriteHTTPBadRequest(w, "could not handle oauth revoke", err)
		return
	}
	w.WriteHeader(200)
}

func (o *OProxy) handleAuthToken(c echo.Context) error {
	if tokenRequest.Code == "" || tokenRequest.CodeVerifier == "" {
		apierrors.WriteHTTPBadRequest(w, "missing required parameters", nil)
		return
	}

	session, err := atproto.HandleOAuthToken(r.Context(), a.CLI, &tokenRequest, a.Model)
	if err != nil {
		apierrors.WriteHTTPBadRequest(w, "could not handle oauth token", err)
		return
	}

	response := map[string]interface{}{
		"access_token":  session.DownstreamAccessToken,
		"token_type":    "DPoP",
		"refresh_token": session.DownstreamRefreshToken,
		"scope":         "atproto transition:generic",
		"expires_in":    atproto.OAuthTokenExpiry.Seconds(),
		"sub":           session.RepoDID,
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(response)
}

func (o *OProxy) handleRefreshToken(c echo.Context) error {
	session, err := atproto.HandleOAuthRefreshToken(r.Context(), a.CLI, &tokenRequest, a.Model)
	if err != nil {
		apierrors.WriteHTTPBadRequest(w, "could not handle oauth token", err)
		return
	}

	response := map[string]interface{}{
		"access_token":  session.DownstreamAccessToken,
		"token_type":    "DPoP",
		"refresh_token": session.DownstreamRefreshToken,
		"scope":         "atproto transition:generic",
		"expires_in":    atproto.OAuthTokenExpiry.Seconds(),
		"sub":           session.RepoDID,
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(response)
}
