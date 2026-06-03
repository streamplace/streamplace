package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
)

// takes in and returns an aturi, doing handle resolution if necessary on the way
func (s *Server) normalizeURI(ctx context.Context, uri string) (*syntax.ATURI, error) {
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to parse AT-URI: "+err.Error())
	}

	if aturi.Authority().IsHandle() {
		handle, err := aturi.Authority().AsHandle()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to convert handle: "+err.Error())
		}
		res, err := s.resolveStreamer(ctx, handle.String())
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to resolve streamer: "+err.Error())
		}
		aturi, err = syntax.ParseATURI(fmt.Sprintf("at://%s/%s", res, aturi.Path()))
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to parse AT-URI: "+err.Error())
		}
	} else if !aturi.Authority().IsDID() {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri authority must be a DID or handle: "+uri)
	}

	return &aturi, nil
}
