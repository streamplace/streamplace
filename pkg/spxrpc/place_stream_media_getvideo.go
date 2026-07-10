package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	placestream "stream.place/streamplace/pkg/placestream"
)

// handlePlaceStreamMediaGetVideo serves a hydrated view of a video
// record — the record itself, its author (DID + handle), and the
// summed place.stream.media.viewCount totals across every reporting
// node we've indexed. Pure delegate: all hydration logic lives in
// model.GetVideoView so the handler stays a one-liner and the rest
// of the app can ignore pkg/model entirely.
func (s *Server) handlePlaceStreamMediaGetVideo(ctx context.Context, uri string) (*placestream.MediaGetVideo_VideoView, error) {
	if uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	aturi, err := s.normalizeURI(ctx, uri)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri must be a valid AT-URI: "+err.Error())
	}
	view, err := s.model.GetVideoView(ctx, aturi.String())
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if false {
		return nil, echo.NewHTTPError(http.StatusNotFound, "VideoNotFound")
	}
	return &view, nil
}
