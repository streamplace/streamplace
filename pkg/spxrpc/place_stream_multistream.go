package spxrpc

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"go.opentelemetry.io/otel"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

var allowedSchemes = []string{"rtmp", "rtmps"}

func (s *Server) handlePlaceStreamMultistreamCreateTarget(ctx context.Context, body *placestreamtypes.MultistreamCreateTarget_Input) (*placestreamtypes.MultistreamDefs_TargetView, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handleComAtprotoRepoUploadBlob")
	defer span.End()

	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	u, err := url.Parse(body.MultistreamTarget.Url)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid multistream target URL")
	}
	if !slices.Contains(allowedSchemes, u.Scheme) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid multistream target scheme (must be rtmp or rtmps)")
	}
	return s.statefulDB.CreateMultistreamTarget(body, session.DID)
}
