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

func (s *Server) handlePlaceStreamGraphGetNotificationPreference(ctx context.Context, streamerDID string, userDID string) (*placestreamtypes.GraphGetNotificationPreference_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamGraphGetNotificationPreference")
	defer span.End()

	if _, err := syntax.ParseDID(userDID); userDID == "" || err != nil {
		return nil, fmt.Errorf("missing or invalid user DID")
	}
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if session.DID != userDID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "cannot access another user's notification preferences")
	}

	if _, err := syntax.ParseDID(streamerDID); streamerDID == "" || err != nil {
		return nil, fmt.Errorf("missing or invalid streamer DID")
	}

	pref, err := s.model.GetNotificationPreference(ctx, userDID, streamerDID)
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

	if _, err := syntax.ParseDID(body.UserDID); body.UserDID == "" || err != nil {
		return nil, fmt.Errorf("missing or invalid user DID")
	}
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if session.DID != body.UserDID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "cannot access another user's notification preferences")
	}

	if _, err := syntax.ParseDID(body.StreamerDID); body.StreamerDID == "" || err != nil {
		return nil, fmt.Errorf("missing or invalid streamer DID")
	}

	if err := s.model.SetNotificationPreference(ctx, body.UserDID, body.StreamerDID, body.Enabled); err != nil {
		return nil, fmt.Errorf("failed to set notification preference: %w", err)
	}

	return &placestreamtypes.GraphSetNotificationPreference_Output{}, nil
}
