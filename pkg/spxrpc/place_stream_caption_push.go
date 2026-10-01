package spxrpc

// Integration placeholder owned by the CaptionSources slice; replaced before commit.

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamCaptionPushCaptions(ctx context.Context, body *placestream.CaptionPushCaptions_Input) (*placestream.CaptionPushCaptions_Output, error) {
	return nil, echo.NewHTTPError(http.StatusNotImplemented, "pushCaptions not implemented yet")
}
