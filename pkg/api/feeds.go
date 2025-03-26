package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

var DidJson = map[string]any{
	"@context": []string{
		"https://www.w3.org/ns/did/v1",
	},
	"id": "did:web:fairway.iameli.link",
	"service": []map[string]any{
		{
			"id":              "#bsky_fg",
			"type":            "BskyFeedGenerator",
			"serviceEndpoint": "https://fairway.iameli.link",
		},
	},
}

func (a *StreamplaceAPI) HandleDidJson(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		bs, err := json.Marshal(DidJson)
		if err != nil {
			log.Error(ctx, "could not marshal did json", "error", err)
			return
		}
		w.Write(bs)
	}
}

var FeedSkeletonRE = regexp.MustCompile(`^at://did:(web|plc):([a-z0-9\.\-]+)/app.bsky.feed.generator/([a-z0-9\.\-]+)$`)

func parseFeedSkeleton(did string) (string, string, error) {
	matches := FeedSkeletonRE.FindStringSubmatch(did)
	log.Warn(context.TODO(), "matches", "matches", matches)
	if len(matches) != 4 {
		return "", "", fmt.Errorf("invalid feed parameter: %s", did)
	}
	return fmt.Sprintf("did:%s:%s", matches[1], matches[2]), matches[3], nil
}

// const FEED_LIVE_STREAMS = "live-streams"
const FEED_ALL_STREAMS = "all-streams"

func (a *StreamplaceAPI) HandleXRPCAppBskyFeedGetFeedSkeleton(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")

		feed := req.URL.Query().Get("feed")
		if feed == "" {
			errors.WriteHTTPBadRequest(w, "feed is required", nil)
			return
		}
		_, name, err := parseFeedSkeleton(feed)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "invalid feed parameter", err)
			return
		}
		limit := 50
		limitStr := req.URL.Query().Get("limit")
		if limitStr != "" {
			limit, err = strconv.Atoi(limitStr)
			if err != nil {
				errors.WriteHTTPBadRequest(w, "invalid limit", nil)
				return
			}
			if limit < 1 || limit > 100 {
				errors.WriteHTTPBadRequest(w, "invalid limit (<1 or >100)", nil)
				return
			}
		}
		var posts []model.FeedPost
		if name == FEED_ALL_STREAMS {
			posts, err = a.Model.ListFeedPostsByType("livestream", limit)
			if err != nil {
				errors.WriteHTTPInternalServerError(w, "error listing feed posts", err)
				return
			}
		} else {
			errors.WriteHTTPBadRequest(w, "invalid feed name", nil)
			return
		}
		res := bsky.FeedGetFeedSkeleton_Output{
			Feed: []*bsky.FeedDefs_SkeletonFeedPost{},
		}
		for _, post := range posts {
			res.Feed = append(res.Feed, &bsky.FeedDefs_SkeletonFeedPost{
				Post: post.URI,
			})
		}
		bs, err := json.Marshal(res)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "error marshalling feed skeleton", err)
			return
		}
		w.Write(bs)
	}
}
