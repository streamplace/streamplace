package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	placestream "stream.place/streamplace/pkg/placestream"
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

	var out placestream.GameSearch_Output
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "failed to parse games response")
	}

	s.GameSearchCache.SetDefault(cacheKey, &out)
	return &out, nil
}

func (s *Server) handlePlaceStreamGameGetGame(ctx context.Context, uri string) (*placestream.GameGetGame_Output, error) {
	if s.cli.GamesAPIURL == "" {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "games API not configured")
	}

	cacheKey := "game:" + uri
	if cached, found := s.GameSearchCache.Get(cacheKey); found {
		return cached.(*placestream.GameGetGame_Output), nil
	}

	// Parse AT URI: at://authority/collection/rkey
	withoutPrefix := strings.TrimPrefix(uri, "at://")
	parts := strings.SplitN(withoutPrefix, "/", 3)
	if len(parts) != 3 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid AT URI")
	}
	did, collection, rkey := parts[0], parts[1], parts[2]

	// resolve the DID to get the PDS
	did, svc, _, err := resolveRepoService(ctx, did)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to resolve DID")
	}

	params := url.Values{}
	params.Set("repo", did)
	params.Set("collection", collection)
	params.Set("rkey", rkey)
	reqURL := svc + "/xrpc/com.atproto.repo.getRecord?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to build request")
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
		return nil, echo.NewHTTPError(http.StatusBadGateway, "failed to read response")
	}

	var record struct {
		Value struct {
			Name    string   `json:"name"`
			Summary string   `json:"summary"`
			Genres  []string `json:"genres"`
			Media   []struct {
				MediaType string `json:"mediaType"`
				Blob      struct {
					Ref struct {
						Link string `json:"$link"`
					} `json:"ref"`
				} `json:"blob"`
			} `json:"media"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &record); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "failed to parse response")
	}

	var coverUrl *string
	for _, m := range record.Value.Media {
		if (m.MediaType == "cover" || m.MediaType == "coverSquare") && m.Blob.Ref.Link != "" {
			u := "https://cdn.bsky.app/img/feed_thumbnail/plain/" + did + "/" + m.Blob.Ref.Link + "@jpeg"
			coverUrl = &u
			break
		}
	}

	out := &placestream.GameGetGame_Output{
		Uri:      uri,
		Name:     record.Value.Name,
		Summary:  &record.Value.Summary,
		Genres:   record.Value.Genres,
		CoverUrl: coverUrl,
	}

	s.GameSearchCache.SetDefault(cacheKey, out)
	return out, nil
}
