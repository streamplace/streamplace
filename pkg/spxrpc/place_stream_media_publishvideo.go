package spxrpc

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	placestream "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/vod"
)

// handlePlaceStreamMediaPublishVideo creates a place.stream.video record in
// the authenticated user's repo for a finished upload. The client hands us
// the record it would otherwise putRecord itself; the server overrides the
// authoritative fields (source tracks + durationMs) from the processed
// upload and backfills a generated thumbnail when the record carries none.
func (s *Server) handlePlaceStreamMediaPublishVideo(ctx context.Context, body *placestream.MediaPublishVideo_Input) (*placestream.MediaPublishVideo_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	if body.UploadId == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uploadId is required")
	}
	if body.Record == nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "record is required")
	}
	if s.playbackStore == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}

	uri, cid, err := vod.PublishVideo(ctx, s.statefulDB, s.playbackStore, session.DID, body.UploadId, body.Record)
	if err != nil {
		switch {
		case errors.Is(err, vod.ErrUploadNotFound):
			return nil, echo.NewHTTPError(http.StatusNotFound, "upload not found")
		case errors.Is(err, vod.ErrUploadNotReady):
			return nil, echo.NewHTTPError(http.StatusConflict, "upload is still processing")
		default:
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	return &placestream.MediaPublishVideo_Output{Uri: uri, Cid: cid}, nil
}
