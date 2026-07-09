package spxrpc

import (
	"io"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	appbsky "stream.place/streamplace/pkg/appbsky"
	comatproto "stream.place/streamplace/pkg/comatproto"
	gamesgamesgamesgamesgames "stream.place/streamplace/pkg/gamesgamesgamesgamesgames"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) RegisterHandlersAppbsky(e *echo.Echo) error {
	e.GET("/xrpc/app.bsky.actor.getPreferences", s.HandleAppBskyActorGetPreferences)
	e.GET("/xrpc/app.bsky.actor.getProfile", s.HandleAppBskyActorGetProfile)
	e.GET("/xrpc/app.bsky.actor.getProfiles", s.HandleAppBskyActorGetProfiles)
	e.GET("/xrpc/app.bsky.actor.getSuggestions", s.HandleAppBskyActorGetSuggestions)
	e.POST("/xrpc/app.bsky.actor.putPreferences", s.HandleAppBskyActorPutPreferences)
	e.GET("/xrpc/app.bsky.actor.searchActors", s.HandleAppBskyActorSearchActors)
	e.GET("/xrpc/app.bsky.actor.searchActorsTypeahead", s.HandleAppBskyActorSearchActorsTypeahead)
	e.POST("/xrpc/app.bsky.ageassurance.begin", s.HandleAppBskyAgeassuranceBegin)
	e.GET("/xrpc/app.bsky.ageassurance.getConfig", s.HandleAppBskyAgeassuranceGetConfig)
	e.GET("/xrpc/app.bsky.ageassurance.getState", s.HandleAppBskyAgeassuranceGetState)
	e.POST("/xrpc/app.bsky.bookmark.createBookmark", s.HandleAppBskyBookmarkCreateBookmark)
	e.POST("/xrpc/app.bsky.bookmark.deleteBookmark", s.HandleAppBskyBookmarkDeleteBookmark)
	e.GET("/xrpc/app.bsky.bookmark.getBookmarks", s.HandleAppBskyBookmarkGetBookmarks)
	e.GET("/xrpc/app.bsky.feed.describeFeedGenerator", s.HandleAppBskyFeedDescribeFeedGenerator)
	e.GET("/xrpc/app.bsky.feed.getActorFeeds", s.HandleAppBskyFeedGetActorFeeds)
	e.GET("/xrpc/app.bsky.feed.getActorLikes", s.HandleAppBskyFeedGetActorLikes)
	e.GET("/xrpc/app.bsky.feed.getAuthorFeed", s.HandleAppBskyFeedGetAuthorFeed)
	e.GET("/xrpc/app.bsky.feed.getFeed", s.HandleAppBskyFeedGetFeed)
	e.GET("/xrpc/app.bsky.feed.getFeedGenerator", s.HandleAppBskyFeedGetFeedGenerator)
	e.GET("/xrpc/app.bsky.feed.getFeedGenerators", s.HandleAppBskyFeedGetFeedGenerators)
	e.GET("/xrpc/app.bsky.feed.getFeedSkeleton", s.HandleAppBskyFeedGetFeedSkeleton)
	e.GET("/xrpc/app.bsky.feed.getLikes", s.HandleAppBskyFeedGetLikes)
	e.GET("/xrpc/app.bsky.feed.getListFeed", s.HandleAppBskyFeedGetListFeed)
	e.GET("/xrpc/app.bsky.feed.getPostThread", s.HandleAppBskyFeedGetPostThread)
	e.GET("/xrpc/app.bsky.feed.getPosts", s.HandleAppBskyFeedGetPosts)
	e.GET("/xrpc/app.bsky.feed.getQuotes", s.HandleAppBskyFeedGetQuotes)
	e.GET("/xrpc/app.bsky.feed.getRepostedBy", s.HandleAppBskyFeedGetRepostedBy)
	e.GET("/xrpc/app.bsky.feed.getSuggestedFeeds", s.HandleAppBskyFeedGetSuggestedFeeds)
	e.GET("/xrpc/app.bsky.feed.getTimeline", s.HandleAppBskyFeedGetTimeline)
	e.GET("/xrpc/app.bsky.feed.searchPosts", s.HandleAppBskyFeedSearchPosts)
	e.POST("/xrpc/app.bsky.feed.sendInteractions", s.HandleAppBskyFeedSendInteractions)
	e.GET("/xrpc/app.bsky.graph.getActorStarterPacks", s.HandleAppBskyGraphGetActorStarterPacks)
	e.GET("/xrpc/app.bsky.graph.getBlocks", s.HandleAppBskyGraphGetBlocks)
	e.GET("/xrpc/app.bsky.graph.getFollowers", s.HandleAppBskyGraphGetFollowers)
	e.GET("/xrpc/app.bsky.graph.getFollows", s.HandleAppBskyGraphGetFollows)
	e.GET("/xrpc/app.bsky.graph.getKnownFollowers", s.HandleAppBskyGraphGetKnownFollowers)
	e.GET("/xrpc/app.bsky.graph.getList", s.HandleAppBskyGraphGetList)
	e.GET("/xrpc/app.bsky.graph.getListBlocks", s.HandleAppBskyGraphGetListBlocks)
	e.GET("/xrpc/app.bsky.graph.getListMutes", s.HandleAppBskyGraphGetListMutes)
	e.GET("/xrpc/app.bsky.graph.getLists", s.HandleAppBskyGraphGetLists)
	e.GET("/xrpc/app.bsky.graph.getListsWithMembership", s.HandleAppBskyGraphGetListsWithMembership)
	e.GET("/xrpc/app.bsky.graph.getMutes", s.HandleAppBskyGraphGetMutes)
	e.GET("/xrpc/app.bsky.graph.getRelationships", s.HandleAppBskyGraphGetRelationships)
	e.GET("/xrpc/app.bsky.graph.getStarterPack", s.HandleAppBskyGraphGetStarterPack)
	e.GET("/xrpc/app.bsky.graph.getStarterPacks", s.HandleAppBskyGraphGetStarterPacks)
	e.GET("/xrpc/app.bsky.graph.getStarterPacksWithMembership", s.HandleAppBskyGraphGetStarterPacksWithMembership)
	e.GET("/xrpc/app.bsky.graph.getSuggestedFollowsByActor", s.HandleAppBskyGraphGetSuggestedFollowsByActor)
	e.POST("/xrpc/app.bsky.graph.muteActor", s.HandleAppBskyGraphMuteActor)
	e.POST("/xrpc/app.bsky.graph.muteActorList", s.HandleAppBskyGraphMuteActorList)
	e.POST("/xrpc/app.bsky.graph.muteThread", s.HandleAppBskyGraphMuteThread)
	e.GET("/xrpc/app.bsky.graph.searchStarterPacks", s.HandleAppBskyGraphSearchStarterPacks)
	e.POST("/xrpc/app.bsky.graph.unmuteActor", s.HandleAppBskyGraphUnmuteActor)
	e.POST("/xrpc/app.bsky.graph.unmuteActorList", s.HandleAppBskyGraphUnmuteActorList)
	e.POST("/xrpc/app.bsky.graph.unmuteThread", s.HandleAppBskyGraphUnmuteThread)
	e.GET("/xrpc/app.bsky.labeler.getServices", s.HandleAppBskyLabelerGetServices)
	e.GET("/xrpc/app.bsky.notification.getPreferences", s.HandleAppBskyNotificationGetPreferences)
	e.GET("/xrpc/app.bsky.notification.getUnreadCount", s.HandleAppBskyNotificationGetUnreadCount)
	e.GET("/xrpc/app.bsky.notification.listActivitySubscriptions", s.HandleAppBskyNotificationListActivitySubscriptions)
	e.GET("/xrpc/app.bsky.notification.listNotifications", s.HandleAppBskyNotificationListNotifications)
	e.POST("/xrpc/app.bsky.notification.putActivitySubscription", s.HandleAppBskyNotificationPutActivitySubscription)
	e.POST("/xrpc/app.bsky.notification.putPreferences", s.HandleAppBskyNotificationPutPreferences)
	e.POST("/xrpc/app.bsky.notification.putPreferencesV2", s.HandleAppBskyNotificationPutPreferencesV2)
	e.POST("/xrpc/app.bsky.notification.registerPush", s.HandleAppBskyNotificationRegisterPush)
	e.POST("/xrpc/app.bsky.notification.unregisterPush", s.HandleAppBskyNotificationUnregisterPush)
	e.POST("/xrpc/app.bsky.notification.updateSeen", s.HandleAppBskyNotificationUpdateSeen)
	e.GET("/xrpc/app.bsky.unspecced.getAgeAssuranceState", s.HandleAppBskyUnspeccedGetAgeAssuranceState)
	e.GET("/xrpc/app.bsky.unspecced.getConfig", s.HandleAppBskyUnspeccedGetConfig)
	e.GET("/xrpc/app.bsky.unspecced.getOnboardingSuggestedStarterPacks", s.HandleAppBskyUnspeccedGetOnboardingSuggestedStarterPacks)
	e.GET("/xrpc/app.bsky.unspecced.getOnboardingSuggestedStarterPacksSkeleton", s.HandleAppBskyUnspeccedGetOnboardingSuggestedStarterPacksSkeleton)
	e.GET("/xrpc/app.bsky.unspecced.getPopularFeedGenerators", s.HandleAppBskyUnspeccedGetPopularFeedGenerators)
	e.GET("/xrpc/app.bsky.unspecced.getPostThreadOtherV2", s.HandleAppBskyUnspeccedGetPostThreadOtherV2)
	e.GET("/xrpc/app.bsky.unspecced.getPostThreadV2", s.HandleAppBskyUnspeccedGetPostThreadV2)
	e.GET("/xrpc/app.bsky.unspecced.getSuggestedFeeds", s.HandleAppBskyUnspeccedGetSuggestedFeeds)
	e.GET("/xrpc/app.bsky.unspecced.getSuggestedFeedsSkeleton", s.HandleAppBskyUnspeccedGetSuggestedFeedsSkeleton)
	e.GET("/xrpc/app.bsky.unspecced.getSuggestedStarterPacks", s.HandleAppBskyUnspeccedGetSuggestedStarterPacks)
	e.GET("/xrpc/app.bsky.unspecced.getSuggestedStarterPacksSkeleton", s.HandleAppBskyUnspeccedGetSuggestedStarterPacksSkeleton)
	e.GET("/xrpc/app.bsky.unspecced.getSuggestedUsers", s.HandleAppBskyUnspeccedGetSuggestedUsers)
	e.GET("/xrpc/app.bsky.unspecced.getSuggestedUsersSkeleton", s.HandleAppBskyUnspeccedGetSuggestedUsersSkeleton)
	e.GET("/xrpc/app.bsky.unspecced.getSuggestionsSkeleton", s.HandleAppBskyUnspeccedGetSuggestionsSkeleton)
	e.GET("/xrpc/app.bsky.unspecced.getTaggedSuggestions", s.HandleAppBskyUnspeccedGetTaggedSuggestions)
	e.GET("/xrpc/app.bsky.unspecced.getTrendingTopics", s.HandleAppBskyUnspeccedGetTrendingTopics)
	e.GET("/xrpc/app.bsky.unspecced.getTrends", s.HandleAppBskyUnspeccedGetTrends)
	e.GET("/xrpc/app.bsky.unspecced.getTrendsSkeleton", s.HandleAppBskyUnspeccedGetTrendsSkeleton)
	e.POST("/xrpc/app.bsky.unspecced.initAgeAssurance", s.HandleAppBskyUnspeccedInitAgeAssurance)
	e.GET("/xrpc/app.bsky.unspecced.searchActorsSkeleton", s.HandleAppBskyUnspeccedSearchActorsSkeleton)
	e.GET("/xrpc/app.bsky.unspecced.searchPostsSkeleton", s.HandleAppBskyUnspeccedSearchPostsSkeleton)
	e.GET("/xrpc/app.bsky.unspecced.searchStarterPacksSkeleton", s.HandleAppBskyUnspeccedSearchStarterPacksSkeleton)
	e.GET("/xrpc/app.bsky.video.getJobStatus", s.HandleAppBskyVideoGetJobStatus)
	e.GET("/xrpc/app.bsky.video.getUploadLimits", s.HandleAppBskyVideoGetUploadLimits)
	e.POST("/xrpc/app.bsky.video.uploadVideo", s.HandleAppBskyVideoUploadVideo)
	return nil
}

