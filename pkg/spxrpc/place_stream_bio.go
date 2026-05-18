package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/atproto"
	placestream "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamBioGetPage(ctx context.Context, repo string) (*placestream.BioPage, error) {
	accountLabels, err := s.model.GetActiveLabels(repo)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to get bio page")
	}
	if atproto.IsBanned(accountLabels...) {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Bio page not found")
	}

	recordLabels, err := s.model.GetActiveLabels(fmt.Sprintf("at://%s/place.stream.bio.page/self", repo))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to get bio page")
	}
	if atproto.IsRecordHidden(recordLabels...) {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Bio page not found")
	}

	bp, err := s.model.GetBioPage(ctx, repo)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to get bio page")
	}
	if bp == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Bio page not found")
	}
	return bp.ToStreamplaceBioPage()
}
