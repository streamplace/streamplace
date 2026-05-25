package spxrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	bsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/spmetrics"

	placestream "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamLiveDenyTeleport(ctx context.Context, input *placestream.LiveDenyTeleport_Input) (*placestream.LiveDenyTeleport_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	if input.Uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "URI is required")
	}

	teleport, err := s.model.GetTeleportByURI(input.Uri)
	if err != nil {
		log.Error(ctx, "failed to get teleport", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to retrieve teleport")
	}

	if teleport == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Teleport not found")
	}

	if teleport.TargetDID != session.DID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You are not the target of this teleport")
	}

	err = s.model.DenyTeleport(ctx, input.Uri)
	if err != nil {
		log.Error(ctx, "failed to deny teleport", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to deny teleport")
	}

	cancelMsg := &placestream.Livestream_TeleportCanceled{
		LexiconTypeID: "place.stream.livestream#teleportCanceled",
		TeleportUri:   input.Uri,
		Reason:        "denied",
	}

	s.bus.Publish(teleport.RepoDID, cancelMsg)
	s.bus.Publish(teleport.TargetDID, cancelMsg)

	return &placestream.LiveDenyTeleport_Output{
		Success: true,
	}, nil
}

var replicationUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 1024 * 10, // 10MB
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handlePlaceStreamLiveGetSegments(ctx context.Context, before string, limit int, userDID string) (*placestream.LiveGetSegments_Output, error) {
	if userDID == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "User DID is required")
	}
	var beforeTime *time.Time
	if before != "" {
		parsedTime, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid 'before' parameter: must be RFC3339 format")
		}
		beforeTime = &parsedTime
	}

	includeUnpublished := false
	sess, _ := oatproxy.GetOAuthSession(ctx)
	if sess != nil && sess.DID == userDID {
		includeUnpublished = true
		// this user gets sent right to the origin in case we're unpublished
		origin, err := s.statefulDB.GetLatestBroadcastOriginForStreamer(sess.DID)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting broadcast origin", err)
		}
		myServerDID := s.cli.ServerDID()
		if origin != nil && origin.ServerDID != myServerDID {
			data, err := s.ProxyServiceRequest(ctx, origin.ServerDID, "GET", "place.stream.live.getSegments",
				url.Values{"userDID": {userDID}, "limit": {strconv.Itoa(limit)}, "before": {before}},
				nil, "application/json")
			if err != nil {
				return nil, fmt.Errorf("error proxying to peer: %w", err)
			}
			var output placestream.LiveGetSegments_Output
			err = json.Unmarshal(data, &output)
			if err != nil {
				return nil, fmt.Errorf("error unmarshalling response: %w", err)
			}
			return &output, nil
		}
	} else {
		svc := GetServiceAuth(ctx)
		if svc != nil {
			// this is a signed request from a peer node, allow them to see unpublished streams
			includeUnpublished = true
		}
	}

	segments, err := s.localDB.LatestSegmentsForUser(userDID, limit, includeUnpublished, beforeTime, nil)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch segments")
	}

	// Convert segments to the expected output format
	output := &placestream.LiveGetSegments_Output{
		Segments: make([]*placestream.Segment_SegmentView, len(segments)),
	}

	for i, segment := range segments {
		record, err := segment.ToStreamplaceSegment()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to convert segment to streamplace segment: %s", err))
		}
		c, err := spid.GetCID(record)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get CID: %s", err))
		}
		ltd := &lexutil.LexiconTypeDecoder{Val: record}

		output.Segments[i] = &placestream.Segment_SegmentView{
			Record: ltd,
			Cid:    c.String(),
		}
	}

	return output, nil
}

