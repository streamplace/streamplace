package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	gamesgamesgamesgamesgames "stream.place/streamplace/pkg/gamesgamesgamesgamesgames"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamGameSearch(ctx context.Context, cursor string, limit int, q string) (*placestream.GameSearch_Output, error) {
	if limit <= 0 {
		limit = 20
	}
	cacheKey := fmt.Sprintf("game_search:%s:%d:%s", q, limit, cursor)
	if cached, found := s.GameSearchCache.Get(cacheKey); found {
		if out, ok := cached.(*placestream.GameSearch_Output); ok {
			return out, nil
		}
	}

	params := url.Values{"q": {q}, "limit": {fmt.Sprintf("%d", limit)}}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	body, err := s.requestGames(ctx, "/xrpc/games.gamesgamesgamesgames.search", params)
	if err != nil {
		return nil, err
	}
	var out placestream.GameSearch_Output
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "failed to parse games response")
	}
	s.GameSearchCache.SetDefault(cacheKey, &out)
	return &out, nil
}

func (s *Server) handlePlaceStreamGameGetGame(ctx context.Context, slug string, uri string) (*placestream.GameGetGame_Output, error) {
	if (slug == "") == (uri == "") {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "provide exactly one game slug or AT URI")
	}
	params := url.Values{}
	if uri != "" {
		parsed, err := syntax.ParseATURI(uri)
		if err != nil || parsed.Collection().String() != "games.gamesgamesgamesgames.game" || parsed.RecordKey().String() == "" {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid game AT URI")
		}
		params.Set("uri", uri)
	} else {
		params.Set("slug", slug)
	}
	cacheKey := "game_lookup:" + params.Encode()
	if cached, found := s.GameSearchCache.Get(cacheKey); found {
		if out, ok := cached.(*placestream.GameGetGame_Output); ok {
			return out, nil
		}
	}

	body, err := s.requestGames(ctx, "/xrpc/games.gamesgamesgamesgames.getGame", params)
	if err != nil {
		return nil, err
	}
	var response struct {
		Game *gamesgamesgamesgamesgames.Defs_GameDetailView `json:"game"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "failed to parse games response")
	}
	if response.Game == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "game not found")
	}
	game := response.Game
	parsed, err := syntax.ParseATURI(game.Uri)
	if err != nil || parsed.Collection().String() != "games.gamesgamesgamesgames.game" || parsed.RecordKey().String() == "" || game.Name == "" {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "invalid game in games response")
	}
	out := &placestream.GameGetGame_Output{Uri: game.Uri, Name: game.Name, Summary: game.Summary, Genres: game.Genres}
	var cover *gamesgamesgamesgamesgames.Defs_MediaItem
	if len(game.Media) > 0 {
		cover = &game.Media[0]
	}
	for i := range game.Media {
		media := &game.Media[i]
		if media.MediaType != nil && (*media.MediaType == "cover" || *media.MediaType == "coverSquare") {
			cover = media
			break
		}
	}
	if cover != nil && cover.Blob != nil {
		coverURL := "https://cdn.bsky.app/img/feed_thumbnail/plain/" + parsed.Authority().String() + "/" + cover.Blob.Ref.String() + "@jpeg"
		out.CoverUrl = &coverURL
	}
	s.GameSearchCache.SetDefault(cacheKey, out)
	return out, nil
}

func (s *Server) requestGames(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if s.cli.GamesAPIURL == "" {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "games API not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cli.GamesAPIURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to build games request")
	}
	if s.cli.GamesAPIClientKey != "" {
		req.Header.Set("X-Client-Key", s.cli.GamesAPIClientKey)
	}
	if s.cli.GamesAPIClientSecret != "" {
		req.Header.Set("X-Client-Secret", s.cli.GamesAPIClientSecret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "games API unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("games API returned %d", resp.StatusCode))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "failed to read games response")
	}
	return body, nil
}

func (s *Server) handleGamesGamesgamesgamesgamesSearch(ctx context.Context, ageRatings []string, applicationTypes []string, cursor string, genres []string, includeCancelled bool, includeUnrated bool, limit int, modes []string, playerPerspectives []string, q string, sort string, themes []string, types []string) (*gamesgamesgamesgamesgames.Search_Output, error) {
	// TODO: implement full games search with all filter params
	// TODO: convert GameSearch_Output to gamesgamesgamesgamesgames.Search_Output
	return nil, fmt.Errorf("games search not yet implemented")
}
