package spxrpc

import (
	"strconv"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	appbskytypes "github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) RegisterHandlersAppBsky(e *echo.Echo) error {
	e.GET("/xrpc/app.bsky.actor.getProfile", s.HandleAppBskyActorGetProfile)
	e.GET("/xrpc/app.bsky.feed.getFeedSkeleton", s.HandleAppBskyFeedGetFeedSkeleton)
	return nil
}

func (s *Server) HandleAppBskyActorGetProfile(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyActorGetProfile")
	defer span.End()
	actor := c.QueryParam("actor")
	var out *appbskytypes.ActorDefs_ProfileViewDetailed
	var handleErr error
	// func (s *Server) handleAppBskyActorGetProfile(ctx context.Context,actor string) (*appbskytypes.ActorDefs_ProfileViewDetailed, error)
	out, handleErr = s.handleAppBskyActorGetProfile(ctx, actor)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
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
	e.GET("/xrpc/com.atproto.identity.resolveHandle", s.HandleComAtprotoIdentityResolveHandle)
	e.POST("/xrpc/com.atproto.repo.uploadBlob", s.HandleComAtprotoRepoUploadBlob)
	return nil
}

func (s *Server) HandleComAtprotoIdentityResolveHandle(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityResolveHandle")
	defer span.End()
	handle := c.QueryParam("handle")
	var out *comatprototypes.IdentityResolveHandle_Output
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityResolveHandle(ctx context.Context,handle string) (*comatprototypes.IdentityResolveHandle_Output, error)
	out, handleErr = s.handleComAtprotoIdentityResolveHandle(ctx, handle)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoUploadBlob(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoUploadBlob")
	defer span.End()
	body := c.Request().Body
	contentType := c.Request().Header.Get("Content-Type")
	var out *comatprototypes.RepoUploadBlob_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoUploadBlob(ctx context.Context,r io.Reader,contentType string) (*comatprototypes.RepoUploadBlob_Output, error)
	out, handleErr = s.handleComAtprotoRepoUploadBlob(ctx, body, contentType)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersPlaceStream(e *echo.Echo) error {
	e.GET("/xrpc/place.stream.graph.getFollowingUser", s.HandlePlaceStreamGraphGetFollowingUser)
	e.GET("/xrpc/place.stream.live.getSegments", s.HandlePlaceStreamLiveGetSegments)
	return nil
}

func (s *Server) HandlePlaceStreamGraphGetFollowingUser(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamGraphGetFollowingUser")
	defer span.End()
	subjectDID := c.QueryParam("subjectDID")
	userDID := c.QueryParam("userDID")
	var out *placestreamtypes.GraphGetFollowingUser_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamGraphGetFollowingUser(ctx context.Context,subjectDID string,userDID string) (*placestreamtypes.GraphGetFollowingUser_Output, error)
	out, handleErr = s.handlePlaceStreamGraphGetFollowingUser(ctx, subjectDID, userDID)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamLiveGetSegments(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveGetSegments")
	defer span.End()
	before := c.QueryParam("before")

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
	userDID := c.QueryParam("userDID")
	var out *placestreamtypes.LiveGetSegments_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetSegments(ctx context.Context,before string,limit int,userDID string) (*placestreamtypes.LiveGetSegments_Output, error)
	out, handleErr = s.handlePlaceStreamLiveGetSegments(ctx, before, limit, userDID)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersToolsOzone(e *echo.Echo) error {
	return nil
}