func (s *Server) handlePlaceStreamLiveGetLiveUsers(ctx context.Context, before string, limit int) (*placestream.LiveGetLiveUsers_Output, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("live_users_%s_%d", before, limit)
	if cached, found := s.LiveUsersCache.Get(cacheKey); found {
		return cached.(*placestream.LiveGetLiveUsers_Output), nil
	}

	var beforeTime *time.Time
	if before != "" {
		parsedTime, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid 'before' parameter: must be RFC3339 format")
		}
		beforeTime = &parsedTime
	}
	segs, err := s.localDB.MostRecentSegments()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch recent segments")
	}
	dids := make([]string, len(segs))
	for i, seg := range segs {
		dids[i] = seg.RepoDID
	}
	ls, err := s.model.GetLatestLivestreams(limit, beforeTime, dids)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch livestreams")
	}

	streams := make([]*placestream.Livestream_LivestreamView, len(ls))

	for i, l := range ls {
		stream, err := l.ToLivestreamView()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to convert livestream to streamplace livestream: %s", err))
		}
		viewers := s.bus.GetViewerCount(stream.Author.Did)
		stream.ViewerCount = &placestream.Livestream_ViewerCount{
			LexiconTypeID: "place.stream.livestream#viewerCount",
			Count:         int64(viewers),
		}
		streams[i] = stream
	}

	liveUsers := &placestream.LiveGetLiveUsers_Output{
		Streams: streams,
	}

	// Cache the result
	s.LiveUsersCache.SetDefault(cacheKey, liveUsers)

	return liveUsers, nil
}

func (s *Server) handlePlaceStreamLiveSubscribeSegments(c echo.Context) error {
	if s.cli.DisableSyndication {
		return echo.NewHTTPError(http.StatusNotImplemented, "Syndication is disabled")
	}
	user := c.QueryParam("streamer")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "User DID is required")
	}
	spmetrics.ReplicationWebsocketsOpen.Inc()
	defer spmetrics.ReplicationWebsocketsOpen.Dec()
	ws, err := replicationUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()
	go func() {

		segChan := s.bus.SubscribeSegmentBuf(ctx, user, "source", 2)
		defer s.bus.UnsubscribeSegment(ctx, user, "source", segChan)
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case file := <-segChan.C:
				if !file.Published {
					continue
				}
				log.Debug(ctx, "got segment", "file", file.Filepath)
				err := ws.WriteMessage(websocket.BinaryMessage, file.Data)
				if err != nil {
					log.Error(ctx, "could not write message", "error", err)
					cancel()
					return
				}
			}
		}

	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Read
		_, msg, err := ws.ReadMessage()
		if err != nil {
			c.Logger().Error(err)
			return err
		}
		log.Debug(c.Request().Context(), "received message", "message", string(msg))
	}
}

func (s *Server) handlePlaceStreamLiveGetRecommendations(ctx context.Context, userDID string) (*placestream.LiveGetRecommendations_Output, error) {
	if userDID == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "userDID is required")
	}

	// Try to get streamer's recommendation list
	rec, err := s.model.GetRecommendation(userDID)
	// If we have a recommendation list, filter for live streamers
	if err == nil {
		streamers, err := rec.GetStreamersArray()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to parse recommendations")
		}

		// Filter for only live streamers
		liveStreamers, err := s.localDB.FilterLiveRepoDIDs(streamers)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to filter live streamers")
		}

		if len(liveStreamers) > 0 {
			var recommendations []*placestream.LiveGetRecommendations_Output_Recommendations_Elem
			for _, did := range liveStreamers {
				recommendations = append(recommendations, &placestream.LiveGetRecommendations_Output_Recommendations_Elem{
					LiveGetRecommendations_LivestreamRecommendation: &placestream.LiveGetRecommendations_LivestreamRecommendation{
						Did:    did,
						Source: "streamer",
					},
				})
			}
			return &placestream.LiveGetRecommendations_Output{
				Recommendations: recommendations,
				UserDID:         &userDID,
			}, nil
		}
	} else {
		// not a big issue but we should log anyways
		log.Log(ctx, "no recommendations found for user", "userDID", userDID)
	}

	// get user's follows and check which are live
	follows, err := s.model.GetUserFollowing(ctx, userDID)
	if err == nil && len(follows) > 0 {
		followDIDs := make([]string, len(follows))
		for i, follow := range follows {
			followDIDs[i] = follow.SubjectDID
		}

		liveFollows, err := s.localDB.FilterLiveRepoDIDs(followDIDs)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to filter live follows")
		}

		if len(liveFollows) > 0 {
			var recommendations []*placestream.LiveGetRecommendations_Output_Recommendations_Elem
			for _, did := range liveFollows {
				recommendations = append(recommendations, &placestream.LiveGetRecommendations_Output_Recommendations_Elem{
					LiveGetRecommendations_LivestreamRecommendation: &placestream.LiveGetRecommendations_LivestreamRecommendation{
						Did:    did,
						Source: "follows",
					},
				})
			}
			return &placestream.LiveGetRecommendations_Output{
				Recommendations: recommendations,
				UserDID:         &userDID,
			}, nil
		}
	}

	// Final fallback: use host's default recommendations
	defaultStreamers := s.cli.DefaultRecommendedStreamers
	if len(defaultStreamers) > 0 {
		liveDefaults, err := s.localDB.FilterLiveRepoDIDs(defaultStreamers)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to filter default streamers")
		}
		var recommendations []*placestream.LiveGetRecommendations_Output_Recommendations_Elem
		for _, did := range liveDefaults {
			recommendations = append(recommendations, &placestream.LiveGetRecommendations_Output_Recommendations_Elem{
				LiveGetRecommendations_LivestreamRecommendation: &placestream.LiveGetRecommendations_LivestreamRecommendation{
					Did:    did,
					Source: "host",
				},
			})
		}
		return &placestream.LiveGetRecommendations_Output{
			Recommendations: recommendations,
			UserDID:         &userDID,
		}, nil
	}

	// No recommendations available
	return &placestream.LiveGetRecommendations_Output{
		Recommendations: []*placestream.LiveGetRecommendations_Output_Recommendations_Elem{},
		UserDID:         &userDID,
	}, nil
}

