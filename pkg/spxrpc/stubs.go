package spxrpc

import (
	"io"
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
	e.POST("/xrpc/com.atproto.identity.refreshIdentity", s.HandleComAtprotoIdentityRefreshIdentity)
	e.GET("/xrpc/com.atproto.identity.resolveHandle", s.HandleComAtprotoIdentityResolveHandle)
	e.POST("/xrpc/com.atproto.moderation.createReport", s.HandleComAtprotoModerationCreateReport)
	e.GET("/xrpc/com.atproto.repo.describeRepo", s.HandleComAtprotoRepoDescribeRepo)
	e.GET("/xrpc/com.atproto.repo.getRecord", s.HandleComAtprotoRepoGetRecord)
	e.GET("/xrpc/com.atproto.repo.listRecords", s.HandleComAtprotoRepoListRecords)
	e.POST("/xrpc/com.atproto.repo.uploadBlob", s.HandleComAtprotoRepoUploadBlob)
	e.GET("/xrpc/com.atproto.server.describeServer", s.HandleComAtprotoServerDescribeServer)
	e.GET("/xrpc/com.atproto.sync.getRecord", s.HandleComAtprotoSyncGetRecord)
	e.GET("/xrpc/com.atproto.sync.listRepos", s.HandleComAtprotoSyncListRepos)
	return nil
}

