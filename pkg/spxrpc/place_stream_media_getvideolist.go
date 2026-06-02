package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	placestream "stream.place/streamplace/pkg/streamplace"
)

// handlePlaceStreamMediaGetVideoList returns a paginated, hydrated
// list of video records. When repo is supplied (DID or handle) the
// list is scoped to that repo; when it's empty the list spans every
// indexed repo, newest first.
func (s *Server) handlePlaceStreamMediaGetVideoList(ctx context.Context, cursor string, limit int, repo string) (*placestream.MediaGetVideoList_Output, error) {
	var repoDID string
	if repo != "" {
		// repo may be a DID or a handle; resolve handles to DIDs so the
		// model only ever sees DIDs.
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
	out, err := s.model.GetVideoList(ctx, repoDID, l, cursor, s.cli.ServerDID())
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return out, nil
}
