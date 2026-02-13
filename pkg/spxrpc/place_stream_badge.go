package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/badges"
	placestream "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamBadgeGetValidBadges(ctx context.Context, streamer string) (*placestream.BadgeGetValidBadges_Output, error) {
	// Get authenticated user's DID from OAuth session
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// Get valid badges using shared badge logic
	badgeList, err := badges.GetValidBadges(ctx, session.DID, streamer, s.cli.MyDID(), s.model)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to get valid badges")
	}

	return &placestream.BadgeGetValidBadges_Output{
		Badges: badgeList,
	}, nil
}
