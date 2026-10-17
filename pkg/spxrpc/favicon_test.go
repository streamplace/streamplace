package spxrpc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/statedb"
)

func bundledFile(t *testing.T, name string) []byte {
	t.Helper()
	files, err := app.Files()
	require.NoError(t, err)
	f, err := files.Open(name)
	require.NoError(t, err)
	defer f.Close()
	data, err := io.ReadAll(f)
	require.NoError(t, err)
	return data
}

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
	require.Equal(t, bundledFile(t, "favicon.png"), png.Body.Bytes())
	ico := serve(s.HandleFaviconICO, "/favicon.ico")
	require.Equal(t, "image/x-icon", ico.Header().Get("Content-Type"))
	require.Equal(t, bundledFile(t, "favicon.ico"), ico.Body.Bytes())

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

// /favicon.png?scheme= serves that color scheme's variant with its own MIME
// type, and falls back to the generic favicon, then the bundled one, when
// the variant is missing, deleted, not an image, or the scheme is unknown.
func TestFaviconSchemeVariants(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{BroadcasterHost: "example.com"}, statefulDB: sdb}
	const broadcaster = "did:web:example.com"
	type icon struct {
		data []byte
		mime string
	}
	bundled := icon{bundledFile(t, "favicon.png"), "image/png"}
	generic := icon{[]byte{0x89, 'P', 'N', 'G', 'g'}, "image/png"}
	light := icon{[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect id="light"/></svg>`), "image/svg+xml"}
	dark := icon{[]byte("RIFF....WEBPdark"), "image/webp"}

	serve := func(query string) icon {
		t.Helper()
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/favicon.png"+query, nil), rec)
		require.NoError(t, s.HandleFaviconPNG(c), query)
		require.Equal(t, http.StatusOK, rec.Code, query)
		require.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"), query)
		return icon{rec.Body.Bytes(), rec.Header().Get("Content-Type")}
	}
	put := func(key string, i icon) {
		t.Helper()
		require.NoError(t, sdb.PutBrandingBlob(broadcaster, key, i.mime, i.data, nil, nil))
	}
	expect := func(generic, light, dark icon) {
		t.Helper()
		require.Equal(t, generic, serve(""))
		require.Equal(t, generic, serve("?scheme=bogus"))
		require.Equal(t, generic, serve("?scheme="))
		require.Equal(t, light, serve("?scheme=light"))
		require.Equal(t, dark, serve("?scheme=dark"))
	}

	expect(bundled, bundled, bundled)

	put("favicon", generic)
	expect(generic, generic, generic)

	put("faviconLight", light)
	expect(generic, light, generic)

	put("faviconDark", dark)
	expect(generic, light, dark)

	// Variants win over the generic icon but never replace it for clients
	// that don't ask for a scheme.
	require.NoError(t, sdb.DeleteBrandingBlob(broadcaster, "favicon"))
	expect(bundled, light, dark)

	// A variant that isn't an image is skipped, not served.
	put("faviconLight", icon{[]byte("not an image"), "text/plain"})
	put("favicon", generic)
	expect(generic, generic, dark)

	require.NoError(t, sdb.DeleteBrandingBlob(broadcaster, "faviconDark"))
	expect(generic, generic, generic)
}
