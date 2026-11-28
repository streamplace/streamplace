package spxrpc

import (
	"context"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"

	placestream "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/vod"
)

// handlePlaceStreamMediaGetVideoList returns a paginated, hydrated
// list of video records. When repo is supplied (DID or handle) the
// list is scoped to that repo; when it's empty the list spans every
// indexed repo, newest first.
func (s *Server) handlePlaceStreamMediaGetVideoList(ctx context.Context, cursor string, limit int, repo string) (*placestream.MediaGetVideoList_Output, error) {
	var repoDID string
	if repo != "" && s.cli.StationDID != "" {
		if alias, ok := s.aliases[repo]; ok {
			repo = alias
		}
		if did, err := syntax.ParseDID(repo); err == nil {
			repoDID = did.String()
		} else {
			account, err := s.model.GetRepoByHandle(repo)
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			if account == nil {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "repo is not locally indexed")
			}
			repoDID = account.DID
		}
	} else if repo != "" {
		resolved, err := s.resolveStreamer(ctx, repo)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "could not resolve repo: "+err.Error())
		}
		repoDID = resolved
	}

	l := 25
	if limit > 0 && limit <= 100 {
		l = limit
	}

	// Only advertise videos this node can actually serve — i.e. whose
	// content blob we host (have a place.stream.media.origin for). A video
	// we've only indexed from the firehose, with no local blob, isn't
	// playable here and shouldn't appear in our listing.
	var out placestream.MediaGetVideoList_Output
	var err error
	if s.cli.StationDID != "" {
		out, err = s.model.GetStationVideoList(ctx, repoDID, l, cursor, func(ctx context.Context, uri, cid string) (bool, error) {
			return vod.StationVideoVisible(ctx, s.playbackStore, s.model, s.cli.StationDID, uri, cid)
		})
	} else {
		out, err = s.model.GetVideoList(ctx, repoDID, l, cursor, s.cli.ServerDID())
	}
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	for i := range out.Videos {
		s.addLivestreamViews(ctx, &out.Videos[i])
	}
	return &out, nil
}
