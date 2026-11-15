package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestGameSearchForwardsTypesAndCursor(t *testing.T) {
	requests := make(chan *http.Request, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r
		fmt.Fprint(w, `{"results":[],"cursor":"next-page"}`)
	}))
	defer upstream.Close()
	s := &Server{cli: &config.CLI{GamesAPIURL: upstream.URL}, GameSearchCache: cache.New(time.Minute, time.Minute)}
	for _, types := range [][]string{{"game"}, nil} {
		out, err := s.handlePlaceStreamGameSearch(context.Background(), "page-2", 15, "zelda", types)
		require.NoError(t, err)
		require.Equal(t, "next-page", *out.Cursor)
		request := <-requests
		require.Equal(t, "page-2", request.URL.Query().Get("cursor"))
		require.Equal(t, "15", request.URL.Query().Get("limit"))
		require.Equal(t, types, request.URL.Query()["types"])
	}
}
