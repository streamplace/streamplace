package spxrpc

import (
	"io"
	"strconv"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	appbsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	placestream "stream.place/streamplace/pkg/streamplace"
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
	var out *appbsky.ActorDefs_ProfileViewDetailed
	var handleErr error
	// func (s *Server) handleAppBskyActorGetProfile(ctx context.Context,actor string) (*appbsky.ActorDefs_ProfileViewDetailed, error)
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
	var out *appbsky.FeedGetFeedSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetFeedSkeleton(ctx context.Context,cursor string,feed string,limit int) (*appbsky.FeedGetFeedSkeleton_Output, error)
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
	e.GET("/xrpc/com.atproto.sync.getRepo", s.HandleComAtprotoSyncGetRepo)
	e.GET("/xrpc/com.atproto.sync.listRepos", s.HandleComAtprotoSyncListRepos)
	return nil
}

func (s *Server) HandleComAtprotoIdentityRefreshIdentity(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityRefreshIdentity")
	defer span.End()

	var body comatproto.IdentityRefreshIdentity_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.IdentityDefs_IdentityInfo
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityRefreshIdentity(ctx context.Context,body *comatproto.IdentityRefreshIdentity_Input) (*comatproto.IdentityDefs_IdentityInfo, error)
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
	var out *comatproto.IdentityResolveHandle_Output
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityResolveHandle(ctx context.Context,handle string) (*comatproto.IdentityResolveHandle_Output, error)
	out, handleErr = s.handleComAtprotoIdentityResolveHandle(ctx, handle)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoModerationCreateReport(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoModerationCreateReport")
	defer span.End()

	var body comatproto.ModerationCreateReport_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.ModerationCreateReport_Output
	var handleErr error
	// func (s *Server) handleComAtprotoModerationCreateReport(ctx context.Context,body *comatproto.ModerationCreateReport_Input) (*comatproto.ModerationCreateReport_Output, error)
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
	var out *comatproto.RepoDescribeRepo_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoDescribeRepo(ctx context.Context,repo string) (*comatproto.RepoDescribeRepo_Output, error)
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
	var out *comatproto.RepoGetRecord_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoGetRecord(ctx context.Context,cid string,collection string,repo string,rkey string) (*comatproto.RepoGetRecord_Output, error)
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
	var out *comatproto.RepoListRecords_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoListRecords(ctx context.Context,collection string,cursor string,limit int,repo string,reverse *bool) (*comatproto.RepoListRecords_Output, error)
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
	var out *comatproto.RepoUploadBlob_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoUploadBlob(ctx context.Context,r io.Reader,contentType string) (*comatproto.RepoUploadBlob_Output, error)
	out, handleErr = s.handleComAtprotoRepoUploadBlob(ctx, body, contentType)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerDescribeServer(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerDescribeServer")
	defer span.End()
	var out *comatproto.ServerDescribeServer_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerDescribeServer(ctx context.Context) (*comatproto.ServerDescribeServer_Output, error)
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

func (s *Server) HandleComAtprotoSyncGetRepo(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetRepo")
	defer span.End()
	did := c.QueryParam("did")
	since := c.QueryParam("since")
	var out io.Reader
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetRepo(ctx context.Context,did string,since string) (io.Reader, error)
	out, handleErr = s.handleComAtprotoSyncGetRepo(ctx, did, since)
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
	var out *comatproto.SyncListRepos_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncListRepos(ctx context.Context,cursor string,limit int) (*comatproto.SyncListRepos_Output, error)
	out, handleErr = s.handleComAtprotoSyncListRepos(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersGamesGamesgamesgamesgames(e *echo.Echo) error {
	return nil
}

func (s *Server) RegisterHandlersPlaceStream(e *echo.Echo) error {
	e.GET("/xrpc/place.stream.badge.getIssuedBadges", s.HandlePlaceStreamBadgeGetIssuedBadges)
	e.GET("/xrpc/place.stream.badge.getValidBadges", s.HandlePlaceStreamBadgeGetValidBadges)
	e.POST("/xrpc/place.stream.branding.deleteBlob", s.HandlePlaceStreamBrandingDeleteBlob)
	e.GET("/xrpc/place.stream.branding.getBlob", s.HandlePlaceStreamBrandingGetBlob)
	e.GET("/xrpc/place.stream.branding.getBranding", s.HandlePlaceStreamBrandingGetBranding)
	e.POST("/xrpc/place.stream.branding.updateBlob", s.HandlePlaceStreamBrandingUpdateBlob)
	e.GET("/xrpc/place.stream.broadcast.getBroadcaster", s.HandlePlaceStreamBroadcastGetBroadcaster)
	e.GET("/xrpc/place.stream.config.getEnv", s.HandlePlaceStreamConfigGetEnv)
	e.GET("/xrpc/place.stream.game.getGame", s.HandlePlaceStreamGameGetGame)
	e.GET("/xrpc/place.stream.game.search", s.HandlePlaceStreamGameSearch)
	e.GET("/xrpc/place.stream.graph.getFollowingUser", s.HandlePlaceStreamGraphGetFollowingUser)
	e.GET("/xrpc/place.stream.ingest.getIngestUrls", s.HandlePlaceStreamIngestGetIngestUrls)
	e.POST("/xrpc/place.stream.live.denyTeleport", s.HandlePlaceStreamLiveDenyTeleport)
	e.GET("/xrpc/place.stream.live.getLiveUsers", s.HandlePlaceStreamLiveGetLiveUsers)
	e.GET("/xrpc/place.stream.live.getProfileCard", s.HandlePlaceStreamLiveGetProfileCard)
	e.GET("/xrpc/place.stream.live.getRecommendations", s.HandlePlaceStreamLiveGetRecommendations)
	e.GET("/xrpc/place.stream.live.getSegments", s.HandlePlaceStreamLiveGetSegments)
	e.GET("/xrpc/place.stream.live.searchActorsTypeahead", s.HandlePlaceStreamLiveSearchActorsTypeahead)
	e.POST("/xrpc/place.stream.live.startLivestream", s.HandlePlaceStreamLiveStartLivestream)
	e.POST("/xrpc/place.stream.live.stopLivestream", s.HandlePlaceStreamLiveStopLivestream)
	e.POST("/xrpc/place.stream.media.createUpload", s.HandlePlaceStreamMediaCreateUpload)
	e.GET("/xrpc/place.stream.media.getUploadStatus", s.HandlePlaceStreamMediaGetUploadStatus)
	e.GET("/xrpc/place.stream.media.getVideo", s.HandlePlaceStreamMediaGetVideo)
	e.GET("/xrpc/place.stream.media.getVideoList", s.HandlePlaceStreamMediaGetVideoList)
	e.POST("/xrpc/place.stream.media.publishVideo", s.HandlePlaceStreamMediaPublishVideo)
	e.POST("/xrpc/place.stream.moderation.createBlock", s.HandlePlaceStreamModerationCreateBlock)
	e.POST("/xrpc/place.stream.moderation.createGate", s.HandlePlaceStreamModerationCreateGate)
	e.POST("/xrpc/place.stream.moderation.createPin", s.HandlePlaceStreamModerationCreatePin)
	e.POST("/xrpc/place.stream.moderation.deleteBlock", s.HandlePlaceStreamModerationDeleteBlock)
	e.POST("/xrpc/place.stream.moderation.deleteGate", s.HandlePlaceStreamModerationDeleteGate)
	e.POST("/xrpc/place.stream.moderation.deletePin", s.HandlePlaceStreamModerationDeletePin)
	e.POST("/xrpc/place.stream.moderation.updateLivestream", s.HandlePlaceStreamModerationUpdateLivestream)
	e.POST("/xrpc/place.stream.multistream.createTarget", s.HandlePlaceStreamMultistreamCreateTarget)
	e.POST("/xrpc/place.stream.multistream.deleteTarget", s.HandlePlaceStreamMultistreamDeleteTarget)
	e.GET("/xrpc/place.stream.multistream.listTargets", s.HandlePlaceStreamMultistreamListTargets)
	e.POST("/xrpc/place.stream.multistream.putTarget", s.HandlePlaceStreamMultistreamPutTarget)
	e.GET("/xrpc/place.stream.playback.getPlaybackServer", s.HandlePlaceStreamPlaybackGetPlaybackServer)
	e.GET("/xrpc/place.stream.playback.getVideoBlob", s.HandlePlaceStreamPlaybackGetVideoBlob)
	e.GET("/xrpc/place.stream.playback.getVideoPlaylist", s.HandlePlaceStreamPlaybackGetVideoPlaylist)
	e.POST("/xrpc/place.stream.playback.whep", s.HandlePlaceStreamPlaybackWhep)
	e.POST("/xrpc/place.stream.server.createWebhook", s.HandlePlaceStreamServerCreateWebhook)
	e.POST("/xrpc/place.stream.server.deleteStorage", s.HandlePlaceStreamServerDeleteStorage)
	e.POST("/xrpc/place.stream.server.deleteWebhook", s.HandlePlaceStreamServerDeleteWebhook)
	e.GET("/xrpc/place.stream.server.getServerTime", s.HandlePlaceStreamServerGetServerTime)
	e.GET("/xrpc/place.stream.server.getStorage", s.HandlePlaceStreamServerGetStorage)
	e.GET("/xrpc/place.stream.server.getWebhook", s.HandlePlaceStreamServerGetWebhook)
	e.GET("/xrpc/place.stream.server.listWebhooks", s.HandlePlaceStreamServerListWebhooks)
	e.POST("/xrpc/place.stream.server.updateWebhook", s.HandlePlaceStreamServerUpdateWebhook)
	e.POST("/xrpc/place.stream.server.upsertStorage", s.HandlePlaceStreamServerUpsertStorage)
	return nil
}

func (s *Server) HandlePlaceStreamBadgeGetIssuedBadges(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBadgeGetIssuedBadges")
	defer span.End()
	streamer := c.QueryParam("streamer")
	var out *placestream.BadgeGetIssuedBadges_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamBadgeGetIssuedBadges(ctx context.Context,streamer string) (*placestream.BadgeGetIssuedBadges_Output, error)
	out, handleErr = s.handlePlaceStreamBadgeGetIssuedBadges(ctx, streamer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamBadgeGetValidBadges(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBadgeGetValidBadges")
	defer span.End()
	streamer := c.QueryParam("streamer")
	var out *placestream.BadgeGetValidBadges_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamBadgeGetValidBadges(ctx context.Context,streamer string) (*placestream.BadgeGetValidBadges_Output, error)
	out, handleErr = s.handlePlaceStreamBadgeGetValidBadges(ctx, streamer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamBrandingDeleteBlob(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBrandingDeleteBlob")
	defer span.End()

	var body placestream.BrandingDeleteBlob_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.BrandingDeleteBlob_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamBrandingDeleteBlob(ctx context.Context,body *placestream.BrandingDeleteBlob_Input) (*placestream.BrandingDeleteBlob_Output, error)
	out, handleErr = s.handlePlaceStreamBrandingDeleteBlob(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamBrandingGetBlob(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBrandingGetBlob")
	defer span.End()
	broadcaster := c.QueryParam("broadcaster")
	key := c.QueryParam("key")
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamBrandingGetBlob(ctx context.Context,broadcaster string,key string) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamBrandingGetBlob(ctx, broadcaster, key)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandlePlaceStreamBrandingGetBranding(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBrandingGetBranding")
	defer span.End()
	broadcaster := c.QueryParam("broadcaster")
	var out *placestream.BrandingGetBranding_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamBrandingGetBranding(ctx context.Context,broadcaster string) (*placestream.BrandingGetBranding_Output, error)
	out, handleErr = s.handlePlaceStreamBrandingGetBranding(ctx, broadcaster)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamBrandingUpdateBlob(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBrandingUpdateBlob")
	defer span.End()

	var body placestream.BrandingUpdateBlob_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.BrandingUpdateBlob_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamBrandingUpdateBlob(ctx context.Context,body *placestream.BrandingUpdateBlob_Input) (*placestream.BrandingUpdateBlob_Output, error)
	out, handleErr = s.handlePlaceStreamBrandingUpdateBlob(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamBroadcastGetBroadcaster(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBroadcastGetBroadcaster")
	defer span.End()
	var out *placestream.BroadcastGetBroadcaster_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamBroadcastGetBroadcaster(ctx context.Context) (*placestream.BroadcastGetBroadcaster_Output, error)
	out, handleErr = s.handlePlaceStreamBroadcastGetBroadcaster(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamConfigGetEnv(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamConfigGetEnv")
	defer span.End()
	var out *placestream.ConfigGetEnv_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamConfigGetEnv(ctx context.Context) (*placestream.ConfigGetEnv_Output, error)
	out, handleErr = s.handlePlaceStreamConfigGetEnv(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamGameGetGame(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamGameGetGame")
	defer span.End()
	uri := c.QueryParam("uri")
	var out *placestream.GameGetGame_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamGameGetGame(ctx context.Context,uri string) (*placestream.GameGetGame_Output, error)
	out, handleErr = s.handlePlaceStreamGameGetGame(ctx, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamGameSearch(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamGameSearch")
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
		limit = 20
	}
	q := c.QueryParam("q")
	var out *placestream.GameSearch_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamGameSearch(ctx context.Context,cursor string,limit int,q string) (*placestream.GameSearch_Output, error)
	out, handleErr = s.handlePlaceStreamGameSearch(ctx, cursor, limit, q)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamGraphGetFollowingUser(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamGraphGetFollowingUser")
	defer span.End()
	subjectDID := c.QueryParam("subjectDID")
	userDID := c.QueryParam("userDID")
	var out *placestream.GraphGetFollowingUser_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamGraphGetFollowingUser(ctx context.Context,subjectDID string,userDID string) (*placestream.GraphGetFollowingUser_Output, error)
	out, handleErr = s.handlePlaceStreamGraphGetFollowingUser(ctx, subjectDID, userDID)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamIngestGetIngestUrls(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamIngestGetIngestUrls")
	defer span.End()
	var out *placestream.IngestGetIngestUrls_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamIngestGetIngestUrls(ctx context.Context) (*placestream.IngestGetIngestUrls_Output, error)
	out, handleErr = s.handlePlaceStreamIngestGetIngestUrls(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamLiveDenyTeleport(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveDenyTeleport")
	defer span.End()

	var body placestream.LiveDenyTeleport_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.LiveDenyTeleport_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveDenyTeleport(ctx context.Context,body *placestream.LiveDenyTeleport_Input) (*placestream.LiveDenyTeleport_Output, error)
	out, handleErr = s.handlePlaceStreamLiveDenyTeleport(ctx, &body)
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
	var out *placestream.LiveGetLiveUsers_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetLiveUsers(ctx context.Context,before string,limit int) (*placestream.LiveGetLiveUsers_Output, error)
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

func (s *Server) HandlePlaceStreamLiveGetRecommendations(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveGetRecommendations")
	defer span.End()
	userDID := c.QueryParam("userDID")
	var out *placestream.LiveGetRecommendations_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetRecommendations(ctx context.Context,userDID string) (*placestream.LiveGetRecommendations_Output, error)
	out, handleErr = s.handlePlaceStreamLiveGetRecommendations(ctx, userDID)
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
	var out *placestream.LiveGetSegments_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetSegments(ctx context.Context,before string,limit int,userDID string) (*placestream.LiveGetSegments_Output, error)
	out, handleErr = s.handlePlaceStreamLiveGetSegments(ctx, before, limit, userDID)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamLiveSearchActorsTypeahead(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveSearchActorsTypeahead")
	defer span.End()

	var limit int
	if p := c.QueryParam("limit"); p != "" {
		var err error
		limit, err = strconv.Atoi(p)
		if err != nil {
			return err
		}
	} else {
		limit = 10
	}
	q := c.QueryParam("q")
	var out *placestream.LiveSearchActorsTypeahead_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveSearchActorsTypeahead(ctx context.Context,limit int,q string) (*placestream.LiveSearchActorsTypeahead_Output, error)
	out, handleErr = s.handlePlaceStreamLiveSearchActorsTypeahead(ctx, limit, q)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamLiveStartLivestream(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveStartLivestream")
	defer span.End()

	var body placestream.LiveStartLivestream_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.LiveStartLivestream_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveStartLivestream(ctx context.Context,body *placestream.LiveStartLivestream_Input) (*placestream.LiveStartLivestream_Output, error)
	out, handleErr = s.handlePlaceStreamLiveStartLivestream(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamLiveStopLivestream(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveStopLivestream")
	defer span.End()

	var body placestream.LiveStopLivestream_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.LiveStopLivestream_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveStopLivestream(ctx context.Context,body *placestream.LiveStopLivestream_Input) (*placestream.LiveStopLivestream_Output, error)
	out, handleErr = s.handlePlaceStreamLiveStopLivestream(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMediaCreateUpload(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMediaCreateUpload")
	defer span.End()

	var body placestream.MediaCreateUpload_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.MediaCreateUpload_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMediaCreateUpload(ctx context.Context,body *placestream.MediaCreateUpload_Input) (*placestream.MediaCreateUpload_Output, error)
	out, handleErr = s.handlePlaceStreamMediaCreateUpload(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMediaGetUploadStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMediaGetUploadStatus")
	defer span.End()
	uploadId := c.QueryParam("uploadId")
	var out *placestream.MediaGetUploadStatus_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMediaGetUploadStatus(ctx context.Context,uploadId string) (*placestream.MediaGetUploadStatus_Output, error)
	out, handleErr = s.handlePlaceStreamMediaGetUploadStatus(ctx, uploadId)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMediaGetVideo(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMediaGetVideo")
	defer span.End()
	uri := c.QueryParam("uri")
	var out *placestream.MediaGetVideo_VideoView
	var handleErr error
	// func (s *Server) handlePlaceStreamMediaGetVideo(ctx context.Context,uri string) (*placestream.MediaGetVideo_VideoView, error)
	out, handleErr = s.handlePlaceStreamMediaGetVideo(ctx, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMediaGetVideoList(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMediaGetVideoList")
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
		limit = 25
	}
	repo := c.QueryParam("repo")
	var out *placestream.MediaGetVideoList_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMediaGetVideoList(ctx context.Context,cursor string,limit int,repo string) (*placestream.MediaGetVideoList_Output, error)
	out, handleErr = s.handlePlaceStreamMediaGetVideoList(ctx, cursor, limit, repo)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMediaPublishVideo(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMediaPublishVideo")
	defer span.End()

	var body placestream.MediaPublishVideo_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.MediaPublishVideo_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMediaPublishVideo(ctx context.Context,body *placestream.MediaPublishVideo_Input) (*placestream.MediaPublishVideo_Output, error)
	out, handleErr = s.handlePlaceStreamMediaPublishVideo(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamModerationCreateBlock(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationCreateBlock")
	defer span.End()

	var body placestream.ModerationCreateBlock_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationCreateBlock_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationCreateBlock(ctx context.Context,body *placestream.ModerationCreateBlock_Input) (*placestream.ModerationCreateBlock_Output, error)
	out, handleErr = s.handlePlaceStreamModerationCreateBlock(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamModerationCreateGate(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationCreateGate")
	defer span.End()

	var body placestream.ModerationCreateGate_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationCreateGate_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationCreateGate(ctx context.Context,body *placestream.ModerationCreateGate_Input) (*placestream.ModerationCreateGate_Output, error)
	out, handleErr = s.handlePlaceStreamModerationCreateGate(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamModerationCreatePin(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationCreatePin")
	defer span.End()

	var body placestream.ModerationCreatePin_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationCreatePin_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationCreatePin(ctx context.Context,body *placestream.ModerationCreatePin_Input) (*placestream.ModerationCreatePin_Output, error)
	out, handleErr = s.handlePlaceStreamModerationCreatePin(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamModerationDeleteBlock(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationDeleteBlock")
	defer span.End()

	var body placestream.ModerationDeleteBlock_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationDeleteBlock_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationDeleteBlock(ctx context.Context,body *placestream.ModerationDeleteBlock_Input) (*placestream.ModerationDeleteBlock_Output, error)
	out, handleErr = s.handlePlaceStreamModerationDeleteBlock(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamModerationDeleteGate(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationDeleteGate")
	defer span.End()

	var body placestream.ModerationDeleteGate_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationDeleteGate_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationDeleteGate(ctx context.Context,body *placestream.ModerationDeleteGate_Input) (*placestream.ModerationDeleteGate_Output, error)
	out, handleErr = s.handlePlaceStreamModerationDeleteGate(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamModerationDeletePin(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationDeletePin")
	defer span.End()

	var body placestream.ModerationDeletePin_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationDeletePin_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationDeletePin(ctx context.Context,body *placestream.ModerationDeletePin_Input) (*placestream.ModerationDeletePin_Output, error)
	out, handleErr = s.handlePlaceStreamModerationDeletePin(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamModerationUpdateLivestream(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationUpdateLivestream")
	defer span.End()

	var body placestream.ModerationUpdateLivestream_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationUpdateLivestream_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationUpdateLivestream(ctx context.Context,body *placestream.ModerationUpdateLivestream_Input) (*placestream.ModerationUpdateLivestream_Output, error)
	out, handleErr = s.handlePlaceStreamModerationUpdateLivestream(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMultistreamCreateTarget(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMultistreamCreateTarget")
	defer span.End()

	var body placestream.MultistreamCreateTarget_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.MultistreamDefs_TargetView
	var handleErr error
	// func (s *Server) handlePlaceStreamMultistreamCreateTarget(ctx context.Context,body *placestream.MultistreamCreateTarget_Input) (*placestream.MultistreamDefs_TargetView, error)
	out, handleErr = s.handlePlaceStreamMultistreamCreateTarget(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMultistreamDeleteTarget(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMultistreamDeleteTarget")
	defer span.End()

	var body placestream.MultistreamDeleteTarget_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.MultistreamDeleteTarget_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMultistreamDeleteTarget(ctx context.Context,body *placestream.MultistreamDeleteTarget_Input) (*placestream.MultistreamDeleteTarget_Output, error)
	out, handleErr = s.handlePlaceStreamMultistreamDeleteTarget(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMultistreamListTargets(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMultistreamListTargets")
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
		limit = 50
	}
	var out *placestream.MultistreamListTargets_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMultistreamListTargets(ctx context.Context,cursor string,limit int) (*placestream.MultistreamListTargets_Output, error)
	out, handleErr = s.handlePlaceStreamMultistreamListTargets(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamMultistreamPutTarget(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMultistreamPutTarget")
	defer span.End()

	var body placestream.MultistreamPutTarget_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.MultistreamDefs_TargetView
	var handleErr error
	// func (s *Server) handlePlaceStreamMultistreamPutTarget(ctx context.Context,body *placestream.MultistreamPutTarget_Input) (*placestream.MultistreamDefs_TargetView, error)
	out, handleErr = s.handlePlaceStreamMultistreamPutTarget(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamPlaybackGetPlaybackServer(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamPlaybackGetPlaybackServer")
	defer span.End()
	stream := c.QueryParam("stream")
	var out *placestream.PlaybackGetPlaybackServer_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamPlaybackGetPlaybackServer(ctx context.Context,stream string) (*placestream.PlaybackGetPlaybackServer_Output, error)
	out, handleErr = s.handlePlaceStreamPlaybackGetPlaybackServer(ctx, stream)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamPlaybackGetVideoBlob(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamPlaybackGetVideoBlob")
	defer span.End()
	cid := c.QueryParam("cid")
	did := c.QueryParam("did")
	sid := c.QueryParam("sid")
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamPlaybackGetVideoBlob(ctx context.Context,cid string,did string,sid string) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamPlaybackGetVideoBlob(ctx, cid, did, sid)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "video/mp4", out)
}

func (s *Server) HandlePlaceStreamPlaybackGetVideoPlaylist(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamPlaybackGetVideoPlaylist")
	defer span.End()

	var end *int
	if p := c.QueryParam("end"); p != "" {
		end_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		end = &end_val
	}
	sid := c.QueryParam("sid")

	var start *int
	if p := c.QueryParam("start"); p != "" {
		start_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		start = &start_val
	}
	track := c.QueryParam("track")
	uri := c.QueryParam("uri")
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamPlaybackGetVideoPlaylist(ctx context.Context,end *int,sid string,start *int,track string,uri string) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamPlaybackGetVideoPlaylist(ctx, end, sid, start, track, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandlePlaceStreamPlaybackWhep(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamPlaybackWhep")
	defer span.End()
	rendition := c.QueryParam("rendition")
	streamer := c.QueryParam("streamer")
	body := c.Request().Body
	contentType := c.Request().Header.Get("Content-Type")
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamPlaybackWhep(ctx context.Context,rendition string,streamer string,r io.Reader,contentType string) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamPlaybackWhep(ctx, rendition, streamer, body, contentType)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandlePlaceStreamServerCreateWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerCreateWebhook")
	defer span.End()

	var body placestream.ServerCreateWebhook_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ServerCreateWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerCreateWebhook(ctx context.Context,body *placestream.ServerCreateWebhook_Input) (*placestream.ServerCreateWebhook_Output, error)
	out, handleErr = s.handlePlaceStreamServerCreateWebhook(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerDeleteStorage(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerDeleteStorage")
	defer span.End()
	var out *placestream.ServerDeleteStorage_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerDeleteStorage(ctx context.Context) (*placestream.ServerDeleteStorage_Output, error)
	out, handleErr = s.handlePlaceStreamServerDeleteStorage(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerDeleteWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerDeleteWebhook")
	defer span.End()

	var body placestream.ServerDeleteWebhook_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ServerDeleteWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerDeleteWebhook(ctx context.Context,body *placestream.ServerDeleteWebhook_Input) (*placestream.ServerDeleteWebhook_Output, error)
	out, handleErr = s.handlePlaceStreamServerDeleteWebhook(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerGetServerTime(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerGetServerTime")
	defer span.End()
	var out *placestream.ServerGetServerTime_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerGetServerTime(ctx context.Context) (*placestream.ServerGetServerTime_Output, error)
	out, handleErr = s.handlePlaceStreamServerGetServerTime(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerGetStorage(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerGetStorage")
	defer span.End()
	var out *placestream.ServerGetStorage_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerGetStorage(ctx context.Context) (*placestream.ServerGetStorage_Output, error)
	out, handleErr = s.handlePlaceStreamServerGetStorage(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerGetWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerGetWebhook")
	defer span.End()
	id := c.QueryParam("id")
	var out *placestream.ServerGetWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerGetWebhook(ctx context.Context,id string) (*placestream.ServerGetWebhook_Output, error)
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
	var out *placestream.ServerListWebhooks_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerListWebhooks(ctx context.Context,active *bool,cursor string,event string,limit int) (*placestream.ServerListWebhooks_Output, error)
	out, handleErr = s.handlePlaceStreamServerListWebhooks(ctx, active, cursor, event, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerUpdateWebhook(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerUpdateWebhook")
	defer span.End()

	var body placestream.ServerUpdateWebhook_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ServerUpdateWebhook_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerUpdateWebhook(ctx context.Context,body *placestream.ServerUpdateWebhook_Input) (*placestream.ServerUpdateWebhook_Output, error)
	out, handleErr = s.handlePlaceStreamServerUpdateWebhook(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamServerUpsertStorage(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamServerUpsertStorage")
	defer span.End()

	var body placestream.ServerUpsertStorage_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ServerUpsertStorage_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerUpsertStorage(ctx context.Context,body *placestream.ServerUpsertStorage_Input) (*placestream.ServerUpsertStorage_Output, error)
	out, handleErr = s.handlePlaceStreamServerUpsertStorage(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersToolsOzone(e *echo.Echo) error {
	return nil
}
