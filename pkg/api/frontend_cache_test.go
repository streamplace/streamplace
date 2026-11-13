package api

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestFrontendCacheControl(t *testing.T) {
	const immutable = "public, max-age=31536000, immutable"
	tests := []struct {
		path      string
		cache     string
		generated bool
	}{
		{"/_expo/static/js/web/entrypoint-9a0e003fe7fb3c660e4fceba5acfe631.js", immutable, false},
		{"/_expo/static/js/web/bootstrap-1810069e34ddde70.js", immutable, false},
		{"/_expo/static/js/web/entrypoint-9a0e003fe7fb3c660e4fceba5acfe631.js.map", immutable, false},
		{"/_expo/static/css/web/global-9a0e003fe7fb3c660e4fceba5acfe631.css", immutable, false},
		{"/assets/assets/fonts/Geist-Regular.923ad9351ff67367546d8254e28c1aff.ttf", immutable, false},
		{"/assets/923ad9351ff67367546d8254e28c1aff", immutable, false},
		{"/assets/images/goose.7f0d685e63fcda3a6f2b3024b6f76b32.png", immutable, false},
		{"/_expo/static/js/web/entrypoint.js", "no-cache", false},
		{"/assets/logo.png", "no-cache", false},
		{"/sw.js", "no-cache", false},
		{"/locales/en-US/common.json", "no-cache", false},
		{"/page.html", "no-cache", false},
		{"/desktop/", "no-cache", true},
		{"/live", "no-cache", true},
	}
	files := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><html><head><title>Streamplace</title></head><body></body></html>")},
	}
	for _, tt := range tests {
		if !tt.generated {
			files[strings.TrimPrefix(tt.path, "/")] = &fstest.MapFile{Data: []byte("asset")}
		}
	}
	a := &StreamplaceAPI{CLI: &config.CLI{}}
	staticHandler := a.FileHandler(context.Background(), http.FileServer(http.FS(files)))
	handler, err := a.buildLinkingHandler(context.Background(), func() (fs.FS, error) { return files, nil })
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			serve := staticHandler
			if tt.generated {
				serve = handler
			}
			rec := httptest.NewRecorder()
			serve.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.cache, rec.Header().Get("Cache-Control"))
		})
	}

	t.Run("index redirect", func(t *testing.T) {
		rec := httptest.NewRecorder()
		staticHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
		require.Equal(t, http.StatusMovedPermanently, rec.Code)
		require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	})
}
