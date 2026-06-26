package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	placestream "stream.place/streamplace/pkg/streamplace"
)

// errNotImplemented is returned by the draft XRPC stubs until Phase 3 lands.
var errNotImplemented = echo.NewHTTPError(http.StatusNotImplemented, "draft VOD endpoint not yet implemented")

// Stubs for the place.stream.vod.* draft XRPCs. Full implementations land in
// Phase 3 once the statedb draft storage layer exists; these satisfy the
// generated server stubs (stubs.go) so the tree compiles in the meantime.

func (s *Server) handlePlaceStreamVodListDrafts(ctx context.Context, cursor string, limit int) (*placestream.VodListDrafts_Output, error) {
	return nil, errNotImplemented
}

func (s *Server) handlePlaceStreamVodGetDraft(ctx context.Context, uri string) (*placestream.VodGetDraft_Output, error) {
	return nil, errNotImplemented
}

func (s *Server) handlePlaceStreamVodUpdateDraft(ctx context.Context, body *placestream.VodUpdateDraft_Input) (*placestream.VodUpdateDraft_Output, error) {
	return nil, errNotImplemented
}

func (s *Server) handlePlaceStreamVodDeleteDraft(ctx context.Context, body *placestream.VodDeleteDraft_Input) error {
	return errNotImplemented
}

func (s *Server) handlePlaceStreamVodPublishDraft(ctx context.Context, body *placestream.VodPublishDraft_Input) (*placestream.VodPublishDraft_Output, error) {
	return nil, errNotImplemented
}
