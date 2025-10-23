package spxrpc

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/log"
)

func (s *Server) HandleWildcard(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleWildcard")
	defer span.End()

	// Get the last path segment in the URL
	path := c.Request().URL.Path
	segments := strings.Split(path, "/")
	lastSegment := segments[len(segments)-1]

	session, client := oatproxy.GetOAuthSession(ctx)

	// Allow app.bsky.* GET requests without authentication
	isAppBskyMethod := strings.HasPrefix(lastSegment, "app.bsky.")
	isGetRequest := c.Request().Method == "GET"

	// Require authentication for non-app.bsky methods and all POST requests
	if !isAppBskyMethod && session == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	var out map[string]any
	var xrpcType string
	var err error

	if isGetRequest {
		xrpcType = xrpc.Query
		queryParams := make(map[string]any)
		for k, v := range c.QueryParams() {
			for _, vv := range v {
				queryParams[k] = vv
			}
		}

		// make unauthed request if this is an app.bsky method
		if client != nil && !isAppBskyMethod {
			err = client.Do(ctx, xrpcType, "application/json", lastSegment, queryParams, nil, &out)
		} else if isAppBskyMethod {
			log.Log(ctx, "making unauthenticated request for app.bsky method", "method", lastSegment)
			err = makeUnauthenticatedRequest(ctx, "https://public.api.bsky.app", lastSegment, queryParams, &out)
		} else {
			// just in case?
			return echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found for non-app.bsky method")
		}
	} else {
		// POST/PUT/DELETE requests require authentication
		if session == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
		}

		xrpcType = xrpc.Procedure
		var body map[string]any
		if err := c.Bind(&body); err != nil {
			return c.JSON(http.StatusBadRequest, xrpc.XRPCError{ErrStr: "BadRequest", Message: fmt.Sprintf("invalid body: %s", err)})
		}
		err = client.Do(ctx, xrpcType, "application/json", lastSegment, nil, body, &out)
	}

	if err != nil {
		log.Error(ctx, "upstream xrpc error", "error", err)
		return err
	}

	return c.JSON(200, out)
}
