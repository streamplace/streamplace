package spxrpc

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/bluesky-social/indigo/api/bsky"
	appbskytypes "github.com/bluesky-social/indigo/api/bsky"
	"stream.place/streamplace/pkg/model"
)

var FeedSkeletonRE = regexp.MustCompile(`^at://did:(web|plc):([a-z0-9\.\-]+)/app.bsky.feed.generator/([a-z0-9\.\-]+)$`)

func parseFeedSkeleton(did string) (string, string, error) {
	matches := FeedSkeletonRE.FindStringSubmatch(did)
	if len(matches) != 4 {
		return "", "", fmt.Errorf("invalid feed parameter: %s", did)
	}
	return fmt.Sprintf("did:%s:%s", matches[1], matches[2]), matches[3], nil
}

const FEED_LIVE_STREAMS = "live-streams"
const FEED_ALL_STREAMS = "all-streams"

func (s *Server) handleAppBskyFeedGetFeedSkeleton(ctx context.Context, inCursor string, feed string, limit int) (*appbskytypes.FeedGetFeedSkeleton_Output, error) {
	_, name, err := parseFeedSkeleton(feed)
	if err != nil {
		return nil, err
	}
	var ts int64
	if inCursor != "" {
		parts := strings.Split(inCursor, "::")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		ts, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
	}
	var posts []model.FeedPost
	outCursor := ""
	if name == FEED_ALL_STREAMS {
		posts, err = s.model.ListFeedPostsByType("livestream", limit, ts)
		if err != nil {
			return nil, err
		}
		if len(posts) > 0 {
			last := posts[len(posts)-1]
			ts := last.CreatedAt.UnixMilli()
			outCursor = fmt.Sprintf("%d::%s", ts, last.CID)
		}
	} else if name == FEED_LIVE_STREAMS {
		segs, err := s.model.MostRecentSegments()
		if err != nil {
			return nil, err
		}
		for _, seg := range segs {
			ls, err := s.model.GetLatestLivestreamForRepo(seg.RepoDID)
			if err != nil {
				return nil, err
			}
			posts = append(posts, model.FeedPost{
				URI: ls.PostURI,
			})
		}
	} else {
		return nil, fmt.Errorf("invalid feed name: %s", name)
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
