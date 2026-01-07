package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/lex/util"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/spmetrics"

	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

var replicationUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 1024 * 10, // 10MB
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handlePlaceStreamLiveGetSegments(ctx context.Context, before string, limit int, userDID string) (*placestreamtypes.LiveGetSegments_Output, error) {
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

	segments, err := s.model.LatestSegmentsForUser(userDID, limit, beforeTime, nil)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch segments")
	}

	// Convert segments to the expected output format
	output := &placestreamtypes.LiveGetSegments_Output{
		Segments: make([]*placestreamtypes.Segment_SegmentView, len(segments)),
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
		ltd := &util.LexiconTypeDecoder{Val: record}

		output.Segments[i] = &placestreamtypes.Segment_SegmentView{
			Record: ltd,
			Cid:    c.String(),
		}
	}

	return output, nil
}

func (s *Server) handlePlaceStreamLiveGetLiveUsers(ctx context.Context, before string, limit int) (*placestreamtypes.LiveGetLiveUsers_Output, error) {
	var beforeTime *time.Time
	if before != "" {
		parsedTime, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid 'before' parameter: must be RFC3339 format")
		}
		beforeTime = &parsedTime
	}
	ls, err := s.model.GetLatestLivestreams(limit, beforeTime)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch livestreams")
	}

	streams := make([]*placestreamtypes.Livestream_LivestreamView, len(ls))

	for i, l := range ls {
		stream, err := l.ToLivestreamView()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to convert livestream to streamplace livestream: %s", err))
		}
		viewers := spmetrics.GetViewCount(stream.Author.Did)
		stream.ViewerCount = &placestreamtypes.Livestream_ViewerCount{
			LexiconTypeID: "place.stream.livestream#viewerCount",
			Count:         int64(viewers),
		}
		streams[i] = stream
	}

	liveUsers := &placestreamtypes.LiveGetLiveUsers_Output{
		Streams: streams,
	}

	return liveUsers, nil
}

func (s *Server) handlePlaceStreamLiveSubscribeSegments(c echo.Context) error {
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

func (s *Server) handlePlaceStreamLiveGetRecommendations(ctx context.Context, userDID string) (*placestreamtypes.LiveGetRecommendations_Output, error) {
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
		liveStreamers, err := s.model.FilterLiveRepoDIDs(streamers)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to filter live streamers")
		}

		if len(liveStreamers) > 0 {
			var recommendations []*placestreamtypes.LiveGetRecommendations_Output_Recommendations_Elem
			for _, did := range liveStreamers {
				recommendations = append(recommendations, &placestreamtypes.LiveGetRecommendations_Output_Recommendations_Elem{
					LiveGetRecommendations_LivestreamRecommendation: &placestreamtypes.LiveGetRecommendations_LivestreamRecommendation{
						Did:    did,
						Source: "streamer",
					},
				})
			}
			return &placestreamtypes.LiveGetRecommendations_Output{
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

		liveFollows, err := s.model.FilterLiveRepoDIDs(followDIDs)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to filter live follows")
		}

		if len(liveFollows) > 0 {
			var recommendations []*placestreamtypes.LiveGetRecommendations_Output_Recommendations_Elem
			for _, did := range liveFollows {
				recommendations = append(recommendations, &placestreamtypes.LiveGetRecommendations_Output_Recommendations_Elem{
					LiveGetRecommendations_LivestreamRecommendation: &placestreamtypes.LiveGetRecommendations_LivestreamRecommendation{
						Did:    did,
						Source: "follows",
					},
				})
			}
			return &placestreamtypes.LiveGetRecommendations_Output{
				Recommendations: recommendations,
				UserDID:         &userDID,
			}, nil
		}
	}

	// Final fallback: use host's default recommendations
	defaultStreamers := s.cli.DefaultRecommendedStreamers
	if len(defaultStreamers) > 0 {
		liveDefaults, err := s.model.FilterLiveRepoDIDs(defaultStreamers)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to filter default streamers")
		}
		var recommendations []*placestreamtypes.LiveGetRecommendations_Output_Recommendations_Elem
		for _, did := range liveDefaults {
			recommendations = append(recommendations, &placestreamtypes.LiveGetRecommendations_Output_Recommendations_Elem{
				LiveGetRecommendations_LivestreamRecommendation: &placestreamtypes.LiveGetRecommendations_LivestreamRecommendation{
					Did:    did,
					Source: "host",
				},
			})
		}
		return &placestreamtypes.LiveGetRecommendations_Output{
			Recommendations: recommendations,
			UserDID:         &userDID,
		}, nil
	}

	// No recommendations available
	return &placestreamtypes.LiveGetRecommendations_Output{
		Recommendations: []*placestreamtypes.LiveGetRecommendations_Output_Recommendations_Elem{},
		UserDID:         &userDID,
	}, nil
}
