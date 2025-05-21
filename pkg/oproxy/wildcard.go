package oproxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/log"
)

func (o *OProxy) HandleWildcard(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleWildcard")
	defer span.End()

	session, client := GetOAuthSession(ctx)
	if session == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	var out map[string]any

	// Get the last path segment in the URL
	path := c.Request().URL.Path
	segments := strings.Split(path, "/")
	lastSegment := segments[len(segments)-1]

	var xrpcType xrpc.XRPCRequestType
	var err error
	if c.Request().Method == "GET" {
		xrpcType = xrpc.Query
		queryParams := make(map[string]any)
		for k, v := range c.QueryParams() {
			for _, vv := range v {
				queryParams[k] = vv
			}
		}
		err = client.Do(ctx, xrpcType, "application/json", lastSegment, queryParams, nil, &out)
	} else {
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
