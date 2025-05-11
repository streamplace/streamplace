package oproxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/atproto"
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
	redirectURL, err := o.Return(ctx, code, iss, state)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// TokenRequest represents the structure of an OAuth token request

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
	return c.JSON(http.StatusOK, res)
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