func (s *Server) handlePlaceStreamLiveStartLivestream(ctx context.Context, body *placestream.LiveStartLivestream_Input) (*placestream.LiveStartLivestream_Output, error) {
	session, client := oatproxy.GetOAuthSession(ctx)
	if session != nil {
		if session.DID != body.Streamer {
			return nil, echo.NewHTTPError(http.StatusForbidden, "you are not the streamer")
		}
	} else {
		svc := GetServiceAuth(ctx)
		if svc == nil {
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "you are not authorized")
		}
		streamerSession, err := s.statefulDB.GetSessionByDID(body.Streamer)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting streamer session", err)
		}
		if streamerSession == nil {
			return nil, echo.NewHTTPError(http.StatusNotFound, "streamer session not found")
		}
		session = streamerSession
		client, err = s.op.GetXrpcClient(streamerSession)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting streamer client", err)
		}
	}

	// proxy to the origin node if the streamer is broadcasting elsewhere
	origin, err := s.statefulDB.GetLatestBroadcastOriginForStreamer(session.DID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting broadcast origin", err)
	}
	myDID := s.cli.ServerDID()
	if origin != nil && origin.ServerDID != myDID {
		bs, err := json.Marshal(body)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "error marshalling body", err)
		}
		data, err := s.ProxyServiceRequest(ctx, origin.ServerDID, "POST", "place.stream.live.startLivestream",
			url.Values{},
			bytes.NewReader(bs), "application/json")
		if err != nil {
			return nil, err
		}
		var output placestream.LiveStartLivestream_Output
		err = json.Unmarshal(data, &output)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "error unmarshalling response", err)
		}
		return &output, nil
	}

	livestream := body.Livestream
	now := time.Now().UTC().Format(time.RFC3339)
	livestream.LexiconTypeID = "place.stream.livestream"
	livestream.CreatedAt = now
	livestream.LastSeenAt = &now

	if livestream.Thumb == nil {
		// Upload the user's current thumbnail to their PDS as the livestream image.
		var thumb *lexutil.LexBlob
		thumbData, err := os.ReadFile(s.cli.ThumbnailFilePath(session.DID))
		if err != nil {
			log.Error(ctx, "failed to read thumbnail file", "err", err)
		} else {
			var uploadOut comatproto.RepoUploadBlob_Output
			err = client.Do(ctx, xrpc.Procedure, "image/jpeg", "com.atproto.repo.uploadBlob", nil, bytes.NewReader(thumbData), &uploadOut)
			if err != nil {
				log.Error(ctx, "failed to upload thumbnail to PDS", "err", err)
			} else {
				thumb = uploadOut.Blob
			}
		}
		livestream.Thumb = thumb
	}

	// Step 3: create a Bluesky post announcing the livestream
	repo, err := s.model.GetRepo(session.DID)
	if err != nil {
		log.Error(ctx, "failed to get repo", "err", err)
	}

	handle := session.DID
	if repo != nil && repo.Handle != "" {
		handle = repo.Handle
	}

	canonicalUrl := fmt.Sprintf("https://%s/%s", s.cli.BroadcasterHost, handle)
	if livestream.CanonicalUrl != nil && *livestream.CanonicalUrl != "" {
		canonicalUrl = *livestream.CanonicalUrl
	}

	if body.CreateBlueskyPost == nil || *body.CreateBlueskyPost {
		prefix := "🔴 LIVE "
		suffix := " " + livestream.Title
		postText := prefix + canonicalUrl + suffix

		linkStart := int64(len(prefix))
		linkEnd := linkStart + int64(len(canonicalUrl))

		postRecord := &bsky.FeedPost{
			LexiconTypeID: "app.bsky.feed.post",
			Text:          postText,
			CreatedAt:     now,
			Langs:         []string{"en"},
			Facets: []*bsky.RichtextFacet{
				{
					Index: &bsky.RichtextFacet_ByteSlice{
						ByteStart: linkStart,
						ByteEnd:   linkEnd,
					},
					Features: []*bsky.RichtextFacet_Features_Elem{
						{
							RichtextFacet_Link: &bsky.RichtextFacet_Link{
								LexiconTypeID: "app.bsky.richtext.facet#link",
								Uri:           canonicalUrl,
							},
						},
					},
				},
			},
			Embed: &bsky.FeedPost_Embed{
				EmbedExternal: &bsky.EmbedExternal{
					External: &bsky.EmbedExternal_External{
						Title:       fmt.Sprintf("@%s is 🔴LIVE on %s!", handle, s.cli.BroadcasterHost),
						Uri:         canonicalUrl,
						Description: livestream.Title,
						Thumb:       livestream.Thumb,
					},
				},
			},
		}

		postInput := comatproto.RepoCreateRecord_Input{
			Collection: "app.bsky.feed.post",
			Record:     &lexutil.LexiconTypeDecoder{Val: postRecord},
			Repo:       session.DID,
		}
		var postOutput comatproto.RepoCreateRecord_Output
		err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, postInput, &postOutput)
		if err != nil {
			log.Error(ctx, "failed to create bluesky post", "err", err)
		} else {
			livestream.Post = &comatproto.RepoStrongRef{
				Uri: postOutput.Uri,
				Cid: postOutput.Cid,
			}
		}
	}

	// Step 4: create the place.stream.livestream record
	lsInput := comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.livestream",
		Record:     &lexutil.LexiconTypeDecoder{Val: livestream},
		Repo:       session.DID,
	}
	var lsOutput comatproto.RepoCreateRecord_Output
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, lsInput, &lsOutput)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create livestream record: %v", err))
	}

	return &placestream.LiveStartLivestream_Output{
		Uri: lsOutput.Uri,
		Cid: lsOutput.Cid,
	}, nil
}

