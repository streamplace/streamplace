package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"go.opentelemetry.io/otel"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamGraphGetNotificationPreference(ctx context.Context, repoDID string) (*placestreamtypes.GraphGetNotificationPreference_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamGraphGetNotificationPreference")
	defer span.End()

	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if _, err := syntax.ParseDID(repoDID); repoDID == "" || err != nil {
		return nil, fmt.Errorf("missing or invalid repo DID")
	}
	pref, err := s.statefulDB.GetNotificationPreference(ctx, session.DID, repoDID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification preference: %w", err)
	}
	enabled := true
	if pref != nil {
		enabled = pref.Enabled
	}
	return &placestreamtypes.GraphGetNotificationPreference_Output{Enabled: enabled}, nil
}

func (s *Server) handlePlaceStreamGraphSetNotificationPreference(ctx context.Context, body *placestreamtypes.GraphSetNotificationPreference_Input) (*placestreamtypes.GraphSetNotificationPreference_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamGraphSetNotificationPreference")
	defer span.End()

	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if _, err := syntax.ParseDID(body.RepoDID); body.RepoDID == "" || err != nil {
		return nil, fmt.Errorf("missing or invalid repo DID")
	}
	if err := s.statefulDB.SetNotificationPreference(ctx, session.DID, &placestreamtypes.GraphNotificationPreference{
		RepoDID: body.RepoDID,
		Enabled: body.Enabled,
	}); err != nil {
		return nil, fmt.Errorf("failed to set notification preference: %w", err)
	}
	return &placestreamtypes.GraphSetNotificationPreference_Output{}, nil
}
