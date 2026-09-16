package spxrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/statedb"
)

// Both icon URLs the page links serve the branded favicon once one is set;
// before that, each serves its bundled file.
func TestFaviconRoutesServeBranding(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{BroadcasterHost: "example.com"}, statefulDB: sdb}
	serve := func(h echo.HandlerFunc, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, path, nil), rec)
		require.NoError(t, h(c))
		return rec
	}

	png := serve(s.HandleFaviconPNG, "/favicon.png")
	require.Equal(t, http.StatusOK, png.Code)
	require.Equal(t, "image/png", png.Header().Get("Content-Type"))
	require.NotEmpty(t, png.Body.Bytes(), "bundled favicon.png")
	ico := serve(s.HandleFaviconICO, "/favicon.ico")
	require.Equal(t, "image/x-icon", ico.Header().Get("Content-Type"))

	branded := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "favicon", "image/png", branded, nil, nil))
	for _, tc := range []struct {
		h    echo.HandlerFunc
		path string
	}{{s.HandleFaviconPNG, "/favicon.png"}, {s.HandleFaviconICO, "/favicon.ico"}} {
		rec := serve(tc.h, tc.path)
		require.Equal(t, http.StatusOK, rec.Code, tc.path)
		require.Equal(t, branded, rec.Body.Bytes(), "%s serves the branded icon", tc.path)
		require.Equal(t, "image/png", rec.Header().Get("Content-Type"), tc.path)
		require.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"), tc.path)
	}
}
