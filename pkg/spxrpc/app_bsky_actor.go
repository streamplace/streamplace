package spxrpc

import (
	"context"
	"net/http"

	appbskytypes "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/oproxy"
)

func (s *Server) handleAppBskyActorGetProfile(ctx context.Context, actor string) (*appbskytypes.ActorDefs_ProfileViewDetailed, error) {
	session, client := oproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// brief check to make sure we can actually do stuff
	var out appbskytypes.ActorDefs_ProfileViewDetailed
	err := client.Do(ctx, xrpc.Query, "application/json", "app.bsky.actor.getProfile", map[string]any{"actor": actor}, nil, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}
