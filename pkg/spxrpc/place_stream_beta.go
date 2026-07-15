package spxrpc

import (
	"context"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"

	placestream "stream.place/streamplace/pkg/placestream"
)

// handlePlaceStreamBetaGetStatus reports an account's access status for a
// named beta feature: "granted", "requested", or "none". The subject defaults
// to the authenticated caller when `did` is omitted, so the common case ("am
// *I* allowed?") needs no parameter beyond the feature. The status is computed
// by the same gate that createUpload enforces, so the UI can never claim
// access the upload path would reject.
func (s *Server) handlePlaceStreamBetaGetStatus(ctx context.Context, did, feature string) (*placestream.BetaGetStatus_Output, error) {
	if feature == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "feature is required")
	}

	subject := did
	if subject == "" {
		session, _ := oatproxy.GetOAuthSession(ctx)
		if session == nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "DidRequired")
		}
		subject = session.DID
	}
	if _, err := syntax.ParseDID(subject); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "did must be a valid DID: "+err.Error())
	}

	status, err := s.betaFeatureStatus(ctx, subject, feature)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return &placestream.BetaGetStatus_Output{
		Did:     subject,
		Feature: feature,
		Status:  status,
	}, nil
}