func (s *Server) handlePlaceStreamLiveStopLivestream(ctx context.Context, body *placestream.LiveStopLivestream_Input) (*placestream.LiveStopLivestream_Output, error) {
	now := time.Now().UTC().Format(util.ISO8601)
	session, _ := oatproxy.GetOAuthSession(ctx)

	_, client := oatproxy.GetOAuthSession(ctx)
	if client == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required to stop livestream")
	}

	livestream, err := s.model.GetLatestLivestreamForRepo(session.DID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting livestream", err)
	}
	if livestream == nil || livestream.Livestream == nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "no active livestream for this repo")
	}

	livestreamView, err := livestream.ToLivestreamView()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error converting livestream to view", err)
	}

	livestreamRecord, ok := livestreamView.Record.Val.(*placestream.Livestream)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "livestream is not a streamplace livestream")
	}

	if livestreamRecord.EndedAt != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "livestream has already ended")
	}

	livestreamRecord.EndedAt = &now

	aturi, err := syntax.ParseATURI(livestreamView.Uri)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error parsing ATURI", err)
	}

	var swapRecord *string
	getOutput := comatproto.RepoGetRecord_Output{}
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       session.DID,
		"collection": "place.stream.livestream",
		"rkey":       aturi.RecordKey().String(),
	}, nil, &getOutput)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting livestream record", err)
	}
	swapRecord = getOutput.Cid

	lsInput := comatproto.RepoPutRecord_Input{
		Collection: "place.stream.livestream",
		Record:     &lexutil.LexiconTypeDecoder{Val: livestreamRecord},
		Rkey:       aturi.RecordKey().String(),
		Repo:       session.DID,
		SwapRecord: swapRecord,
	}
	var lsOutput comatproto.RepoPutRecord_Output
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, lsInput, &lsOutput)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error updating livestream record", err)
	}

	return &placestream.LiveStopLivestream_Output{
		Uri: lsOutput.Uri,
		Cid: lsOutput.Cid,
	}, nil
}
