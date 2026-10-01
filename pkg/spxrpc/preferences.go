package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamServerGetPreferences(ctx context.Context) (*placestream.ServerGetPreferences_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	prefs, err := s.statefulDB.GetUserPreferences(ctx, session.DID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "get preferences: "+err.Error())
	}
	return &placestream.ServerGetPreferences_Output{Preferences: prefs.ToLexicon()}, nil
}

func (s *Server) handlePlaceStreamServerPutPreferences(ctx context.Context, input *placestream.ServerPutPreferences_Input) (*placestream.ServerPutPreferences_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	prefs, err := s.statefulDB.GetUserPreferences(ctx, session.DID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "get preferences: "+err.Error())
	}
	if input.AutoPublishVods != nil {
		prefs.AutoPublishVODs = *input.AutoPublishVods
	}
	if err := s.statefulDB.PutUserPreferences(ctx, prefs); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "save preferences: "+err.Error())
	}
	log.Log(ctx, "preferences updated", "did", session.DID, "autoPublishVods", prefs.AutoPublishVODs)
	return &placestream.ServerPutPreferences_Output{Preferences: prefs.ToLexicon()}, nil
}
