package oproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
)

func (o *OProxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // todo: ehhhhhhhhhhhh
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,DPoP")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Expose-Headers", "DPoP-Nonce")
		o.e.ServeHTTP(w, r)
	})
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

func (o *OProxy) HandleJwksUpstream(c echo.Context) error {
	pubKey, err := o.upstreamJWK.PublicKey()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not get public key")
	}
	return c.JSON(200, helpers.CreateJwksResponseObject(pubKey))
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

	resp, err := o.NewPAR(ctx, c, &par, dpopHeader)
	if errors.Is(err, ErrFirstNonce) {
		res := map[string]interface{}{
			"error":             "use_dpop_nonce",
			"error_description": "Authorization server requires nonce in DPoP proof",
		}
		return c.JSON(http.StatusBadRequest, res)
	} else if err != nil {
		return err
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
	redirectURL, err := o.Authorize(ctx, requestURI, clientID)
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
	jkt, _, err := getJKT(dpopHeader)
	if err != nil {
		return err
	}
	sess, err := o.loadOAuthSession(jkt)
	if err != nil {
		return err
	}
	sess.DownstreamDPoPNonce = makeNonce()
	err = o.updateOAuthSession(sess.DownstreamDPoPJKT, sess)
	if err != nil {
		return err
	}
	c.Response().Header().Set("DPoP-Nonce", sess.DownstreamDPoPNonce)

	return c.JSON(http.StatusOK, res)
}

func (o *OProxy) HandleOAuthRevoke(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleOAuthRevoke")
	defer span.End()
	var revokeRequest RevokeRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&revokeRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", err))
	}
	dpopHeader := c.Request().Header.Get("DPoP")
	if dpopHeader == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "DPoP header is required")
	}
	err := o.Revoke(ctx, dpopHeader, &revokeRequest)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("could not handle oauth revoke: %s", err))
	}
	return c.JSON(http.StatusOK, map[string]interface{}{})
}