func (s *Server) HandleComAtprotoIdentityRefreshIdentity(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityRefreshIdentity")
	defer span.End()

	var body comatprototypes.IdentityRefreshIdentity_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatprototypes.IdentityDefs_IdentityInfo
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityRefreshIdentity(ctx context.Context,body *comatprototypes.IdentityRefreshIdentity_Input) (*comatprototypes.IdentityDefs_IdentityInfo, error)
	out, handleErr = s.handleComAtprotoIdentityRefreshIdentity(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
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

func (s *Server) HandleComAtprotoModerationCreateReport(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoModerationCreateReport")
	defer span.End()

	var body comatprototypes.ModerationCreateReport_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatprototypes.ModerationCreateReport_Output
	var handleErr error
	// func (s *Server) handleComAtprotoModerationCreateReport(ctx context.Context,body *comatprototypes.ModerationCreateReport_Input) (*comatprototypes.ModerationCreateReport_Output, error)
	out, handleErr = s.handleComAtprotoModerationCreateReport(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoDescribeRepo(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoDescribeRepo")
	defer span.End()
	repo := c.QueryParam("repo")
	var out *comatprototypes.RepoDescribeRepo_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoDescribeRepo(ctx context.Context,repo string) (*comatprototypes.RepoDescribeRepo_Output, error)
	out, handleErr = s.handleComAtprotoRepoDescribeRepo(ctx, repo)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoGetRecord(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoGetRecord")
	defer span.End()
	cid := c.QueryParam("cid")
	collection := c.QueryParam("collection")
	repo := c.QueryParam("repo")
	rkey := c.QueryParam("rkey")
	var out *comatprototypes.RepoGetRecord_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoGetRecord(ctx context.Context,cid string,collection string,repo string,rkey string) (*comatprototypes.RepoGetRecord_Output, error)
	out, handleErr = s.handleComAtprotoRepoGetRecord(ctx, cid, collection, repo, rkey)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoListRecords(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoListRecords")
	defer span.End()
	collection := c.QueryParam("collection")
	cursor := c.QueryParam("cursor")

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
	repo := c.QueryParam("repo")

	var reverse *bool
	if p := c.QueryParam("reverse"); p != "" {
		reverse_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		reverse = &reverse_val
	}
	var out *comatprototypes.RepoListRecords_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoListRecords(ctx context.Context,collection string,cursor string,limit int,repo string,reverse *bool) (*comatprototypes.RepoListRecords_Output, error)
	out, handleErr = s.handleComAtprotoRepoListRecords(ctx, collection, cursor, limit, repo, reverse)
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

func (s *Server) HandleComAtprotoServerDescribeServer(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerDescribeServer")
	defer span.End()
	var out *comatprototypes.ServerDescribeServer_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerDescribeServer(ctx context.Context) (*comatprototypes.ServerDescribeServer_Output, error)
	out, handleErr = s.handleComAtprotoServerDescribeServer(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncGetRecord(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetRecord")
	defer span.End()
	collection := c.QueryParam("collection")
	did := c.QueryParam("did")
	rkey := c.QueryParam("rkey")
	var out io.Reader
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetRecord(ctx context.Context,collection string,did string,rkey string) (io.Reader, error)
	out, handleErr = s.handleComAtprotoSyncGetRecord(ctx, collection, did, rkey)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/vnd.ipld.car", out)
}

func (s *Server) HandleComAtprotoSyncListRepos(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncListRepos")
	defer span.End()
	cursor := c.QueryParam("cursor")

	var limit int
	if p := c.QueryParam("limit"); p != "" {
		var err error
		limit, err = strconv.Atoi(p)
		if err != nil {
			return err
		}
	} else {
		limit = 500
	}
	var out *comatprototypes.SyncListRepos_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncListRepos(ctx context.Context,cursor string,limit int) (*comatprototypes.SyncListRepos_Output, error)
	out, handleErr = s.handleComAtprotoSyncListRepos(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersPlaceStream(e *echo.Echo) error {
	e.GET("/xrpc/place.stream.graph.getFollowingUser", s.HandlePlaceStreamGraphGetFollowingUser)
	e.GET("/xrpc/place.stream.live.getLiveUsers", s.HandlePlaceStreamLiveGetLiveUsers)
	e.GET("/xrpc/place.stream.live.getProfileCard", s.HandlePlaceStreamLiveGetProfileCard)
	e.GET("/xrpc/place.stream.live.getSegments", s.HandlePlaceStreamLiveGetSegments)
	e.POST("/xrpc/place.stream.server.createWebhook", s.HandlePlaceStreamServerCreateWebhook)
	e.POST("/xrpc/place.stream.server.deleteWebhook", s.HandlePlaceStreamServerDeleteWebhook)
	e.GET("/xrpc/place.stream.server.getWebhook", s.HandlePlaceStreamServerGetWebhook)
	e.GET("/xrpc/place.stream.server.listWebhooks", s.HandlePlaceStreamServerListWebhooks)
	e.POST("/xrpc/place.stream.server.updateWebhook", s.HandlePlaceStreamServerUpdateWebhook)
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

func (s *Server) HandlePlaceStreamLiveGetLiveUsers(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveGetLiveUsers")
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
	var out *placestreamtypes.LiveGetLiveUsers_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetLiveUsers(ctx context.Context,before string,limit int) (*placestreamtypes.LiveGetLiveUsers_Output, error)
	out, handleErr = s.handlePlaceStreamLiveGetLiveUsers(ctx, before, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamLiveGetProfileCard(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveGetProfileCard")
	defer span.End()
	id := c.QueryParam("id")
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetProfileCard(ctx context.Context,id string) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamLiveGetProfileCard(ctx, id)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
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

func (s *Server) HandlePlaceStreamServerCreateWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerCreateWebhook")
	defer span.End()

	var body placestreamtypes.ServerCreateWebhook_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestreamtypes.ServerCreateWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerCreateWebhook(ctx context.Context,body *placestreamtypes.ServerCreateWebhook_Input) (*placestreamtypes.ServerCreateWebhook_Output, error)
	out, handleErr = s.handlePlaceStreamServerCreateWebhook(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerDeleteWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerDeleteWebhook")
	defer span.End()

	var body placestreamtypes.ServerDeleteWebhook_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestreamtypes.ServerDeleteWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerDeleteWebhook(ctx context.Context,body *placestreamtypes.ServerDeleteWebhook_Input) (*placestreamtypes.ServerDeleteWebhook_Output, error)
	out, handleErr = s.handlePlaceStreamServerDeleteWebhook(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerGetWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerGetWebhook")
	defer span.End()
	id := c.QueryParam("id")
	var out *placestreamtypes.ServerGetWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerGetWebhook(ctx context.Context,id string) (*placestreamtypes.ServerGetWebhook_Output, error)
	out, handleErr = s.handlePlaceStreamServerGetWebhook(ctx, id)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerListWebhooks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerListWebhooks")
	defer span.End()

	var active *bool
	if p := c.QueryParam("active"); p != "" {
		active_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		active = &active_val
	}
	cursor := c.QueryParam("cursor")
	event := c.QueryParam("event")

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
	var out *placestreamtypes.ServerListWebhooks_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerListWebhooks(ctx context.Context,active *bool,cursor string,event string,limit int) (*placestreamtypes.ServerListWebhooks_Output, error)
	out, handleErr = s.handlePlaceStreamServerListWebhooks(ctx, active, cursor, event, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerUpdateWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerUpdateWebhook")
	defer span.End()

	var body placestreamtypes.ServerUpdateWebhook_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestreamtypes.ServerUpdateWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerUpdateWebhook(ctx context.Context,body *placestreamtypes.ServerUpdateWebhook_Input) (*placestreamtypes.ServerUpdateWebhook_Output, error)
	out, handleErr = s.handlePlaceStreamServerUpdateWebhook(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersToolsOzone(e *echo.Echo) error {
	return nil
}
