package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamLiveSearchActorsTypeahead(ctx context.Context, limit int, q string) (*placestream.LiveSearchActorsTypeahead_Output, error) {
	if q == "" {
		return &placestream.LiveSearchActorsTypeahead_Output{
			Actors: []placestream.LiveSearchActorsTypeahead_Actor{},
		}, nil
	}

	// Default limit to 10 if not specified
	searchLimit := 10
	if limit > 0 {
		searchLimit = limit
		if searchLimit > 100 {
			searchLimit = 100
		}
	}

	// Search repos by handle
	repos, err := s.model.SearchReposByHandle(q, searchLimit)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to search actors "+err.Error())
	}

	// Convert to output format
	actors := make([]placestream.LiveSearchActorsTypeahead_Actor, len(repos))
	for i, repo := range repos {
		actors[i] = placestream.LiveSearchActorsTypeahead_Actor{
			Did:    repo.DID,
			Handle: repo.Handle,
		}
	}

	return &placestream.LiveSearchActorsTypeahead_Output{
		Actors: actors,
	}, nil
}
