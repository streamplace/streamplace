package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestGameGetGameDoesNotUseUnindexedPDSRecord(t *testing.T) {
	const did = "did:plc:ho26ynsdw2ey7l56xx4owjzd"
	const uri = "at://" + did + "/games.gamesgamesgamesgames.game/example"
	paths := make(chan string, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		switch r.URL.Path {
		case "/xrpc/games.gamesgamesgamesgames.getGame":
			fmt.Fprint(w, `{}`)
		case "/xrpc/com.atproto.repo.getRecord":
			fmt.Fprint(w, `{"value":{"name":"Unindexed Game","genres":["adventure"]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{DID: syntax.DID(did), Services: map[string]identity.ServiceEndpoint{
		"atproto_pds": {Type: "AtprotoPersonalDataServer", URL: upstream.URL},
	}})
	s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL, GamesAPIClientSecret: "secret"}, GameSearchCache: cache.New(time.Minute, time.Minute), identityDirectory: func() identity.Directory { return dir }}
	out, err := s.handlePlaceStreamGameGetGame(context.Background(), "", uri)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusNotFound, httpErr.Code)
	require.Nil(t, out)
	require.Len(t, paths, 1)
	require.Equal(t, "/xrpc/games.gamesgamesgamesgames.getGame", <-paths)
}

func TestGameGetGameUsesCatalog(t *testing.T) {
	const gameURI = "at://did:web:gamesgamesgamesgames.games/games.gamesgamesgamesgames.game/3mghjjzsvqv2d"
	for _, tc := range []struct{ name, slug, uri, param, value string }{
		{"slug", "cooking-simulator", "", "slug", "cooking-simulator"},
		{"AT URI", "", gameURI, "uri", gameURI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan *http.Request, 2)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r
				fmt.Fprintf(w, `{"game":{"uri":%q,"name":"Cooking Simulator","createdAt":"2026-06-13T06:35:13Z","genres":["simulation"],"media":[{"mediaType":"cover","blob":{"$type":"blob","mimeType":"image/jpeg","ref":{"$link":"bafkreiclkfr6vsybhlgn3tt6dfnvwodaxhsokfgxj7vutgmnolxoymuso4"},"size":21633}}]}}`, gameURI)
			}))
			defer upstream.Close()
			s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL, GamesAPIClientKey: "client-key", GamesAPIClientSecret: "client-secret"}, GameSearchCache: cache.New(time.Minute, time.Minute)}
			for range 2 {
				out, err := s.handlePlaceStreamGameGetGame(context.Background(), tc.slug, tc.uri)
				require.NoError(t, err)
				require.Equal(t, gameURI, out.Uri)
				require.Equal(t, "Cooking Simulator", out.Name)
				require.Equal(t, []string{"simulation"}, out.Genres)
				require.Contains(t, *out.CoverUrl, "bafkreiclkfr6vsybhlgn3tt6dfnvwodaxhsokfgxj7vutgmnolxoymuso4")
			}
			require.Len(t, requests, 1)
			got := <-requests
			require.Equal(t, "/xrpc/games.gamesgamesgamesgames.getGame", got.URL.Path)
			require.Equal(t, url.Values{tc.param: {tc.value}}, got.URL.Query())
			require.Equal(t, "client-key", got.Header.Get("X-Client-Key"))
			require.Equal(t, "client-secret", got.Header.Get("X-Client-Secret"))
		})
	}
}

func TestGameGetGameUsesFirstMediaWhenCoverMissing(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"game":{"uri":"at://did:web:gamesgamesgamesgames.games/games.gamesgamesgamesgames.game/3mghjjzsvqv2d","name":"Game","media":[{"mediaType":"screenshot","blob":{"$type":"blob","mimeType":"image/jpeg","ref":{"$link":"bafkreiclkfr6vsybhlgn3tt6dfnvwodaxhsokfgxj7vutgmnolxoymuso4"},"size":21633}}]}}`)
	}))
	defer upstream.Close()
	s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL}, GameSearchCache: cache.New(time.Minute, time.Minute)}
	out, err := s.handlePlaceStreamGameGetGame(context.Background(), "game", "")
	require.NoError(t, err)
	require.NotNil(t, out.CoverUrl)
	require.Contains(t, *out.CoverUrl, "bafkreiclkfr6vsybhlgn3tt6dfnvwodaxhsokfgxj7vutgmnolxoymuso4")
}

func TestGameGetGameHandlesMissingAndInvalidGames(t *testing.T) {
	requests := make(chan string, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer upstream.Close()
	s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL}, GameSearchCache: cache.New(time.Minute, time.Minute)}
	out, err := s.handlePlaceStreamGameGetGame(context.Background(), "no-such-game", "")
	require.Error(t, err)
	require.Nil(t, out)
	for _, input := range []string{
		"at://did:plc:abc/place.stream.video/abc",
		"at://did:plc:abc/games.gamesgamesgamesgames.game/",
	} {
		_, err = s.handlePlaceStreamGameGetGame(context.Background(), "", input)
		require.Error(t, err, input)
	}
	_, err = s.handlePlaceStreamGameGetGame(context.Background(), "", "")
	require.Error(t, err)
	_, err = s.handlePlaceStreamGameGetGame(context.Background(), "game", "at://did:plc:abc/games.gamesgamesgamesgames.game/abc")
	require.Error(t, err)
	require.Len(t, requests, 1)
}

func TestGameGetGameReportsLookupFailures(t *testing.T) {
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
			_, err := s.handlePlaceStreamGameGetGame(context.Background(), "example", "")
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
