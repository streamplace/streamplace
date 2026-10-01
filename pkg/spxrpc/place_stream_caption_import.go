package spxrpc

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/captions/records"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// handlePlaceStreamCaptionImportCaptions imports a WebVTT or SRT file as
// transcript records in the caller's repo, for one of the caller's videos,
// replacing the caller's earlier imported or human records of that video and
// language.
func (s *Server) handlePlaceStreamCaptionImportCaptions(ctx context.Context, body *placestream.CaptionImportCaptions_Input) (*placestream.CaptionImportCaptions_Output, error) {
	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil || client == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	ctx = log.WithLogValues(ctx, "func", "importCaptions", "did", session.DID, "video", body.Video)

	kind := ""
	if body.Kind != nil {
		kind = *body.Kind
	}
	imp := &records.Importer{Store: s.model}
	uris, err := imp.Import(ctx, client, session.DID, records.ImportInput{
		Video:    body.Video,
		Language: body.Language,
		Kind:     kind,
		Format:   body.Format,
		Body:     body.Body,
	})
	switch {
	case err == nil:
	case errors.Is(err, records.ErrNotFound):
		return nil, echo.NewHTTPError(http.StatusNotFound, "NotFound: "+err.Error())
	case errors.Is(err, records.ErrNotOwner):
		return nil, echo.NewHTTPError(http.StatusForbidden, err.Error())
	case errors.Is(err, records.ErrInvalidCaptions):
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidCaptions: "+err.Error())
	default:
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "import captions: "+err.Error())
	}
	log.Log(ctx, "imported captions", "language", body.Language, "format", body.Format, "records", len(uris))
	return &placestream.CaptionImportCaptions_Output{Uris: uris}, nil
}
