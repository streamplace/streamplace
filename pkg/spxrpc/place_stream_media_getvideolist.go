package spxrpc

import (
	"context"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"

	placestream "stream.place/streamplace/pkg/streamplace"
)

// handlePlaceStreamMediaGetVideoList returns a paginated, hydrated
// list of video records for a given repo DID.
func (s *Server) handlePlaceStreamMediaGetVideoList(ctx context.Context, cursor string, limit int, repo string) (*placestream.MediaGetVideoList_Output, error) {
	if repo == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "repo is required")
	}
	if _, err := syntax.ParseDID(repo); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "repo must be a valid DID: "+err.Error())
	}

	l := 25
	if limit > 0 && limit <= 100 {
		l = limit
	}

	out, err := s.model.GetVideoList(ctx, repo, l, cursor)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return out, nil
}
