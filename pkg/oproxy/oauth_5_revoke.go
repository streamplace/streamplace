package oproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/AxisCommunications/go-dpop"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
)

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

func (o *OProxy) Revoke(ctx context.Context, dpopHeader string, revokeRequest *RevokeRequest) error {
	proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/revoke"}, dpop.ParseOptions{
		Nonce:      "",
		TimeWindow: &dpopTimeWindow,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid DPoP proof")
	}

	session, err := o.getOAuthSession(proof.PublicKey())
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
