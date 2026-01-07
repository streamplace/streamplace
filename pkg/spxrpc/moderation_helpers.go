package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/moderation"
)

// DelegatedModerationContext contains validated session and client for delegated moderation actions
type DelegatedModerationContext struct {
	ModeratorDID     string
	ModeratorSession *oatproxy.OAuthSession
	StreamerClient   *oatproxy.XrpcClient
	StreamerSession  *oatproxy.OAuthSession
}

// GetDelegatedModerationContext validates moderator OAuth, checks permission, and returns streamer client
// This consolidates the repeated pattern in all moderation handlers to eliminate duplication and fix security issues
func (s *Server) GetDelegatedModerationContext(
	ctx context.Context,
	streamerDID string,
	action string,
) (*DelegatedModerationContext, error) {

	// Step 1: Get and validate moderator OAuth session
	// NOTE: GetOAuthSession returns (*OAuthSession, *XrpcClient), not (*OAuthSession, error)
	moderatorSession, _ := oatproxy.GetOAuthSession(ctx)
	if moderatorSession == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	moderatorDID := moderatorSession.DID

	// Step 2: Check permission
	permChecker := moderation.NewPermissionChecker(s.model)
	if err := permChecker.CheckPermission(ctx, moderatorDID, streamerDID, action); err != nil {
		log.Warn(ctx, "permission denied", "moderator", moderatorDID, "streamer", streamerDID, "action", action, "error", err)
		return nil, echo.NewHTTPError(http.StatusForbidden, fmt.Sprintf("permission denied: %v", err))
	}

	// Step 3: Get streamer session
	streamerSession, err := s.statefulDB.GetSessionByDID(streamerDID)
	if err != nil {
		log.Error(ctx, "failed to get streamer session", "streamer", streamerDID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get streamer session: %v", err))
	}
	if streamerSession == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("session not found for streamer %s", streamerDID))
	}

	// Step 4: Refresh session if needed
	streamerSession, err = s.op.RefreshIfNeeded(streamerSession)
	if err != nil {
		log.Error(ctx, "failed to refresh streamer session", "streamer", streamerDID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to refresh session: %v", err))
	}
	if streamerSession == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "streamer session not found after refresh")
	}

	// Step 5: Get XRPC client for streamer
	client, err := s.op.GetXrpcClient(streamerSession)
	if err != nil {
		log.Error(ctx, "failed to get xrpc client", "streamer", streamerDID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get xrpc client: %v", err))
	}

	return &DelegatedModerationContext{
		ModeratorDID:     moderatorDID,
		ModeratorSession: moderatorSession,
		StreamerClient:   client,
		StreamerSession:  streamerSession,
	}, nil
}