func (s *Server) HandleAppBskyActorGetPreferences(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyActorGetPreferences")
	defer span.End()
	var out *appbsky.ActorGetPreferences_Output
	var handleErr error
	// func (s *Server) handleAppBskyActorGetPreferences(ctx context.Context) (*appbsky.ActorGetPreferences_Output, error)
	out, handleErr = s.handleAppBskyActorGetPreferences(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
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

func (s *Server) HandleAppBskyActorGetProfiles(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyActorGetProfiles")
	defer span.End()
	actors := c.QueryParams()["actors"]
	var out *appbsky.ActorGetProfiles_Output
	var handleErr error
	// func (s *Server) handleAppBskyActorGetProfiles(ctx context.Context,actors []string) (*appbsky.ActorGetProfiles_Output, error)
	out, handleErr = s.handleAppBskyActorGetProfiles(ctx, actors)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyActorGetSuggestions(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyActorGetSuggestions")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.ActorGetSuggestions_Output
	var handleErr error
	// func (s *Server) handleAppBskyActorGetSuggestions(ctx context.Context,cursor string,limit *int) (*appbsky.ActorGetSuggestions_Output, error)
	out, handleErr = s.handleAppBskyActorGetSuggestions(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyActorPutPreferences(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyActorPutPreferences")
	defer span.End()
	var body appbsky.ActorPutPreferences_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyActorPutPreferences(ctx context.Context,body *appbsky.ActorPutPreferences_Input) error
	handleErr = s.handleAppBskyActorPutPreferences(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyActorSearchActors(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyActorSearchActors")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	q := c.QueryParam("q")
	term := c.QueryParam("term")
	var out *appbsky.ActorSearchActors_Output
	var handleErr error
	// func (s *Server) handleAppBskyActorSearchActors(ctx context.Context,cursor string,limit *int,q string,term string) (*appbsky.ActorSearchActors_Output, error)
	out, handleErr = s.handleAppBskyActorSearchActors(ctx, cursor, limit, q, term)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyActorSearchActorsTypeahead(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyActorSearchActorsTypeahead")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	q := c.QueryParam("q")
	term := c.QueryParam("term")
	var out *appbsky.ActorSearchActorsTypeahead_Output
	var handleErr error
	// func (s *Server) handleAppBskyActorSearchActorsTypeahead(ctx context.Context,limit *int,q string,term string) (*appbsky.ActorSearchActorsTypeahead_Output, error)
	out, handleErr = s.handleAppBskyActorSearchActorsTypeahead(ctx, limit, q, term)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyAgeassuranceBegin(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyAgeassuranceBegin")
	defer span.End()
	var body appbsky.AgeassuranceBegin_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *appbsky.AgeassuranceDefs_State
	var handleErr error
	// func (s *Server) handleAppBskyAgeassuranceBegin(ctx context.Context,body *appbsky.AgeassuranceBegin_Input) (*appbsky.AgeassuranceDefs_State, error)
	out, handleErr = s.handleAppBskyAgeassuranceBegin(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyAgeassuranceGetConfig(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyAgeassuranceGetConfig")
	defer span.End()
	var out *appbsky.AgeassuranceDefs_Config
	var handleErr error
	// func (s *Server) handleAppBskyAgeassuranceGetConfig(ctx context.Context) (*appbsky.AgeassuranceDefs_Config, error)
	out, handleErr = s.handleAppBskyAgeassuranceGetConfig(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyAgeassuranceGetState(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyAgeassuranceGetState")
	defer span.End()
	countryCode := c.QueryParam("countryCode")
	regionCode := c.QueryParam("regionCode")
	var out *appbsky.AgeassuranceGetState_Output
	var handleErr error
	// func (s *Server) handleAppBskyAgeassuranceGetState(ctx context.Context,countryCode string,regionCode string) (*appbsky.AgeassuranceGetState_Output, error)
	out, handleErr = s.handleAppBskyAgeassuranceGetState(ctx, countryCode, regionCode)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyBookmarkCreateBookmark(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyBookmarkCreateBookmark")
	defer span.End()
	var body appbsky.BookmarkCreateBookmark_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyBookmarkCreateBookmark(ctx context.Context,body *appbsky.BookmarkCreateBookmark_Input) error
	handleErr = s.handleAppBskyBookmarkCreateBookmark(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyBookmarkDeleteBookmark(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyBookmarkDeleteBookmark")
	defer span.End()
	var body appbsky.BookmarkDeleteBookmark_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyBookmarkDeleteBookmark(ctx context.Context,body *appbsky.BookmarkDeleteBookmark_Input) error
	handleErr = s.handleAppBskyBookmarkDeleteBookmark(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyBookmarkGetBookmarks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyBookmarkGetBookmarks")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.BookmarkGetBookmarks_Output
	var handleErr error
	// func (s *Server) handleAppBskyBookmarkGetBookmarks(ctx context.Context,cursor string,limit *int) (*appbsky.BookmarkGetBookmarks_Output, error)
	out, handleErr = s.handleAppBskyBookmarkGetBookmarks(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedDescribeFeedGenerator(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedDescribeFeedGenerator")
	defer span.End()
	var out *appbsky.FeedDescribeFeedGenerator_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedDescribeFeedGenerator(ctx context.Context) (*appbsky.FeedDescribeFeedGenerator_Output, error)
	out, handleErr = s.handleAppBskyFeedDescribeFeedGenerator(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetActorFeeds(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetActorFeeds")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.FeedGetActorFeeds_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetActorFeeds(ctx context.Context,actor string,cursor string,limit *int) (*appbsky.FeedGetActorFeeds_Output, error)
	out, handleErr = s.handleAppBskyFeedGetActorFeeds(ctx, actor, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetActorLikes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetActorLikes")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.FeedGetActorLikes_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetActorLikes(ctx context.Context,actor string,cursor string,limit *int) (*appbsky.FeedGetActorLikes_Output, error)
	out, handleErr = s.handleAppBskyFeedGetActorLikes(ctx, actor, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetAuthorFeed(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetAuthorFeed")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	filter := c.QueryParam("filter")
	var includePins *bool
	if p := c.QueryParam("includePins"); p != "" {
		includePins_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		includePins = &includePins_val
	}
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.FeedGetAuthorFeed_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetAuthorFeed(ctx context.Context,actor string,cursor string,filter string,includePins *bool,limit *int) (*appbsky.FeedGetAuthorFeed_Output, error)
	out, handleErr = s.handleAppBskyFeedGetAuthorFeed(ctx, actor, cursor, filter, includePins, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetFeed(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetFeed")
	defer span.End()
	cursor := c.QueryParam("cursor")
	feed := c.QueryParam("feed")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.FeedGetFeed_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetFeed(ctx context.Context,cursor string,feed string,limit *int) (*appbsky.FeedGetFeed_Output, error)
	out, handleErr = s.handleAppBskyFeedGetFeed(ctx, cursor, feed, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetFeedGenerator(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetFeedGenerator")
	defer span.End()
	feed := c.QueryParam("feed")
	var out *appbsky.FeedGetFeedGenerator_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetFeedGenerator(ctx context.Context,feed string) (*appbsky.FeedGetFeedGenerator_Output, error)
	out, handleErr = s.handleAppBskyFeedGetFeedGenerator(ctx, feed)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetFeedGenerators(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetFeedGenerators")
	defer span.End()
	feeds := c.QueryParams()["feeds"]
	var out *appbsky.FeedGetFeedGenerators_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetFeedGenerators(ctx context.Context,feeds []string) (*appbsky.FeedGetFeedGenerators_Output, error)
	out, handleErr = s.handleAppBskyFeedGetFeedGenerators(ctx, feeds)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.FeedGetFeedSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetFeedSkeleton(ctx context.Context,cursor string,feed string,limit *int) (*appbsky.FeedGetFeedSkeleton_Output, error)
	out, handleErr = s.handleAppBskyFeedGetFeedSkeleton(ctx, cursor, feed, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetLikes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetLikes")
	defer span.End()
	cid := c.QueryParam("cid")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	uri := c.QueryParam("uri")
	var out *appbsky.FeedGetLikes_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetLikes(ctx context.Context,cid string,cursor string,limit *int,uri string) (*appbsky.FeedGetLikes_Output, error)
	out, handleErr = s.handleAppBskyFeedGetLikes(ctx, cid, cursor, limit, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetListFeed(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetListFeed")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	list := c.QueryParam("list")
	var out *appbsky.FeedGetListFeed_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetListFeed(ctx context.Context,cursor string,limit *int,list string) (*appbsky.FeedGetListFeed_Output, error)
	out, handleErr = s.handleAppBskyFeedGetListFeed(ctx, cursor, limit, list)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetPostThread(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetPostThread")
	defer span.End()
	var depth *int
	if p := c.QueryParam("depth"); p != "" {
		depth_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		depth = &depth_val
	}
	var parentHeight *int
	if p := c.QueryParam("parentHeight"); p != "" {
		parentHeight_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		parentHeight = &parentHeight_val
	}
	uri := c.QueryParam("uri")
	var out *appbsky.FeedGetPostThread_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetPostThread(ctx context.Context,depth *int,parentHeight *int,uri string) (*appbsky.FeedGetPostThread_Output, error)
	out, handleErr = s.handleAppBskyFeedGetPostThread(ctx, depth, parentHeight, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetPosts(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetPosts")
	defer span.End()
	uris := c.QueryParams()["uris"]
	var out *appbsky.FeedGetPosts_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetPosts(ctx context.Context,uris []string) (*appbsky.FeedGetPosts_Output, error)
	out, handleErr = s.handleAppBskyFeedGetPosts(ctx, uris)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetQuotes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetQuotes")
	defer span.End()
	cid := c.QueryParam("cid")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	uri := c.QueryParam("uri")
	var out *appbsky.FeedGetQuotes_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetQuotes(ctx context.Context,cid string,cursor string,limit *int,uri string) (*appbsky.FeedGetQuotes_Output, error)
	out, handleErr = s.handleAppBskyFeedGetQuotes(ctx, cid, cursor, limit, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetRepostedBy(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetRepostedBy")
	defer span.End()
	cid := c.QueryParam("cid")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	uri := c.QueryParam("uri")
	var out *appbsky.FeedGetRepostedBy_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetRepostedBy(ctx context.Context,cid string,cursor string,limit *int,uri string) (*appbsky.FeedGetRepostedBy_Output, error)
	out, handleErr = s.handleAppBskyFeedGetRepostedBy(ctx, cid, cursor, limit, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetSuggestedFeeds(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetSuggestedFeeds")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.FeedGetSuggestedFeeds_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetSuggestedFeeds(ctx context.Context,cursor string,limit *int) (*appbsky.FeedGetSuggestedFeeds_Output, error)
	out, handleErr = s.handleAppBskyFeedGetSuggestedFeeds(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedGetTimeline(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedGetTimeline")
	defer span.End()
	algorithm := c.QueryParam("algorithm")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.FeedGetTimeline_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedGetTimeline(ctx context.Context,algorithm string,cursor string,limit *int) (*appbsky.FeedGetTimeline_Output, error)
	out, handleErr = s.handleAppBskyFeedGetTimeline(ctx, algorithm, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedSearchPosts(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedSearchPosts")
	defer span.End()
	author := c.QueryParam("author")
	cursor := c.QueryParam("cursor")
	domain := c.QueryParam("domain")
	lang := c.QueryParam("lang")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	mentions := c.QueryParam("mentions")
	q := c.QueryParam("q")
	since := c.QueryParam("since")
	sort := c.QueryParam("sort")
	tag := c.QueryParams()["tag"]
	until := c.QueryParam("until")
	url := c.QueryParam("url")
	var out *appbsky.FeedSearchPosts_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedSearchPosts(ctx context.Context,author string,cursor string,domain string,lang string,limit *int,mentions string,q string,since string,sort string,tag []string,until string,url string) (*appbsky.FeedSearchPosts_Output, error)
	out, handleErr = s.handleAppBskyFeedSearchPosts(ctx, author, cursor, domain, lang, limit, mentions, q, since, sort, tag, until, url)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyFeedSendInteractions(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyFeedSendInteractions")
	defer span.End()
	var body appbsky.FeedSendInteractions_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *appbsky.FeedSendInteractions_Output
	var handleErr error
	// func (s *Server) handleAppBskyFeedSendInteractions(ctx context.Context,body *appbsky.FeedSendInteractions_Input) (*appbsky.FeedSendInteractions_Output, error)
	out, handleErr = s.handleAppBskyFeedSendInteractions(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetActorStarterPacks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetActorStarterPacks")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetActorStarterPacks_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetActorStarterPacks(ctx context.Context,actor string,cursor string,limit *int) (*appbsky.GraphGetActorStarterPacks_Output, error)
	out, handleErr = s.handleAppBskyGraphGetActorStarterPacks(ctx, actor, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetBlocks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetBlocks")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetBlocks_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetBlocks(ctx context.Context,cursor string,limit *int) (*appbsky.GraphGetBlocks_Output, error)
	out, handleErr = s.handleAppBskyGraphGetBlocks(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetFollowers(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetFollowers")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetFollowers_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetFollowers(ctx context.Context,actor string,cursor string,limit *int) (*appbsky.GraphGetFollowers_Output, error)
	out, handleErr = s.handleAppBskyGraphGetFollowers(ctx, actor, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetFollows(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetFollows")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetFollows_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetFollows(ctx context.Context,actor string,cursor string,limit *int) (*appbsky.GraphGetFollows_Output, error)
	out, handleErr = s.handleAppBskyGraphGetFollows(ctx, actor, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetKnownFollowers(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetKnownFollowers")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetKnownFollowers_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetKnownFollowers(ctx context.Context,actor string,cursor string,limit *int) (*appbsky.GraphGetKnownFollowers_Output, error)
	out, handleErr = s.handleAppBskyGraphGetKnownFollowers(ctx, actor, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetList(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetList")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	list := c.QueryParam("list")
	var out *appbsky.GraphGetList_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetList(ctx context.Context,cursor string,limit *int,list string) (*appbsky.GraphGetList_Output, error)
	out, handleErr = s.handleAppBskyGraphGetList(ctx, cursor, limit, list)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetListBlocks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetListBlocks")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetListBlocks_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetListBlocks(ctx context.Context,cursor string,limit *int) (*appbsky.GraphGetListBlocks_Output, error)
	out, handleErr = s.handleAppBskyGraphGetListBlocks(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetListMutes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetListMutes")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetListMutes_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetListMutes(ctx context.Context,cursor string,limit *int) (*appbsky.GraphGetListMutes_Output, error)
	out, handleErr = s.handleAppBskyGraphGetListMutes(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetLists(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetLists")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	purposes := c.QueryParams()["purposes"]
	var out *appbsky.GraphGetLists_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetLists(ctx context.Context,actor string,cursor string,limit *int,purposes []string) (*appbsky.GraphGetLists_Output, error)
	out, handleErr = s.handleAppBskyGraphGetLists(ctx, actor, cursor, limit, purposes)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetListsWithMembership(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetListsWithMembership")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	purposes := c.QueryParams()["purposes"]
	var out *appbsky.GraphGetListsWithMembership_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetListsWithMembership(ctx context.Context,actor string,cursor string,limit *int,purposes []string) (*appbsky.GraphGetListsWithMembership_Output, error)
	out, handleErr = s.handleAppBskyGraphGetListsWithMembership(ctx, actor, cursor, limit, purposes)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetMutes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetMutes")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetMutes_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetMutes(ctx context.Context,cursor string,limit *int) (*appbsky.GraphGetMutes_Output, error)
	out, handleErr = s.handleAppBskyGraphGetMutes(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetRelationships(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetRelationships")
	defer span.End()
	actor := c.QueryParam("actor")
	others := c.QueryParams()["others"]
	var out *appbsky.GraphGetRelationships_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetRelationships(ctx context.Context,actor string,others []string) (*appbsky.GraphGetRelationships_Output, error)
	out, handleErr = s.handleAppBskyGraphGetRelationships(ctx, actor, others)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetStarterPack(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetStarterPack")
	defer span.End()
	starterPack := c.QueryParam("starterPack")
	var out *appbsky.GraphGetStarterPack_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetStarterPack(ctx context.Context,starterPack string) (*appbsky.GraphGetStarterPack_Output, error)
	out, handleErr = s.handleAppBskyGraphGetStarterPack(ctx, starterPack)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetStarterPacks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetStarterPacks")
	defer span.End()
	uris := c.QueryParams()["uris"]
	var out *appbsky.GraphGetStarterPacks_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetStarterPacks(ctx context.Context,uris []string) (*appbsky.GraphGetStarterPacks_Output, error)
	out, handleErr = s.handleAppBskyGraphGetStarterPacks(ctx, uris)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetStarterPacksWithMembership(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetStarterPacksWithMembership")
	defer span.End()
	actor := c.QueryParam("actor")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.GraphGetStarterPacksWithMembership_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetStarterPacksWithMembership(ctx context.Context,actor string,cursor string,limit *int) (*appbsky.GraphGetStarterPacksWithMembership_Output, error)
	out, handleErr = s.handleAppBskyGraphGetStarterPacksWithMembership(ctx, actor, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphGetSuggestedFollowsByActor(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphGetSuggestedFollowsByActor")
	defer span.End()
	actor := c.QueryParam("actor")
	var out *appbsky.GraphGetSuggestedFollowsByActor_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphGetSuggestedFollowsByActor(ctx context.Context,actor string) (*appbsky.GraphGetSuggestedFollowsByActor_Output, error)
	out, handleErr = s.handleAppBskyGraphGetSuggestedFollowsByActor(ctx, actor)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphMuteActor(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphMuteActor")
	defer span.End()
	var body appbsky.GraphMuteActor_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyGraphMuteActor(ctx context.Context,body *appbsky.GraphMuteActor_Input) error
	handleErr = s.handleAppBskyGraphMuteActor(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyGraphMuteActorList(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphMuteActorList")
	defer span.End()
	var body appbsky.GraphMuteActorList_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyGraphMuteActorList(ctx context.Context,body *appbsky.GraphMuteActorList_Input) error
	handleErr = s.handleAppBskyGraphMuteActorList(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyGraphMuteThread(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphMuteThread")
	defer span.End()
	var body appbsky.GraphMuteThread_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyGraphMuteThread(ctx context.Context,body *appbsky.GraphMuteThread_Input) error
	handleErr = s.handleAppBskyGraphMuteThread(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyGraphSearchStarterPacks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphSearchStarterPacks")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	q := c.QueryParam("q")
	var out *appbsky.GraphSearchStarterPacks_Output
	var handleErr error
	// func (s *Server) handleAppBskyGraphSearchStarterPacks(ctx context.Context,cursor string,limit *int,q string) (*appbsky.GraphSearchStarterPacks_Output, error)
	out, handleErr = s.handleAppBskyGraphSearchStarterPacks(ctx, cursor, limit, q)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyGraphUnmuteActor(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphUnmuteActor")
	defer span.End()
	var body appbsky.GraphUnmuteActor_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyGraphUnmuteActor(ctx context.Context,body *appbsky.GraphUnmuteActor_Input) error
	handleErr = s.handleAppBskyGraphUnmuteActor(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyGraphUnmuteActorList(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphUnmuteActorList")
	defer span.End()
	var body appbsky.GraphUnmuteActorList_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyGraphUnmuteActorList(ctx context.Context,body *appbsky.GraphUnmuteActorList_Input) error
	handleErr = s.handleAppBskyGraphUnmuteActorList(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyGraphUnmuteThread(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyGraphUnmuteThread")
	defer span.End()
	var body appbsky.GraphUnmuteThread_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyGraphUnmuteThread(ctx context.Context,body *appbsky.GraphUnmuteThread_Input) error
	handleErr = s.handleAppBskyGraphUnmuteThread(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyLabelerGetServices(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyLabelerGetServices")
	defer span.End()
	var detailed *bool
	if p := c.QueryParam("detailed"); p != "" {
		detailed_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		detailed = &detailed_val
	}
	dids := c.QueryParams()["dids"]
	var out *appbsky.LabelerGetServices_Output
	var handleErr error
	// func (s *Server) handleAppBskyLabelerGetServices(ctx context.Context,detailed *bool,dids []string) (*appbsky.LabelerGetServices_Output, error)
	out, handleErr = s.handleAppBskyLabelerGetServices(ctx, detailed, dids)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyNotificationGetPreferences(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationGetPreferences")
	defer span.End()
	var out *appbsky.NotificationGetPreferences_Output
	var handleErr error
	// func (s *Server) handleAppBskyNotificationGetPreferences(ctx context.Context) (*appbsky.NotificationGetPreferences_Output, error)
	out, handleErr = s.handleAppBskyNotificationGetPreferences(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyNotificationGetUnreadCount(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationGetUnreadCount")
	defer span.End()
	var priority *bool
	if p := c.QueryParam("priority"); p != "" {
		priority_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		priority = &priority_val
	}
	seenAt := c.QueryParam("seenAt")
	var out *appbsky.NotificationGetUnreadCount_Output
	var handleErr error
	// func (s *Server) handleAppBskyNotificationGetUnreadCount(ctx context.Context,priority *bool,seenAt string) (*appbsky.NotificationGetUnreadCount_Output, error)
	out, handleErr = s.handleAppBskyNotificationGetUnreadCount(ctx, priority, seenAt)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyNotificationListActivitySubscriptions(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationListActivitySubscriptions")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.NotificationListActivitySubscriptions_Output
	var handleErr error
	// func (s *Server) handleAppBskyNotificationListActivitySubscriptions(ctx context.Context,cursor string,limit *int) (*appbsky.NotificationListActivitySubscriptions_Output, error)
	out, handleErr = s.handleAppBskyNotificationListActivitySubscriptions(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyNotificationListNotifications(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationListNotifications")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var priority *bool
	if p := c.QueryParam("priority"); p != "" {
		priority_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		priority = &priority_val
	}
	reasons := c.QueryParams()["reasons"]
	seenAt := c.QueryParam("seenAt")
	var out *appbsky.NotificationListNotifications_Output
	var handleErr error
	// func (s *Server) handleAppBskyNotificationListNotifications(ctx context.Context,cursor string,limit *int,priority *bool,reasons []string,seenAt string) (*appbsky.NotificationListNotifications_Output, error)
	out, handleErr = s.handleAppBskyNotificationListNotifications(ctx, cursor, limit, priority, reasons, seenAt)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyNotificationPutActivitySubscription(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationPutActivitySubscription")
	defer span.End()
	var body appbsky.NotificationPutActivitySubscription_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *appbsky.NotificationPutActivitySubscription_Output
	var handleErr error
	// func (s *Server) handleAppBskyNotificationPutActivitySubscription(ctx context.Context,body *appbsky.NotificationPutActivitySubscription_Input) (*appbsky.NotificationPutActivitySubscription_Output, error)
	out, handleErr = s.handleAppBskyNotificationPutActivitySubscription(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyNotificationPutPreferences(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationPutPreferences")
	defer span.End()
	var body appbsky.NotificationPutPreferences_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyNotificationPutPreferences(ctx context.Context,body *appbsky.NotificationPutPreferences_Input) error
	handleErr = s.handleAppBskyNotificationPutPreferences(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyNotificationPutPreferencesV2(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationPutPreferencesV2")
	defer span.End()
	var body appbsky.NotificationPutPreferencesV2_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *appbsky.NotificationPutPreferencesV2_Output
	var handleErr error
	// func (s *Server) handleAppBskyNotificationPutPreferencesV2(ctx context.Context,body *appbsky.NotificationPutPreferencesV2_Input) (*appbsky.NotificationPutPreferencesV2_Output, error)
	out, handleErr = s.handleAppBskyNotificationPutPreferencesV2(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyNotificationRegisterPush(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationRegisterPush")
	defer span.End()
	var body appbsky.NotificationRegisterPush_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyNotificationRegisterPush(ctx context.Context,body *appbsky.NotificationRegisterPush_Input) error
	handleErr = s.handleAppBskyNotificationRegisterPush(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyNotificationUnregisterPush(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationUnregisterPush")
	defer span.End()
	var body appbsky.NotificationUnregisterPush_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyNotificationUnregisterPush(ctx context.Context,body *appbsky.NotificationUnregisterPush_Input) error
	handleErr = s.handleAppBskyNotificationUnregisterPush(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyNotificationUpdateSeen(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyNotificationUpdateSeen")
	defer span.End()
	var body appbsky.NotificationUpdateSeen_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleAppBskyNotificationUpdateSeen(ctx context.Context,body *appbsky.NotificationUpdateSeen_Input) error
	handleErr = s.handleAppBskyNotificationUpdateSeen(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleAppBskyUnspeccedGetAgeAssuranceState(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetAgeAssuranceState")
	defer span.End()
	var out *appbsky.UnspeccedDefs_AgeAssuranceState
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetAgeAssuranceState(ctx context.Context) (*appbsky.UnspeccedDefs_AgeAssuranceState, error)
	out, handleErr = s.handleAppBskyUnspeccedGetAgeAssuranceState(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetConfig(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetConfig")
	defer span.End()
	var out *appbsky.UnspeccedGetConfig_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetConfig(ctx context.Context) (*appbsky.UnspeccedGetConfig_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetConfig(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetOnboardingSuggestedStarterPacks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetOnboardingSuggestedStarterPacks")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.UnspeccedGetOnboardingSuggestedStarterPacks_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetOnboardingSuggestedStarterPacks(ctx context.Context,limit *int) (*appbsky.UnspeccedGetOnboardingSuggestedStarterPacks_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetOnboardingSuggestedStarterPacks(ctx, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetOnboardingSuggestedStarterPacksSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetOnboardingSuggestedStarterPacksSkeleton")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedGetOnboardingSuggestedStarterPacksSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetOnboardingSuggestedStarterPacksSkeleton(ctx context.Context,limit *int,viewer string) (*appbsky.UnspeccedGetOnboardingSuggestedStarterPacksSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetOnboardingSuggestedStarterPacksSkeleton(ctx, limit, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetPopularFeedGenerators(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetPopularFeedGenerators")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	query := c.QueryParam("query")
	var out *appbsky.UnspeccedGetPopularFeedGenerators_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetPopularFeedGenerators(ctx context.Context,cursor string,limit *int,query string) (*appbsky.UnspeccedGetPopularFeedGenerators_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetPopularFeedGenerators(ctx, cursor, limit, query)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetPostThreadOtherV2(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetPostThreadOtherV2")
	defer span.End()
	anchor := c.QueryParam("anchor")
	var out *appbsky.UnspeccedGetPostThreadOtherV2_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetPostThreadOtherV2(ctx context.Context,anchor string) (*appbsky.UnspeccedGetPostThreadOtherV2_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetPostThreadOtherV2(ctx, anchor)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetPostThreadV2(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetPostThreadV2")
	defer span.End()
	var above *bool
	if p := c.QueryParam("above"); p != "" {
		above_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		above = &above_val
	}
	anchor := c.QueryParam("anchor")
	var below *int
	if p := c.QueryParam("below"); p != "" {
		below_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		below = &below_val
	}
	var branchingFactor *int
	if p := c.QueryParam("branchingFactor"); p != "" {
		branchingFactor_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		branchingFactor = &branchingFactor_val
	}
	sort := c.QueryParam("sort")
	var out *appbsky.UnspeccedGetPostThreadV2_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetPostThreadV2(ctx context.Context,above *bool,anchor string,below *int,branchingFactor *int,sort string) (*appbsky.UnspeccedGetPostThreadV2_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetPostThreadV2(ctx, above, anchor, below, branchingFactor, sort)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetSuggestedFeeds(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetSuggestedFeeds")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.UnspeccedGetSuggestedFeeds_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetSuggestedFeeds(ctx context.Context,limit *int) (*appbsky.UnspeccedGetSuggestedFeeds_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetSuggestedFeeds(ctx, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetSuggestedFeedsSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetSuggestedFeedsSkeleton")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedGetSuggestedFeedsSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetSuggestedFeedsSkeleton(ctx context.Context,limit *int,viewer string) (*appbsky.UnspeccedGetSuggestedFeedsSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetSuggestedFeedsSkeleton(ctx, limit, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetSuggestedStarterPacks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetSuggestedStarterPacks")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.UnspeccedGetSuggestedStarterPacks_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetSuggestedStarterPacks(ctx context.Context,limit *int) (*appbsky.UnspeccedGetSuggestedStarterPacks_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetSuggestedStarterPacks(ctx, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetSuggestedStarterPacksSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetSuggestedStarterPacksSkeleton")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedGetSuggestedStarterPacksSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetSuggestedStarterPacksSkeleton(ctx context.Context,limit *int,viewer string) (*appbsky.UnspeccedGetSuggestedStarterPacksSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetSuggestedStarterPacksSkeleton(ctx, limit, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetSuggestedUsers(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetSuggestedUsers")
	defer span.End()
	category := c.QueryParam("category")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.UnspeccedGetSuggestedUsers_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetSuggestedUsers(ctx context.Context,category string,limit *int) (*appbsky.UnspeccedGetSuggestedUsers_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetSuggestedUsers(ctx, category, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetSuggestedUsersSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetSuggestedUsersSkeleton")
	defer span.End()
	category := c.QueryParam("category")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedGetSuggestedUsersSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetSuggestedUsersSkeleton(ctx context.Context,category string,limit *int,viewer string) (*appbsky.UnspeccedGetSuggestedUsersSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetSuggestedUsersSkeleton(ctx, category, limit, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetSuggestionsSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetSuggestionsSkeleton")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	relativeToDid := c.QueryParam("relativeToDid")
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedGetSuggestionsSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetSuggestionsSkeleton(ctx context.Context,cursor string,limit *int,relativeToDid string,viewer string) (*appbsky.UnspeccedGetSuggestionsSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetSuggestionsSkeleton(ctx, cursor, limit, relativeToDid, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetTaggedSuggestions(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetTaggedSuggestions")
	defer span.End()
	var out *appbsky.UnspeccedGetTaggedSuggestions_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetTaggedSuggestions(ctx context.Context) (*appbsky.UnspeccedGetTaggedSuggestions_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetTaggedSuggestions(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetTrendingTopics(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetTrendingTopics")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedGetTrendingTopics_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetTrendingTopics(ctx context.Context,limit *int,viewer string) (*appbsky.UnspeccedGetTrendingTopics_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetTrendingTopics(ctx, limit, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetTrends(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetTrends")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *appbsky.UnspeccedGetTrends_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetTrends(ctx context.Context,limit *int) (*appbsky.UnspeccedGetTrends_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetTrends(ctx, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedGetTrendsSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedGetTrendsSkeleton")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedGetTrendsSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedGetTrendsSkeleton(ctx context.Context,limit *int,viewer string) (*appbsky.UnspeccedGetTrendsSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedGetTrendsSkeleton(ctx, limit, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedInitAgeAssurance(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedInitAgeAssurance")
	defer span.End()
	var body appbsky.UnspeccedInitAgeAssurance_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *appbsky.UnspeccedDefs_AgeAssuranceState
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedInitAgeAssurance(ctx context.Context,body *appbsky.UnspeccedInitAgeAssurance_Input) (*appbsky.UnspeccedDefs_AgeAssuranceState, error)
	out, handleErr = s.handleAppBskyUnspeccedInitAgeAssurance(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedSearchActorsSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedSearchActorsSkeleton")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	q := c.QueryParam("q")
	var typeahead *bool
	if p := c.QueryParam("typeahead"); p != "" {
		typeahead_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		typeahead = &typeahead_val
	}
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedSearchActorsSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedSearchActorsSkeleton(ctx context.Context,cursor string,limit *int,q string,typeahead *bool,viewer string) (*appbsky.UnspeccedSearchActorsSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedSearchActorsSkeleton(ctx, cursor, limit, q, typeahead, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedSearchPostsSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedSearchPostsSkeleton")
	defer span.End()
	author := c.QueryParam("author")
	cursor := c.QueryParam("cursor")
	domain := c.QueryParam("domain")
	lang := c.QueryParam("lang")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	mentions := c.QueryParam("mentions")
	q := c.QueryParam("q")
	since := c.QueryParam("since")
	sort := c.QueryParam("sort")
	tag := c.QueryParams()["tag"]
	until := c.QueryParam("until")
	url := c.QueryParam("url")
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedSearchPostsSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedSearchPostsSkeleton(ctx context.Context,author string,cursor string,domain string,lang string,limit *int,mentions string,q string,since string,sort string,tag []string,until string,url string,viewer string) (*appbsky.UnspeccedSearchPostsSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedSearchPostsSkeleton(ctx, author, cursor, domain, lang, limit, mentions, q, since, sort, tag, until, url, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyUnspeccedSearchStarterPacksSkeleton(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyUnspeccedSearchStarterPacksSkeleton")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	q := c.QueryParam("q")
	viewer := c.QueryParam("viewer")
	var out *appbsky.UnspeccedSearchStarterPacksSkeleton_Output
	var handleErr error
	// func (s *Server) handleAppBskyUnspeccedSearchStarterPacksSkeleton(ctx context.Context,cursor string,limit *int,q string,viewer string) (*appbsky.UnspeccedSearchStarterPacksSkeleton_Output, error)
	out, handleErr = s.handleAppBskyUnspeccedSearchStarterPacksSkeleton(ctx, cursor, limit, q, viewer)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyVideoGetJobStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyVideoGetJobStatus")
	defer span.End()
	jobId := c.QueryParam("jobId")
	var out *appbsky.VideoGetJobStatus_Output
	var handleErr error
	// func (s *Server) handleAppBskyVideoGetJobStatus(ctx context.Context,jobId string) (*appbsky.VideoGetJobStatus_Output, error)
	out, handleErr = s.handleAppBskyVideoGetJobStatus(ctx, jobId)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyVideoGetUploadLimits(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyVideoGetUploadLimits")
	defer span.End()
	var out *appbsky.VideoGetUploadLimits_Output
	var handleErr error
	// func (s *Server) handleAppBskyVideoGetUploadLimits(ctx context.Context) (*appbsky.VideoGetUploadLimits_Output, error)
	out, handleErr = s.handleAppBskyVideoGetUploadLimits(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleAppBskyVideoUploadVideo(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleAppBskyVideoUploadVideo")
	defer span.End()
	body := c.Request().Body
	var out *appbsky.VideoUploadVideo_Output
	var handleErr error
	// func (s *Server) handleAppBskyVideoUploadVideo(ctx context.Context,r io.Reader) (*appbsky.VideoUploadVideo_Output, error)
	out, handleErr = s.handleAppBskyVideoUploadVideo(ctx, body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersComatproto(e *echo.Echo) error {
	e.POST("/xrpc/com.atproto.admin.deleteAccount", s.HandleComAtprotoAdminDeleteAccount)
	e.POST("/xrpc/com.atproto.admin.disableAccountInvites", s.HandleComAtprotoAdminDisableAccountInvites)
	e.POST("/xrpc/com.atproto.admin.disableInviteCodes", s.HandleComAtprotoAdminDisableInviteCodes)
	e.POST("/xrpc/com.atproto.admin.enableAccountInvites", s.HandleComAtprotoAdminEnableAccountInvites)
	e.GET("/xrpc/com.atproto.admin.getAccountInfo", s.HandleComAtprotoAdminGetAccountInfo)
	e.GET("/xrpc/com.atproto.admin.getAccountInfos", s.HandleComAtprotoAdminGetAccountInfos)
	e.GET("/xrpc/com.atproto.admin.getInviteCodes", s.HandleComAtprotoAdminGetInviteCodes)
	e.GET("/xrpc/com.atproto.admin.getSubjectStatus", s.HandleComAtprotoAdminGetSubjectStatus)
	e.GET("/xrpc/com.atproto.admin.searchAccounts", s.HandleComAtprotoAdminSearchAccounts)
	e.POST("/xrpc/com.atproto.admin.sendEmail", s.HandleComAtprotoAdminSendEmail)
	e.POST("/xrpc/com.atproto.admin.updateAccountEmail", s.HandleComAtprotoAdminUpdateAccountEmail)
	e.POST("/xrpc/com.atproto.admin.updateAccountHandle", s.HandleComAtprotoAdminUpdateAccountHandle)
	e.POST("/xrpc/com.atproto.admin.updateAccountPassword", s.HandleComAtprotoAdminUpdateAccountPassword)
	e.POST("/xrpc/com.atproto.admin.updateAccountSigningKey", s.HandleComAtprotoAdminUpdateAccountSigningKey)
	e.POST("/xrpc/com.atproto.admin.updateSubjectStatus", s.HandleComAtprotoAdminUpdateSubjectStatus)
	e.GET("/xrpc/com.atproto.identity.getRecommendedDidCredentials", s.HandleComAtprotoIdentityGetRecommendedDidCredentials)
	e.POST("/xrpc/com.atproto.identity.refreshIdentity", s.HandleComAtprotoIdentityRefreshIdentity)
	e.POST("/xrpc/com.atproto.identity.requestPlcOperationSignature", s.HandleComAtprotoIdentityRequestPlcOperationSignature)
	e.GET("/xrpc/com.atproto.identity.resolveDid", s.HandleComAtprotoIdentityResolveDid)
	e.GET("/xrpc/com.atproto.identity.resolveHandle", s.HandleComAtprotoIdentityResolveHandle)
	e.GET("/xrpc/com.atproto.identity.resolveIdentity", s.HandleComAtprotoIdentityResolveIdentity)
	e.POST("/xrpc/com.atproto.identity.signPlcOperation", s.HandleComAtprotoIdentitySignPlcOperation)
	e.POST("/xrpc/com.atproto.identity.submitPlcOperation", s.HandleComAtprotoIdentitySubmitPlcOperation)
	e.POST("/xrpc/com.atproto.identity.updateHandle", s.HandleComAtprotoIdentityUpdateHandle)
	e.GET("/xrpc/com.atproto.label.queryLabels", s.HandleComAtprotoLabelQueryLabels)
	e.POST("/xrpc/com.atproto.moderation.createReport", s.HandleComAtprotoModerationCreateReport)
	e.POST("/xrpc/com.atproto.repo.applyWrites", s.HandleComAtprotoRepoApplyWrites)
	e.POST("/xrpc/com.atproto.repo.createRecord", s.HandleComAtprotoRepoCreateRecord)
	e.POST("/xrpc/com.atproto.repo.deleteRecord", s.HandleComAtprotoRepoDeleteRecord)
	e.GET("/xrpc/com.atproto.repo.describeRepo", s.HandleComAtprotoRepoDescribeRepo)
	e.GET("/xrpc/com.atproto.repo.getRecord", s.HandleComAtprotoRepoGetRecord)
	e.POST("/xrpc/com.atproto.repo.importRepo", s.HandleComAtprotoRepoImportRepo)
	e.GET("/xrpc/com.atproto.repo.listMissingBlobs", s.HandleComAtprotoRepoListMissingBlobs)
	e.GET("/xrpc/com.atproto.repo.listRecords", s.HandleComAtprotoRepoListRecords)
	e.POST("/xrpc/com.atproto.repo.putRecord", s.HandleComAtprotoRepoPutRecord)
	e.POST("/xrpc/com.atproto.repo.uploadBlob", s.HandleComAtprotoRepoUploadBlob)
	e.POST("/xrpc/com.atproto.server.activateAccount", s.HandleComAtprotoServerActivateAccount)
	e.GET("/xrpc/com.atproto.server.checkAccountStatus", s.HandleComAtprotoServerCheckAccountStatus)
	e.POST("/xrpc/com.atproto.server.confirmEmail", s.HandleComAtprotoServerConfirmEmail)
	e.POST("/xrpc/com.atproto.server.createAccount", s.HandleComAtprotoServerCreateAccount)
	e.POST("/xrpc/com.atproto.server.createAppPassword", s.HandleComAtprotoServerCreateAppPassword)
	e.POST("/xrpc/com.atproto.server.createInviteCode", s.HandleComAtprotoServerCreateInviteCode)
	e.POST("/xrpc/com.atproto.server.createInviteCodes", s.HandleComAtprotoServerCreateInviteCodes)
	e.POST("/xrpc/com.atproto.server.createSession", s.HandleComAtprotoServerCreateSession)
	e.POST("/xrpc/com.atproto.server.deactivateAccount", s.HandleComAtprotoServerDeactivateAccount)
	e.POST("/xrpc/com.atproto.server.deleteAccount", s.HandleComAtprotoServerDeleteAccount)
	e.POST("/xrpc/com.atproto.server.deleteSession", s.HandleComAtprotoServerDeleteSession)
	e.GET("/xrpc/com.atproto.server.describeServer", s.HandleComAtprotoServerDescribeServer)
	e.GET("/xrpc/com.atproto.server.getAccountInviteCodes", s.HandleComAtprotoServerGetAccountInviteCodes)
	e.GET("/xrpc/com.atproto.server.getServiceAuth", s.HandleComAtprotoServerGetServiceAuth)
	e.GET("/xrpc/com.atproto.server.getSession", s.HandleComAtprotoServerGetSession)
	e.GET("/xrpc/com.atproto.server.listAppPasswords", s.HandleComAtprotoServerListAppPasswords)
	e.POST("/xrpc/com.atproto.server.refreshSession", s.HandleComAtprotoServerRefreshSession)
	e.POST("/xrpc/com.atproto.server.requestAccountDelete", s.HandleComAtprotoServerRequestAccountDelete)
	e.POST("/xrpc/com.atproto.server.requestEmailConfirmation", s.HandleComAtprotoServerRequestEmailConfirmation)
	e.POST("/xrpc/com.atproto.server.requestEmailUpdate", s.HandleComAtprotoServerRequestEmailUpdate)
	e.POST("/xrpc/com.atproto.server.requestPasswordReset", s.HandleComAtprotoServerRequestPasswordReset)
	e.POST("/xrpc/com.atproto.server.reserveSigningKey", s.HandleComAtprotoServerReserveSigningKey)
	e.POST("/xrpc/com.atproto.server.resetPassword", s.HandleComAtprotoServerResetPassword)
	e.POST("/xrpc/com.atproto.server.revokeAppPassword", s.HandleComAtprotoServerRevokeAppPassword)
	e.POST("/xrpc/com.atproto.server.updateEmail", s.HandleComAtprotoServerUpdateEmail)
	e.GET("/xrpc/com.atproto.sync.getBlob", s.HandleComAtprotoSyncGetBlob)
	e.GET("/xrpc/com.atproto.sync.getBlocks", s.HandleComAtprotoSyncGetBlocks)
	e.GET("/xrpc/com.atproto.sync.getCheckout", s.HandleComAtprotoSyncGetCheckout)
	e.GET("/xrpc/com.atproto.sync.getHead", s.HandleComAtprotoSyncGetHead)
	e.GET("/xrpc/com.atproto.sync.getHostStatus", s.HandleComAtprotoSyncGetHostStatus)
	e.GET("/xrpc/com.atproto.sync.getLatestCommit", s.HandleComAtprotoSyncGetLatestCommit)
	e.GET("/xrpc/com.atproto.sync.getRecord", s.HandleComAtprotoSyncGetRecord)
	e.GET("/xrpc/com.atproto.sync.getRepo", s.HandleComAtprotoSyncGetRepo)
	e.GET("/xrpc/com.atproto.sync.getRepoStatus", s.HandleComAtprotoSyncGetRepoStatus)
	e.GET("/xrpc/com.atproto.sync.listBlobs", s.HandleComAtprotoSyncListBlobs)
	e.GET("/xrpc/com.atproto.sync.listHosts", s.HandleComAtprotoSyncListHosts)
	e.GET("/xrpc/com.atproto.sync.listRepos", s.HandleComAtprotoSyncListRepos)
	e.GET("/xrpc/com.atproto.sync.listReposByCollection", s.HandleComAtprotoSyncListReposByCollection)
	e.POST("/xrpc/com.atproto.sync.notifyOfUpdate", s.HandleComAtprotoSyncNotifyOfUpdate)
	e.POST("/xrpc/com.atproto.sync.requestCrawl", s.HandleComAtprotoSyncRequestCrawl)
	e.POST("/xrpc/com.atproto.temp.addReservedHandle", s.HandleComAtprotoTempAddReservedHandle)
	e.GET("/xrpc/com.atproto.temp.checkHandleAvailability", s.HandleComAtprotoTempCheckHandleAvailability)
	e.GET("/xrpc/com.atproto.temp.checkSignupQueue", s.HandleComAtprotoTempCheckSignupQueue)
	e.GET("/xrpc/com.atproto.temp.dereferenceScope", s.HandleComAtprotoTempDereferenceScope)
	e.GET("/xrpc/com.atproto.temp.fetchLabels", s.HandleComAtprotoTempFetchLabels)
	e.POST("/xrpc/com.atproto.temp.requestPhoneVerification", s.HandleComAtprotoTempRequestPhoneVerification)
	e.POST("/xrpc/com.atproto.temp.revokeAccountCredentials", s.HandleComAtprotoTempRevokeAccountCredentials)
	return nil
}

func (s *Server) HandleComAtprotoAdminDeleteAccount(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminDeleteAccount")
	defer span.End()
	var body comatproto.AdminDeleteAccount_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminDeleteAccount(ctx context.Context,body *comatproto.AdminDeleteAccount_Input) error
	handleErr = s.handleComAtprotoAdminDeleteAccount(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminDisableAccountInvites(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminDisableAccountInvites")
	defer span.End()
	var body comatproto.AdminDisableAccountInvites_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminDisableAccountInvites(ctx context.Context,body *comatproto.AdminDisableAccountInvites_Input) error
	handleErr = s.handleComAtprotoAdminDisableAccountInvites(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminDisableInviteCodes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminDisableInviteCodes")
	defer span.End()
	var body comatproto.AdminDisableInviteCodes_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminDisableInviteCodes(ctx context.Context,body *comatproto.AdminDisableInviteCodes_Input) error
	handleErr = s.handleComAtprotoAdminDisableInviteCodes(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminEnableAccountInvites(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminEnableAccountInvites")
	defer span.End()
	var body comatproto.AdminEnableAccountInvites_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminEnableAccountInvites(ctx context.Context,body *comatproto.AdminEnableAccountInvites_Input) error
	handleErr = s.handleComAtprotoAdminEnableAccountInvites(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminGetAccountInfo(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminGetAccountInfo")
	defer span.End()
	did := c.QueryParam("did")
	var out *comatproto.AdminDefs_AccountView
	var handleErr error
	// func (s *Server) handleComAtprotoAdminGetAccountInfo(ctx context.Context,did string) (*comatproto.AdminDefs_AccountView, error)
	out, handleErr = s.handleComAtprotoAdminGetAccountInfo(ctx, did)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoAdminGetAccountInfos(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminGetAccountInfos")
	defer span.End()
	dids := c.QueryParams()["dids"]
	var out *comatproto.AdminGetAccountInfos_Output
	var handleErr error
	// func (s *Server) handleComAtprotoAdminGetAccountInfos(ctx context.Context,dids []string) (*comatproto.AdminGetAccountInfos_Output, error)
	out, handleErr = s.handleComAtprotoAdminGetAccountInfos(ctx, dids)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoAdminGetInviteCodes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminGetInviteCodes")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	sort := c.QueryParam("sort")
	var out *comatproto.AdminGetInviteCodes_Output
	var handleErr error
	// func (s *Server) handleComAtprotoAdminGetInviteCodes(ctx context.Context,cursor string,limit *int,sort string) (*comatproto.AdminGetInviteCodes_Output, error)
	out, handleErr = s.handleComAtprotoAdminGetInviteCodes(ctx, cursor, limit, sort)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoAdminGetSubjectStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminGetSubjectStatus")
	defer span.End()
	blob := c.QueryParam("blob")
	did := c.QueryParam("did")
	uri := c.QueryParam("uri")
	var out *comatproto.AdminGetSubjectStatus_Output
	var handleErr error
	// func (s *Server) handleComAtprotoAdminGetSubjectStatus(ctx context.Context,blob string,did string,uri string) (*comatproto.AdminGetSubjectStatus_Output, error)
	out, handleErr = s.handleComAtprotoAdminGetSubjectStatus(ctx, blob, did, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoAdminSearchAccounts(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminSearchAccounts")
	defer span.End()
	cursor := c.QueryParam("cursor")
	email := c.QueryParam("email")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *comatproto.AdminSearchAccounts_Output
	var handleErr error
	// func (s *Server) handleComAtprotoAdminSearchAccounts(ctx context.Context,cursor string,email string,limit *int) (*comatproto.AdminSearchAccounts_Output, error)
	out, handleErr = s.handleComAtprotoAdminSearchAccounts(ctx, cursor, email, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoAdminSendEmail(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminSendEmail")
	defer span.End()
	var body comatproto.AdminSendEmail_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.AdminSendEmail_Output
	var handleErr error
	// func (s *Server) handleComAtprotoAdminSendEmail(ctx context.Context,body *comatproto.AdminSendEmail_Input) (*comatproto.AdminSendEmail_Output, error)
	out, handleErr = s.handleComAtprotoAdminSendEmail(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoAdminUpdateAccountEmail(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminUpdateAccountEmail")
	defer span.End()
	var body comatproto.AdminUpdateAccountEmail_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminUpdateAccountEmail(ctx context.Context,body *comatproto.AdminUpdateAccountEmail_Input) error
	handleErr = s.handleComAtprotoAdminUpdateAccountEmail(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminUpdateAccountHandle(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminUpdateAccountHandle")
	defer span.End()
	var body comatproto.AdminUpdateAccountHandle_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminUpdateAccountHandle(ctx context.Context,body *comatproto.AdminUpdateAccountHandle_Input) error
	handleErr = s.handleComAtprotoAdminUpdateAccountHandle(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminUpdateAccountPassword(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminUpdateAccountPassword")
	defer span.End()
	var body comatproto.AdminUpdateAccountPassword_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminUpdateAccountPassword(ctx context.Context,body *comatproto.AdminUpdateAccountPassword_Input) error
	handleErr = s.handleComAtprotoAdminUpdateAccountPassword(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminUpdateAccountSigningKey(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminUpdateAccountSigningKey")
	defer span.End()
	var body comatproto.AdminUpdateAccountSigningKey_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoAdminUpdateAccountSigningKey(ctx context.Context,body *comatproto.AdminUpdateAccountSigningKey_Input) error
	handleErr = s.handleComAtprotoAdminUpdateAccountSigningKey(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoAdminUpdateSubjectStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoAdminUpdateSubjectStatus")
	defer span.End()
	var body comatproto.AdminUpdateSubjectStatus_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.AdminUpdateSubjectStatus_Output
	var handleErr error
	// func (s *Server) handleComAtprotoAdminUpdateSubjectStatus(ctx context.Context,body *comatproto.AdminUpdateSubjectStatus_Input) (*comatproto.AdminUpdateSubjectStatus_Output, error)
	out, handleErr = s.handleComAtprotoAdminUpdateSubjectStatus(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoIdentityGetRecommendedDidCredentials(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityGetRecommendedDidCredentials")
	defer span.End()
	var out *comatproto.IdentityGetRecommendedDidCredentials_Output
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityGetRecommendedDidCredentials(ctx context.Context) (*comatproto.IdentityGetRecommendedDidCredentials_Output, error)
	out, handleErr = s.handleComAtprotoIdentityGetRecommendedDidCredentials(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
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

func (s *Server) HandleComAtprotoIdentityRequestPlcOperationSignature(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityRequestPlcOperationSignature")
	defer span.End()
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityRequestPlcOperationSignature(ctx context.Context) error
	handleErr = s.handleComAtprotoIdentityRequestPlcOperationSignature(ctx)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoIdentityResolveDid(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityResolveDid")
	defer span.End()
	did := c.QueryParam("did")
	var out *comatproto.IdentityResolveDid_Output
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityResolveDid(ctx context.Context,did string) (*comatproto.IdentityResolveDid_Output, error)
	out, handleErr = s.handleComAtprotoIdentityResolveDid(ctx, did)
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

func (s *Server) HandleComAtprotoIdentityResolveIdentity(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityResolveIdentity")
	defer span.End()
	identifier := c.QueryParam("identifier")
	var out *comatproto.IdentityDefs_IdentityInfo
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityResolveIdentity(ctx context.Context,identifier string) (*comatproto.IdentityDefs_IdentityInfo, error)
	out, handleErr = s.handleComAtprotoIdentityResolveIdentity(ctx, identifier)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoIdentitySignPlcOperation(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentitySignPlcOperation")
	defer span.End()
	var body comatproto.IdentitySignPlcOperation_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.IdentitySignPlcOperation_Output
	var handleErr error
	// func (s *Server) handleComAtprotoIdentitySignPlcOperation(ctx context.Context,body *comatproto.IdentitySignPlcOperation_Input) (*comatproto.IdentitySignPlcOperation_Output, error)
	out, handleErr = s.handleComAtprotoIdentitySignPlcOperation(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoIdentitySubmitPlcOperation(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentitySubmitPlcOperation")
	defer span.End()
	var body comatproto.IdentitySubmitPlcOperation_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoIdentitySubmitPlcOperation(ctx context.Context,body *comatproto.IdentitySubmitPlcOperation_Input) error
	handleErr = s.handleComAtprotoIdentitySubmitPlcOperation(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoIdentityUpdateHandle(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoIdentityUpdateHandle")
	defer span.End()
	var body comatproto.IdentityUpdateHandle_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoIdentityUpdateHandle(ctx context.Context,body *comatproto.IdentityUpdateHandle_Input) error
	handleErr = s.handleComAtprotoIdentityUpdateHandle(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoLabelQueryLabels(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoLabelQueryLabels")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	sources := c.QueryParams()["sources"]
	uriPatterns := c.QueryParams()["uriPatterns"]
	var out *comatproto.LabelQueryLabels_Output
	var handleErr error
	// func (s *Server) handleComAtprotoLabelQueryLabels(ctx context.Context,cursor string,limit *int,sources []string,uriPatterns []string) (*comatproto.LabelQueryLabels_Output, error)
	out, handleErr = s.handleComAtprotoLabelQueryLabels(ctx, cursor, limit, sources, uriPatterns)
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

func (s *Server) HandleComAtprotoRepoApplyWrites(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoApplyWrites")
	defer span.End()
	var body comatproto.RepoApplyWrites_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.RepoApplyWrites_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoApplyWrites(ctx context.Context,body *comatproto.RepoApplyWrites_Input) (*comatproto.RepoApplyWrites_Output, error)
	out, handleErr = s.handleComAtprotoRepoApplyWrites(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoCreateRecord(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoCreateRecord")
	defer span.End()
	var body comatproto.RepoCreateRecord_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.RepoCreateRecord_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoCreateRecord(ctx context.Context,body *comatproto.RepoCreateRecord_Input) (*comatproto.RepoCreateRecord_Output, error)
	out, handleErr = s.handleComAtprotoRepoCreateRecord(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoDeleteRecord(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoDeleteRecord")
	defer span.End()
	var body comatproto.RepoDeleteRecord_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.RepoDeleteRecord_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoDeleteRecord(ctx context.Context,body *comatproto.RepoDeleteRecord_Input) (*comatproto.RepoDeleteRecord_Output, error)
	out, handleErr = s.handleComAtprotoRepoDeleteRecord(ctx, &body)
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

func (s *Server) HandleComAtprotoRepoImportRepo(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoImportRepo")
	defer span.End()
	body := c.Request().Body
	var handleErr error
	// func (s *Server) handleComAtprotoRepoImportRepo(ctx context.Context,r io.Reader) error
	handleErr = s.handleComAtprotoRepoImportRepo(ctx, body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoRepoListMissingBlobs(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoListMissingBlobs")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *comatproto.RepoListMissingBlobs_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoListMissingBlobs(ctx context.Context,cursor string,limit *int) (*comatproto.RepoListMissingBlobs_Output, error)
	out, handleErr = s.handleComAtprotoRepoListMissingBlobs(ctx, cursor, limit)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
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
	// func (s *Server) handleComAtprotoRepoListRecords(ctx context.Context,collection string,cursor string,limit *int,repo string,reverse *bool) (*comatproto.RepoListRecords_Output, error)
	out, handleErr = s.handleComAtprotoRepoListRecords(ctx, collection, cursor, limit, repo, reverse)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoPutRecord(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoPutRecord")
	defer span.End()
	var body comatproto.RepoPutRecord_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.RepoPutRecord_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoPutRecord(ctx context.Context,body *comatproto.RepoPutRecord_Input) (*comatproto.RepoPutRecord_Output, error)
	out, handleErr = s.handleComAtprotoRepoPutRecord(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoRepoUploadBlob(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoRepoUploadBlob")
	defer span.End()
	body := c.Request().Body
	var out *comatproto.RepoUploadBlob_Output
	var handleErr error
	// func (s *Server) handleComAtprotoRepoUploadBlob(ctx context.Context,r io.Reader) (*comatproto.RepoUploadBlob_Output, error)
	out, handleErr = s.handleComAtprotoRepoUploadBlob(ctx, body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerActivateAccount(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerActivateAccount")
	defer span.End()
	var handleErr error
	// func (s *Server) handleComAtprotoServerActivateAccount(ctx context.Context) error
	handleErr = s.handleComAtprotoServerActivateAccount(ctx)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerCheckAccountStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerCheckAccountStatus")
	defer span.End()
	var out *comatproto.ServerCheckAccountStatus_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerCheckAccountStatus(ctx context.Context) (*comatproto.ServerCheckAccountStatus_Output, error)
	out, handleErr = s.handleComAtprotoServerCheckAccountStatus(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerConfirmEmail(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerConfirmEmail")
	defer span.End()
	var body comatproto.ServerConfirmEmail_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoServerConfirmEmail(ctx context.Context,body *comatproto.ServerConfirmEmail_Input) error
	handleErr = s.handleComAtprotoServerConfirmEmail(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerCreateAccount(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerCreateAccount")
	defer span.End()
	var body comatproto.ServerCreateAccount_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.ServerCreateAccount_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerCreateAccount(ctx context.Context,body *comatproto.ServerCreateAccount_Input) (*comatproto.ServerCreateAccount_Output, error)
	out, handleErr = s.handleComAtprotoServerCreateAccount(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerCreateAppPassword(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerCreateAppPassword")
	defer span.End()
	var body comatproto.ServerCreateAppPassword_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.ServerCreateAppPassword_AppPassword
	var handleErr error
	// func (s *Server) handleComAtprotoServerCreateAppPassword(ctx context.Context,body *comatproto.ServerCreateAppPassword_Input) (*comatproto.ServerCreateAppPassword_AppPassword, error)
	out, handleErr = s.handleComAtprotoServerCreateAppPassword(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerCreateInviteCode(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerCreateInviteCode")
	defer span.End()
	var body comatproto.ServerCreateInviteCode_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.ServerCreateInviteCode_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerCreateInviteCode(ctx context.Context,body *comatproto.ServerCreateInviteCode_Input) (*comatproto.ServerCreateInviteCode_Output, error)
	out, handleErr = s.handleComAtprotoServerCreateInviteCode(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerCreateInviteCodes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerCreateInviteCodes")
	defer span.End()
	var body comatproto.ServerCreateInviteCodes_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.ServerCreateInviteCodes_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerCreateInviteCodes(ctx context.Context,body *comatproto.ServerCreateInviteCodes_Input) (*comatproto.ServerCreateInviteCodes_Output, error)
	out, handleErr = s.handleComAtprotoServerCreateInviteCodes(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerCreateSession(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerCreateSession")
	defer span.End()
	var body comatproto.ServerCreateSession_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.ServerCreateSession_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerCreateSession(ctx context.Context,body *comatproto.ServerCreateSession_Input) (*comatproto.ServerCreateSession_Output, error)
	out, handleErr = s.handleComAtprotoServerCreateSession(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerDeactivateAccount(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerDeactivateAccount")
	defer span.End()
	var body comatproto.ServerDeactivateAccount_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoServerDeactivateAccount(ctx context.Context,body *comatproto.ServerDeactivateAccount_Input) error
	handleErr = s.handleComAtprotoServerDeactivateAccount(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerDeleteAccount(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerDeleteAccount")
	defer span.End()
	var body comatproto.ServerDeleteAccount_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoServerDeleteAccount(ctx context.Context,body *comatproto.ServerDeleteAccount_Input) error
	handleErr = s.handleComAtprotoServerDeleteAccount(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerDeleteSession(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerDeleteSession")
	defer span.End()
	var handleErr error
	// func (s *Server) handleComAtprotoServerDeleteSession(ctx context.Context) error
	handleErr = s.handleComAtprotoServerDeleteSession(ctx)
	if handleErr != nil {
		return handleErr
	}
	return nil
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

func (s *Server) HandleComAtprotoServerGetAccountInviteCodes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerGetAccountInviteCodes")
	defer span.End()
	var createAvailable *bool
	if p := c.QueryParam("createAvailable"); p != "" {
		createAvailable_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		createAvailable = &createAvailable_val
	}
	var includeUsed *bool
	if p := c.QueryParam("includeUsed"); p != "" {
		includeUsed_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		includeUsed = &includeUsed_val
	}
	var out *comatproto.ServerGetAccountInviteCodes_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerGetAccountInviteCodes(ctx context.Context,createAvailable *bool,includeUsed *bool) (*comatproto.ServerGetAccountInviteCodes_Output, error)
	out, handleErr = s.handleComAtprotoServerGetAccountInviteCodes(ctx, createAvailable, includeUsed)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerGetServiceAuth(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerGetServiceAuth")
	defer span.End()
	aud := c.QueryParam("aud")
	var exp *int
	if p := c.QueryParam("exp"); p != "" {
		exp_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		exp = &exp_val
	}
	lxm := c.QueryParam("lxm")
	var out *comatproto.ServerGetServiceAuth_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerGetServiceAuth(ctx context.Context,aud string,exp *int,lxm string) (*comatproto.ServerGetServiceAuth_Output, error)
	out, handleErr = s.handleComAtprotoServerGetServiceAuth(ctx, aud, exp, lxm)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerGetSession(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerGetSession")
	defer span.End()
	var out *comatproto.ServerGetSession_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerGetSession(ctx context.Context) (*comatproto.ServerGetSession_Output, error)
	out, handleErr = s.handleComAtprotoServerGetSession(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerListAppPasswords(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerListAppPasswords")
	defer span.End()
	var out *comatproto.ServerListAppPasswords_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerListAppPasswords(ctx context.Context) (*comatproto.ServerListAppPasswords_Output, error)
	out, handleErr = s.handleComAtprotoServerListAppPasswords(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerRefreshSession(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerRefreshSession")
	defer span.End()
	var out *comatproto.ServerRefreshSession_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerRefreshSession(ctx context.Context) (*comatproto.ServerRefreshSession_Output, error)
	out, handleErr = s.handleComAtprotoServerRefreshSession(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerRequestAccountDelete(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerRequestAccountDelete")
	defer span.End()
	var handleErr error
	// func (s *Server) handleComAtprotoServerRequestAccountDelete(ctx context.Context) error
	handleErr = s.handleComAtprotoServerRequestAccountDelete(ctx)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerRequestEmailConfirmation(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerRequestEmailConfirmation")
	defer span.End()
	var handleErr error
	// func (s *Server) handleComAtprotoServerRequestEmailConfirmation(ctx context.Context) error
	handleErr = s.handleComAtprotoServerRequestEmailConfirmation(ctx)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerRequestEmailUpdate(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerRequestEmailUpdate")
	defer span.End()
	var out *comatproto.ServerRequestEmailUpdate_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerRequestEmailUpdate(ctx context.Context) (*comatproto.ServerRequestEmailUpdate_Output, error)
	out, handleErr = s.handleComAtprotoServerRequestEmailUpdate(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerRequestPasswordReset(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerRequestPasswordReset")
	defer span.End()
	var body comatproto.ServerRequestPasswordReset_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoServerRequestPasswordReset(ctx context.Context,body *comatproto.ServerRequestPasswordReset_Input) error
	handleErr = s.handleComAtprotoServerRequestPasswordReset(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerReserveSigningKey(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerReserveSigningKey")
	defer span.End()
	var body comatproto.ServerReserveSigningKey_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.ServerReserveSigningKey_Output
	var handleErr error
	// func (s *Server) handleComAtprotoServerReserveSigningKey(ctx context.Context,body *comatproto.ServerReserveSigningKey_Input) (*comatproto.ServerReserveSigningKey_Output, error)
	out, handleErr = s.handleComAtprotoServerReserveSigningKey(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoServerResetPassword(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerResetPassword")
	defer span.End()
	var body comatproto.ServerResetPassword_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoServerResetPassword(ctx context.Context,body *comatproto.ServerResetPassword_Input) error
	handleErr = s.handleComAtprotoServerResetPassword(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerRevokeAppPassword(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerRevokeAppPassword")
	defer span.End()
	var body comatproto.ServerRevokeAppPassword_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoServerRevokeAppPassword(ctx context.Context,body *comatproto.ServerRevokeAppPassword_Input) error
	handleErr = s.handleComAtprotoServerRevokeAppPassword(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoServerUpdateEmail(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoServerUpdateEmail")
	defer span.End()
	var body comatproto.ServerUpdateEmail_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoServerUpdateEmail(ctx context.Context,body *comatproto.ServerUpdateEmail_Input) error
	handleErr = s.handleComAtprotoServerUpdateEmail(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoSyncGetBlob(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetBlob")
	defer span.End()
	cid := c.QueryParam("cid")
	did := c.QueryParam("did")
	var out io.Reader
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetBlob(ctx context.Context,cid string,did string) (io.Reader, error)
	out, handleErr = s.handleComAtprotoSyncGetBlob(ctx, cid, did)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandleComAtprotoSyncGetBlocks(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetBlocks")
	defer span.End()
	cids := c.QueryParams()["cids"]
	did := c.QueryParam("did")
	var out io.Reader
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetBlocks(ctx context.Context,cids []string,did string) (io.Reader, error)
	out, handleErr = s.handleComAtprotoSyncGetBlocks(ctx, cids, did)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandleComAtprotoSyncGetCheckout(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetCheckout")
	defer span.End()
	did := c.QueryParam("did")
	var out io.Reader
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetCheckout(ctx context.Context,did string) (io.Reader, error)
	out, handleErr = s.handleComAtprotoSyncGetCheckout(ctx, did)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandleComAtprotoSyncGetHead(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetHead")
	defer span.End()
	did := c.QueryParam("did")
	var out *comatproto.SyncGetHead_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetHead(ctx context.Context,did string) (*comatproto.SyncGetHead_Output, error)
	out, handleErr = s.handleComAtprotoSyncGetHead(ctx, did)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncGetHostStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetHostStatus")
	defer span.End()
	hostname := c.QueryParam("hostname")
	var out *comatproto.SyncGetHostStatus_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetHostStatus(ctx context.Context,hostname string) (*comatproto.SyncGetHostStatus_Output, error)
	out, handleErr = s.handleComAtprotoSyncGetHostStatus(ctx, hostname)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncGetLatestCommit(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetLatestCommit")
	defer span.End()
	did := c.QueryParam("did")
	var out *comatproto.SyncGetLatestCommit_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetLatestCommit(ctx context.Context,did string) (*comatproto.SyncGetLatestCommit_Output, error)
	out, handleErr = s.handleComAtprotoSyncGetLatestCommit(ctx, did)
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
	return c.Stream(200, "application/octet-stream", out)
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
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandleComAtprotoSyncGetRepoStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncGetRepoStatus")
	defer span.End()
	did := c.QueryParam("did")
	var out *comatproto.SyncGetRepoStatus_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncGetRepoStatus(ctx context.Context,did string) (*comatproto.SyncGetRepoStatus_Output, error)
	out, handleErr = s.handleComAtprotoSyncGetRepoStatus(ctx, did)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncListBlobs(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncListBlobs")
	defer span.End()
	cursor := c.QueryParam("cursor")
	did := c.QueryParam("did")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	since := c.QueryParam("since")
	var out *comatproto.SyncListBlobs_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncListBlobs(ctx context.Context,cursor string,did string,limit *int,since string) (*comatproto.SyncListBlobs_Output, error)
	out, handleErr = s.handleComAtprotoSyncListBlobs(ctx, cursor, did, limit, since)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncListHosts(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncListHosts")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *comatproto.SyncListHosts_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncListHosts(ctx context.Context,cursor string,limit *int) (*comatproto.SyncListHosts_Output, error)
	out, handleErr = s.handleComAtprotoSyncListHosts(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncListRepos(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncListRepos")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *comatproto.SyncListRepos_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncListRepos(ctx context.Context,cursor string,limit *int) (*comatproto.SyncListRepos_Output, error)
	out, handleErr = s.handleComAtprotoSyncListRepos(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncListReposByCollection(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncListReposByCollection")
	defer span.End()
	collection := c.QueryParam("collection")
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *comatproto.SyncListReposByCollection_Output
	var handleErr error
	// func (s *Server) handleComAtprotoSyncListReposByCollection(ctx context.Context,collection string,cursor string,limit *int) (*comatproto.SyncListReposByCollection_Output, error)
	out, handleErr = s.handleComAtprotoSyncListReposByCollection(ctx, collection, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoSyncNotifyOfUpdate(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncNotifyOfUpdate")
	defer span.End()
	var body comatproto.SyncNotifyOfUpdate_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoSyncNotifyOfUpdate(ctx context.Context,body *comatproto.SyncNotifyOfUpdate_Input) error
	handleErr = s.handleComAtprotoSyncNotifyOfUpdate(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoSyncRequestCrawl(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoSyncRequestCrawl")
	defer span.End()
	var body comatproto.SyncRequestCrawl_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoSyncRequestCrawl(ctx context.Context,body *comatproto.SyncRequestCrawl_Input) error
	handleErr = s.handleComAtprotoSyncRequestCrawl(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoTempAddReservedHandle(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoTempAddReservedHandle")
	defer span.End()
	var body comatproto.TempAddReservedHandle_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *comatproto.TempAddReservedHandle_Output
	var handleErr error
	// func (s *Server) handleComAtprotoTempAddReservedHandle(ctx context.Context,body *comatproto.TempAddReservedHandle_Input) (*comatproto.TempAddReservedHandle_Output, error)
	out, handleErr = s.handleComAtprotoTempAddReservedHandle(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoTempCheckHandleAvailability(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoTempCheckHandleAvailability")
	defer span.End()
	birthDate := c.QueryParam("birthDate")
	email := c.QueryParam("email")
	handle := c.QueryParam("handle")
	var out *comatproto.TempCheckHandleAvailability_Output
	var handleErr error
	// func (s *Server) handleComAtprotoTempCheckHandleAvailability(ctx context.Context,birthDate string,email string,handle string) (*comatproto.TempCheckHandleAvailability_Output, error)
	out, handleErr = s.handleComAtprotoTempCheckHandleAvailability(ctx, birthDate, email, handle)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoTempCheckSignupQueue(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoTempCheckSignupQueue")
	defer span.End()
	var out *comatproto.TempCheckSignupQueue_Output
	var handleErr error
	// func (s *Server) handleComAtprotoTempCheckSignupQueue(ctx context.Context) (*comatproto.TempCheckSignupQueue_Output, error)
	out, handleErr = s.handleComAtprotoTempCheckSignupQueue(ctx)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoTempDereferenceScope(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoTempDereferenceScope")
	defer span.End()
	scope := c.QueryParam("scope")
	var out *comatproto.TempDereferenceScope_Output
	var handleErr error
	// func (s *Server) handleComAtprotoTempDereferenceScope(ctx context.Context,scope string) (*comatproto.TempDereferenceScope_Output, error)
	out, handleErr = s.handleComAtprotoTempDereferenceScope(ctx, scope)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoTempFetchLabels(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoTempFetchLabels")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var since *int
	if p := c.QueryParam("since"); p != "" {
		since_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		since = &since_val
	}
	var out *comatproto.TempFetchLabels_Output
	var handleErr error
	// func (s *Server) handleComAtprotoTempFetchLabels(ctx context.Context,limit *int,since *int) (*comatproto.TempFetchLabels_Output, error)
	out, handleErr = s.handleComAtprotoTempFetchLabels(ctx, limit, since)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandleComAtprotoTempRequestPhoneVerification(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoTempRequestPhoneVerification")
	defer span.End()
	var body comatproto.TempRequestPhoneVerification_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoTempRequestPhoneVerification(ctx context.Context,body *comatproto.TempRequestPhoneVerification_Input) error
	handleErr = s.handleComAtprotoTempRequestPhoneVerification(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) HandleComAtprotoTempRevokeAccountCredentials(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoTempRevokeAccountCredentials")
	defer span.End()
	var body comatproto.TempRevokeAccountCredentials_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var handleErr error
	// func (s *Server) handleComAtprotoTempRevokeAccountCredentials(ctx context.Context,body *comatproto.TempRevokeAccountCredentials_Input) error
	handleErr = s.handleComAtprotoTempRevokeAccountCredentials(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return nil
}

func (s *Server) RegisterHandlersGamesgamesgamesgamesgames(e *echo.Echo) error {
	e.GET("/xrpc/games.gamesgamesgamesgames.search", s.HandleGamesGamesgamesgamesgamesSearch)
	return nil
}

func (s *Server) HandleGamesGamesgamesgamesgamesSearch(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleGamesGamesgamesgamesgamesSearch")
	defer span.End()
	ageRatings := c.QueryParams()["ageRatings"]
	applicationTypes := c.QueryParams()["applicationTypes"]
	cursor := c.QueryParam("cursor")
	genres := c.QueryParams()["genres"]
	var includeCancelled *bool
	if p := c.QueryParam("includeCancelled"); p != "" {
		includeCancelled_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		includeCancelled = &includeCancelled_val
	}
	var includeUnrated *bool
	if p := c.QueryParam("includeUnrated"); p != "" {
		includeUnrated_val, err := strconv.ParseBool(p)
		if err != nil {
			return err
		}
		includeUnrated = &includeUnrated_val
	}
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	modes := c.QueryParams()["modes"]
	playerPerspectives := c.QueryParams()["playerPerspectives"]
	q := c.QueryParam("q")
	sort := c.QueryParam("sort")
	themes := c.QueryParams()["themes"]
	types := c.QueryParams()["types"]
	var out *gamesgamesgamesgamesgames.Search_Output
	var handleErr error
	// func (s *Server) handleGamesGamesgamesgamesgamesSearch(ctx context.Context,ageRatings []string,applicationTypes []string,cursor string,genres []string,includeCancelled *bool,includeUnrated *bool,limit *int,modes []string,playerPerspectives []string,q string,sort string,themes []string,types []string) (*gamesgamesgamesgamesgames.Search_Output, error)
	out, handleErr = s.handleGamesGamesgamesgamesgamesSearch(ctx, ageRatings, applicationTypes, cursor, genres, includeCancelled, includeUnrated, limit, modes, playerPerspectives, q, sort, themes, types)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersPlacestream(e *echo.Echo) error {
	e.GET("/xrpc/place.stream.badge.getIssuedBadges", s.HandlePlaceStreamBadgeGetIssuedBadges)
	e.GET("/xrpc/place.stream.badge.getValidBadges", s.HandlePlaceStreamBadgeGetValidBadges)
	e.GET("/xrpc/place.stream.beta.getStatus", s.HandlePlaceStreamBetaGetStatus)
	e.POST("/xrpc/place.stream.branding.deleteBlob", s.HandlePlaceStreamBrandingDeleteBlob)
	e.GET("/xrpc/place.stream.branding.getBlob", s.HandlePlaceStreamBrandingGetBlob)
	e.GET("/xrpc/place.stream.branding.getBranding", s.HandlePlaceStreamBrandingGetBranding)
	e.POST("/xrpc/place.stream.branding.updateBlob", s.HandlePlaceStreamBrandingUpdateBlob)
	e.GET("/xrpc/place.stream.broadcast.getBroadcaster", s.HandlePlaceStreamBroadcastGetBroadcaster)
	e.GET("/xrpc/place.stream.config.getEnv", s.HandlePlaceStreamConfigGetEnv)
	e.GET("/xrpc/place.stream.game.getGame", s.HandlePlaceStreamGameGetGame)
	e.GET("/xrpc/place.stream.game.search", s.HandlePlaceStreamGameSearch)
	e.GET("/xrpc/place.stream.getLikes", s.HandlePlaceStreamGetLikes)
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
	e.POST("/xrpc/place.stream.media.finalizeLivestream", s.HandlePlaceStreamMediaFinalizeLivestream)
	e.GET("/xrpc/place.stream.media.getUploadStatus", s.HandlePlaceStreamMediaGetUploadStatus)
	e.GET("/xrpc/place.stream.media.getVideo", s.HandlePlaceStreamMediaGetVideo)
	e.GET("/xrpc/place.stream.media.getVideoList", s.HandlePlaceStreamMediaGetVideoList)
	e.POST("/xrpc/place.stream.media.publishVideo", s.HandlePlaceStreamMediaPublishVideo)
	e.POST("/xrpc/place.stream.moderation.createBlock", s.HandlePlaceStreamModerationCreateBlock)
	e.POST("/xrpc/place.stream.moderation.createGate", s.HandlePlaceStreamModerationCreateGate)
	e.POST("/xrpc/place.stream.moderation.createPin", s.HandlePlaceStreamModerationCreatePin)
	e.POST("/xrpc/place.stream.moderation.createVodGate", s.HandlePlaceStreamModerationCreateVodGate)
	e.POST("/xrpc/place.stream.moderation.deleteBlock", s.HandlePlaceStreamModerationDeleteBlock)
	e.POST("/xrpc/place.stream.moderation.deleteGate", s.HandlePlaceStreamModerationDeleteGate)
	e.POST("/xrpc/place.stream.moderation.deletePin", s.HandlePlaceStreamModerationDeletePin)
	e.POST("/xrpc/place.stream.moderation.deleteVodGate", s.HandlePlaceStreamModerationDeleteVodGate)
	e.POST("/xrpc/place.stream.moderation.updateLivestream", s.HandlePlaceStreamModerationUpdateLivestream)
	e.POST("/xrpc/place.stream.multistream.createTarget", s.HandlePlaceStreamMultistreamCreateTarget)
	e.POST("/xrpc/place.stream.multistream.deleteTarget", s.HandlePlaceStreamMultistreamDeleteTarget)
	e.GET("/xrpc/place.stream.multistream.listTargets", s.HandlePlaceStreamMultistreamListTargets)
	e.POST("/xrpc/place.stream.multistream.putTarget", s.HandlePlaceStreamMultistreamPutTarget)
	e.GET("/xrpc/place.stream.playback.getLivePlaylist", s.HandlePlaceStreamPlaybackGetLivePlaylist)
	e.GET("/xrpc/place.stream.playback.getLiveSegment", s.HandlePlaceStreamPlaybackGetLiveSegment)
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
	e.POST("/xrpc/place.stream.vod.createDraft", s.HandlePlaceStreamVodCreateDraft)
	e.POST("/xrpc/place.stream.vod.deleteDraft", s.HandlePlaceStreamVodDeleteDraft)
	e.GET("/xrpc/place.stream.vod.getComments", s.HandlePlaceStreamVodGetComments)
	e.GET("/xrpc/place.stream.vod.getDraft", s.HandlePlaceStreamVodGetDraft)
	e.GET("/xrpc/place.stream.vod.listDrafts", s.HandlePlaceStreamVodListDrafts)
	e.POST("/xrpc/place.stream.vod.publishDraft", s.HandlePlaceStreamVodPublishDraft)
	e.POST("/xrpc/place.stream.vod.updateDraft", s.HandlePlaceStreamVodUpdateDraft)
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

func (s *Server) HandlePlaceStreamBetaGetStatus(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamBetaGetStatus")
	defer span.End()
	did := c.QueryParam("did")
	feature := c.QueryParam("feature")
	var out *placestream.BetaGetStatus_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamBetaGetStatus(ctx context.Context,did string,feature string) (*placestream.BetaGetStatus_Output, error)
	out, handleErr = s.handlePlaceStreamBetaGetStatus(ctx, did, feature)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	q := c.QueryParam("q")
	var out *placestream.GameSearch_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamGameSearch(ctx context.Context,cursor string,limit *int,q string) (*placestream.GameSearch_Output, error)
	out, handleErr = s.handlePlaceStreamGameSearch(ctx, cursor, limit, q)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamGetLikes(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamGetLikes")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	subject := c.QueryParam("subject")
	var out *placestream.GetLikes_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamGetLikes(ctx context.Context,cursor string,limit *int,subject string) (*placestream.GetLikes_Output, error)
	out, handleErr = s.handlePlaceStreamGetLikes(ctx, cursor, limit, subject)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *placestream.LiveGetLiveUsers_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetLiveUsers(ctx context.Context,before string,limit *int) (*placestream.LiveGetLiveUsers_Output, error)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	userDID := c.QueryParam("userDID")
	var out *placestream.LiveGetSegments_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveGetSegments(ctx context.Context,before string,limit *int,userDID string) (*placestream.LiveGetSegments_Output, error)
	out, handleErr = s.handlePlaceStreamLiveGetSegments(ctx, before, limit, userDID)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamLiveSearchActorsTypeahead(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamLiveSearchActorsTypeahead")
	defer span.End()
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	q := c.QueryParam("q")
	var out *placestream.LiveSearchActorsTypeahead_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamLiveSearchActorsTypeahead(ctx context.Context,limit *int,q string) (*placestream.LiveSearchActorsTypeahead_Output, error)
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

func (s *Server) HandlePlaceStreamMediaFinalizeLivestream(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamMediaFinalizeLivestream")
	defer span.End()
	var body placestream.MediaFinalizeLivestream_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.MediaFinalizeLivestream_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMediaFinalizeLivestream(ctx context.Context,body *placestream.MediaFinalizeLivestream_Input) (*placestream.MediaFinalizeLivestream_Output, error)
	out, handleErr = s.handlePlaceStreamMediaFinalizeLivestream(ctx, &body)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	repo := c.QueryParam("repo")
	var out *placestream.MediaGetVideoList_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMediaGetVideoList(ctx context.Context,cursor string,limit *int,repo string) (*placestream.MediaGetVideoList_Output, error)
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

func (s *Server) HandlePlaceStreamModerationCreateVodGate(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationCreateVodGate")
	defer span.End()
	var body placestream.ModerationCreateVodGate_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationCreateVodGate_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationCreateVodGate(ctx context.Context,body *placestream.ModerationCreateVodGate_Input) (*placestream.ModerationCreateVodGate_Output, error)
	out, handleErr = s.handlePlaceStreamModerationCreateVodGate(ctx, &body)
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

func (s *Server) HandlePlaceStreamModerationDeleteVodGate(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamModerationDeleteVodGate")
	defer span.End()
	var body placestream.ModerationDeleteVodGate_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.ModerationDeleteVodGate_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamModerationDeleteVodGate(ctx context.Context,body *placestream.ModerationDeleteVodGate_Input) (*placestream.ModerationDeleteVodGate_Output, error)
	out, handleErr = s.handlePlaceStreamModerationDeleteVodGate(ctx, &body)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *placestream.MultistreamListTargets_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamMultistreamListTargets(ctx context.Context,cursor string,limit *int) (*placestream.MultistreamListTargets_Output, error)
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

func (s *Server) HandlePlaceStreamPlaybackGetLivePlaylist(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamPlaybackGetLivePlaylist")
	defer span.End()
	sid := c.QueryParam("sid")
	streamer := c.QueryParam("streamer")
	track := c.QueryParam("track")
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamPlaybackGetLivePlaylist(ctx context.Context,sid string,streamer string,track string) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamPlaybackGetLivePlaylist(ctx, sid, streamer, track)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
}

func (s *Server) HandlePlaceStreamPlaybackGetLiveSegment(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamPlaybackGetLiveSegment")
	defer span.End()
	seg := c.QueryParam("seg")
	sid := c.QueryParam("sid")
	streamer := c.QueryParam("streamer")
	track := c.QueryParam("track")
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamPlaybackGetLiveSegment(ctx context.Context,seg string,sid string,streamer string,track string) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamPlaybackGetLiveSegment(ctx, seg, sid, streamer, track)
	if handleErr != nil {
		return handleErr
	}
	return c.Stream(200, "application/octet-stream", out)
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
	return c.Stream(200, "application/octet-stream", out)
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
	body := c.Request().Body
	var out io.Reader
	var handleErr error
	// func (s *Server) handlePlaceStreamPlaybackWhep(ctx context.Context,r io.Reader) (io.Reader, error)
	out, handleErr = s.handlePlaceStreamPlaybackWhep(ctx, body)
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
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *placestream.ServerListWebhooks_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamServerListWebhooks(ctx context.Context,active *bool,cursor string,event string,limit *int) (*placestream.ServerListWebhooks_Output, error)
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

func (s *Server) HandlePlaceStreamVodCreateDraft(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamVodCreateDraft")
	defer span.End()
	var body placestream.VodCreateDraft_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.VodCreateDraft_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamVodCreateDraft(ctx context.Context,body *placestream.VodCreateDraft_Input) (*placestream.VodCreateDraft_Output, error)
	out, handleErr = s.handlePlaceStreamVodCreateDraft(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamVodDeleteDraft(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamVodDeleteDraft")
	defer span.End()
	var body placestream.VodDeleteDraft_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.VodDeleteDraft_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamVodDeleteDraft(ctx context.Context,body *placestream.VodDeleteDraft_Input) (*placestream.VodDeleteDraft_Output, error)
	out, handleErr = s.handlePlaceStreamVodDeleteDraft(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamVodGetComments(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamVodGetComments")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	video := c.QueryParam("video")
	var out *placestream.VodGetComments_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamVodGetComments(ctx context.Context,cursor string,limit *int,video string) (*placestream.VodGetComments_Output, error)
	out, handleErr = s.handlePlaceStreamVodGetComments(ctx, cursor, limit, video)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamVodGetDraft(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamVodGetDraft")
	defer span.End()
	uri := c.QueryParam("uri")
	var out *placestream.VodGetDraft_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamVodGetDraft(ctx context.Context,uri string) (*placestream.VodGetDraft_Output, error)
	out, handleErr = s.handlePlaceStreamVodGetDraft(ctx, uri)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamVodListDrafts(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamVodListDrafts")
	defer span.End()
	cursor := c.QueryParam("cursor")
	var limit *int
	if p := c.QueryParam("limit"); p != "" {
		limit_val, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		limit = &limit_val
	}
	var out *placestream.VodListDrafts_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamVodListDrafts(ctx context.Context,cursor string,limit *int) (*placestream.VodListDrafts_Output, error)
	out, handleErr = s.handlePlaceStreamVodListDrafts(ctx, cursor, limit)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamVodPublishDraft(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamVodPublishDraft")
	defer span.End()
	var body placestream.VodPublishDraft_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.VodPublishDraft_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamVodPublishDraft(ctx context.Context,body *placestream.VodPublishDraft_Input) (*placestream.VodPublishDraft_Output, error)
	out, handleErr = s.handlePlaceStreamVodPublishDraft(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) HandlePlaceStreamVodUpdateDraft(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamVodUpdateDraft")
	defer span.End()
	var body placestream.VodUpdateDraft_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestream.VodUpdateDraft_Output
	var handleErr error
	// func (s *Server) handlePlaceStreamVodUpdateDraft(ctx context.Context,body *placestream.VodUpdateDraft_Input) (*placestream.VodUpdateDraft_Output, error)
	out, handleErr = s.handlePlaceStreamVodUpdateDraft(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}
