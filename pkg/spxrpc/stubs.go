package spxrpc

import (
	"strconv"

	appbskytypes "github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) RegisterHandlersAppBsky(e *echo.Echo) error {
	e.GET("/xrpc/app.bsky.feed.getFeedSkeleton", s.HandleAppBskyFeedGetFeedSkeleton)
	return nil
}

func (s *Server) HandleAppBskyFeedGetFeedSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetFeedSkeleton")
	defer span.End()
	cursor := c.QueryParam("cursor")
	feed := c.QueryParam("feed")

	var limit int
	if p := c.QueryParam("limit"); p != "" {
		var err error
		limit, err = strconv.Atoi(p)
		if err != nil {
			return err
		}
	} else {
		limit = 50
	}
	var out *appbskytypes.FeedGetFeedSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetFeedSkeleton(ctx context.Context,cursor string,feed string,limit int) (*appbskytypes.FeedGetFeedSkeleton_Output, error)
	out, handleErr = s.handleAppBskyFeedGetFeedSkeleton(ctx, cursor, feed, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersChatBsky(e *echo.Echo) error {
	return nil
}

func (s *Server) RegisterHandlersComAtproto(e *echo.Echo) error {
	return nil
}

func (s *Server) RegisterHandlersPlaceStream(e *echo.Echo) error {
	e.GET("/xrpc/place.stream.graph.getFollowingUser", s.HandlePlaceStreamGraphGetFollowingUser)
	return nil
}

func (s *Server) HandlePlaceStreamGraphGetFollowingUser(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamGraphGetFollowingUser")
	defer span.End()
	subjectDID := c.QueryParam("subjectDID")
	var out *placestreamtypes.GraphGetFollowingUser_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamGraphGetFollowingUser(ctx context.Context,subjectDID string) (*placestreamtypes.GraphGetFollowingUser_Output, error)
	out, handleErr = s.handlePlaceStreamGraphGetFollowingUser(ctx, subjectDID)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersToolsOzone(e *echo.Echo) error {
	return nil
}
