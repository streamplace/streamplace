package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	placestream "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamBioGetPage(ctx context.Context, repo string) (*placestream.BioPage, error) {
	bp, err := s.model.GetBioPage(ctx, repo)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to get bio page")
	}
	if bp == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Bio page not found")
	}
	return bp.ToStreamplaceBioPage()
}
