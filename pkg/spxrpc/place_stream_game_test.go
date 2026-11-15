package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestGameSearchResolvesGameLinks(t *testing.T) {
	const gameURI = "at://did:web:gamesgamesgamesgames.games/games.gamesgamesgamesgames.game/3mghjjzsvqv2d"
	for _, tc := range []struct {
		name, input, param, value string
	}{
		{"cartridge link", "https://cartridge.dev/game/cooking-simulator?ref=share#details", "slug", "cooking-simulator"},
		{"cartridge link with trailing slash", " https://cartridge.dev/game/cooking-simulator/ ", "slug", "cooking-simulator"},
		{"AT URI", "  " + gameURI + "  ", "uri", gameURI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type request struct {
				path, key, secret string
				params            url.Values
			}
			requests := make(chan request, 2)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- request{r.URL.Path, r.Header.Get("X-Client-Key"), r.Header.Get("X-Client-Secret"), r.URL.Query()}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"game":{"uri":%q,"name":"Cooking Simulator","slug":"cooking-simulator","createdAt":"2026-06-13T06:35:13Z","media":[{"mediaType":"cover","blob":{"$type":"blob","mimeType":"image/jpeg","ref":{"$link":"bafkreiclkfr6vsybhlgn3tt6dfnvwodaxhsokfgxj7vutgmnolxoymuso4"},"size":21633}}]}}`, gameURI)
			}))
			defer upstream.Close()
			s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL, GamesAPIClientKey: "client-key", GamesAPIClientSecret: "client-secret"}, GameSearchCache: cache.New(time.Minute, time.Minute)}
			for range 2 {
				out, err := s.handlePlaceStreamGameSearch(context.Background(), "", 8, tc.input)
				require.NoError(t, err)
				require.Len(t, out.Results, 1)
				game := out.Results[0].Defs_GameSummaryView
				require.NotNil(t, game)
				require.Equal(t, gameURI, game.Uri)
				require.Equal(t, "Cooking Simulator", game.Name)
				encoded, err := json.Marshal(out)
				require.NoError(t, err)
				require.Contains(t, string(encoded), `"$type":"games.gamesgamesgamesgames.defs#gameSummaryView"`)
				require.Contains(t, string(encoded), "bafkreiclkfr6vsybhlgn3tt6dfnvwodaxhsokfgxj7vutgmnolxoymuso4")
			}
			require.Len(t, requests, 1)
			got := <-requests
			require.Equal(t, "/xrpc/games.gamesgamesgamesgames.getGame", got.path)
			require.Equal(t, "client-key", got.key)
			require.Equal(t, "client-secret", got.secret)
			require.Equal(t, url.Values{tc.param: {tc.value}}, got.params)
		})
	}
}

func TestGameSearchHandlesMissingAndInvalidGames(t *testing.T) {
	requests := make(chan string, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer upstream.Close()
	s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL}, GameSearchCache: cache.New(time.Minute, time.Minute)}
	out, err := s.handlePlaceStreamGameSearch(context.Background(), "", 8, "https://cartridge.dev/game/no-such-game")
	require.NoError(t, err)
	require.Empty(t, out.Results)
	for _, input := range []string{
		"at://did:plc:abc/place.stream.video/abc",
		"at://did:plc:abc/games.gamesgamesgamesgames.game/",
		"https://cartridge.dev/user/cooking-simulator",
		"https://cartridge.dev/game/cooking%2Fsimulator",
		"https://someone@cartridge.dev/game/cooking-simulator",
	} {
		_, err = s.handlePlaceStreamGameSearch(context.Background(), "", 8, input)
		require.Error(t, err, input)
	}
	require.Len(t, requests, 1)
}

func TestGameSearchReportsLookupFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"upstream error", http.StatusServiceUnavailable, `{"error":"Unavailable"}`},
		{"invalid JSON", http.StatusOK, `not JSON`},
		{"missing game identity", http.StatusOK, `{"game":{"name":"Unknown"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer upstream.Close()
			s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL}, GameSearchCache: cache.New(time.Minute, time.Minute)}
			_, err := s.handlePlaceStreamGameSearch(context.Background(), "", 8, "https://cartridge.dev/game/example")
			require.Error(t, err)
		})
	}
}

func TestGameSearchStillSearchesNames(t *testing.T) {
	requests := make(chan *http.Request, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer upstream.Close()
	s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL}, GameSearchCache: cache.New(time.Minute, time.Minute)}
	for _, input := range []string{"metroid", "https://cartridge.dev.example/game/cooking-simulator"} {
		out, err := s.handlePlaceStreamGameSearch(context.Background(), "", 8, input)
		require.NoError(t, err)
		require.Empty(t, out.Results)
		request := <-requests
		require.Equal(t, "/xrpc/games.gamesgamesgamesgames.search", request.URL.Path)
		require.Equal(t, input, request.URL.Query().Get("q"))
		require.Empty(t, request.URL.Query().Get("types"))
	}
}

func TestGameSearchFromIGDBLinks(t *testing.T) {
	requests := make(chan *http.Request, 3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer upstream.Close()
	s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL}, GameSearchCache: cache.New(time.Minute, time.Minute)}
	for _, tc := range []struct{ input, query string }{
		{"https://www.igdb.com/games/cooking-simulator?ref=share", "cooking simulator"},
		{" https://igdb.com/games/resident-evil-4--1/ ", "resident evil 4"},
	} {
		_, err := s.handlePlaceStreamGameSearch(context.Background(), "", 8, tc.input)
		require.NoError(t, err)
		request := <-requests
		require.Equal(t, "/xrpc/games.gamesgamesgamesgames.search", request.URL.Path)
		require.Equal(t, tc.query, request.URL.Query().Get("q"))
	}
	for _, input := range []string{"https://igdb.com/games/", "https://igdb.com/games/a%2Fb", "https://person@igdb.com/games/test"} {
		_, err := s.handlePlaceStreamGameSearch(context.Background(), "", 8, input)
		require.Error(t, err, input)
	}
	require.Empty(t, requests)
}
