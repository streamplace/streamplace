package spxrpc

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/statedb"
)

func TestNotificationIconUsesRuntimeLogo(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{BroadcasterHost: "example.com"}, statefulDB: sdb}
	favicon := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="red"/></svg>`)
	require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "favicon", "image/svg+xml", favicon, nil, nil))
	serve := func(t *testing.T) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/notification-icon", nil), rec)
		require.NoError(t, s.HandleNotificationIcon(c))
		require.Equal(t, http.StatusOK, rec.Code)
		return rec
	}
	checkFallback := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
		cfg, err := png.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
		require.NoError(t, err)
		require.Equal(t, 512, cfg.Width)
		require.Equal(t, 512, cfg.Height)
	}

	// A favicon alone must not replace the high-resolution bundled fallback.
	checkFallback(t, serve(t))
	logo := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path d="M256 32L480 480H32Z" fill="blue"/></svg>`)
	require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "mainLogo", "image/svg+xml", logo, nil, nil))
	rec := serve(t)
	require.Equal(t, logo, rec.Body.Bytes())
	require.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
	require.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"))

	// A later runtime rebrand uses the same binary and endpoint.
	updated := bytes.ReplaceAll(logo, []byte("blue"), []byte("green"))
	require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "mainLogo", "image/svg+xml", updated, nil, nil))
	require.Equal(t, updated, serve(t).Body.Bytes())

	for _, tc := range []struct {
		name string
		mime string
		data []byte
	}{
		{name: "empty", mime: "image/svg+xml"},
		{name: "not an image", mime: "text/plain", data: []byte("not a logo")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "mainLogo", tc.mime, tc.data, nil, nil))
			checkFallback(t, serve(t))
		})
	}
}
