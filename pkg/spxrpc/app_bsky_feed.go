package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

var FeedSkeletonRE = regexp.MustCompile(`^at://did:(web|plc):([a-z0-9\.\-]+)/app.bsky.feed.generator/([a-z0-9\.\-]+)$`)

func parseFeedSkeleton(did string) (string, string, error) {
	matches := FeedSkeletonRE.FindStringSubmatch(did)
	if len(matches) != 4 {
		return "", "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid feed parameter: %s", did))
	}
	return fmt.Sprintf("did:%s:%s", matches[1], matches[2]), matches[3], nil
}

const FeedLiveStreams = "live-streams"
const FeedAllStreams = "all-streams"

func (s *Server) handleAppBskyFeedGetFeedSkeleton(ctx context.Context, inCursor string, feed string, limit int) (*bsky.FeedGetFeedSkeleton_Output, error) {
	_, name, err := parseFeedSkeleton(feed)
	if err != nil {
		return nil, err
	}
	var ts int64
	if inCursor != "" {
		parts := strings.Split(inCursor, "::")
		if len(parts) != 2 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cursor format")
		}
		ts, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cursor timestamp")
		}
	}
	var posts []model.FeedPost
	outCursor := ""
	if name == FeedAllStreams {
		posts, err = s.model.ListFeedPostsByType("livestream", limit, ts)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to list feed posts: %v", err))
		}
		if len(posts) > 0 {
			last := posts[len(posts)-1]
			ts := last.CreatedAt.UnixMilli()
			outCursor = fmt.Sprintf("%d::%s", ts, last.CID)
		}
	} else if name == FeedLiveStreams {
		segs, err := s.model.MostRecentSegments()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get recent segments: %v", err))
		}
		for _, seg := range segs {
			ls, err := s.model.GetLatestLivestreamForRepo(seg.RepoDID)
			if err != nil {
				log.Error(ctx, "failed to get latest livestream, skipping", "repoDID", seg.RepoDID, "error", err)
				continue
			}
			posts = append(posts, model.FeedPost{
				URI: ls.PostURI,
			})
		}
	} else {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid feed name: %s", name))
	}
	res := bsky.FeedGetFeedSkeleton_Output{
		Feed: []*bsky.FeedDefs_SkeletonFeedPost{},
	}
	if outCursor != "" {
		res.Cursor = &outCursor
	}
	for _, post := range posts {
		res.Feed = append(res.Feed, &bsky.FeedDefs_SkeletonFeedPost{
			Post: post.URI,
		})
	}
	return &res, nil
}
