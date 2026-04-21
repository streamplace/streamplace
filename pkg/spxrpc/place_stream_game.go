package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	placestream "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamGameSearch(ctx context.Context, cursor string, limit int, q string) (*placestream.GameSearch_Output, error) {
	if s.cli.GamesAPIURL == "" {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "games API not configured")
	}

	if limit <= 0 {
		limit = 20
	}

	cacheKey := fmt.Sprintf("game_search:%s:%d:%s", q, limit, cursor)
	if cached, found := s.GameSearchCache.Get(cacheKey); found {
		return cached.(*placestream.GameSearch_Output), nil
	}

	params := url.Values{}
	params.Set("q", q)
	params.Set("limit", fmt.Sprintf("%d", limit))
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	reqURL := s.cli.GamesAPIURL + "/xrpc/games.gamesgamesgamesgames.search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to build games request")
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

	var out placestream.GameSearch_Output
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "failed to parse games response")
	}

	s.GameSearchCache.SetDefault(cacheKey, &out)
	return &out, nil
}
