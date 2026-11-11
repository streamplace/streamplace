package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/linking"
)

func TestFrontendCachePolicy(t *testing.T) {
	const bundle = "_expo/static/js/web/entry-4435f578c13fb3bd2a64b65e8dc7f216.js"
	index := []byte("<!doctype html><html><head><title>Streamplace</title></head><body></body></html>")
	files := fstest.MapFS{
		"index.html":         {Data: index},
		"assets/favicon.png": {Data: []byte("png")},
		"sw.js":              {Data: []byte("// service worker")},
		bundle:               {Data: []byte("console.log(1)")},
	}
	linker, err := linking.NewLinker(context.Background(), index, nil, nil)
	require.NoError(t, err)
	a := &StreamplaceAPI{CLI: &config.CLI{}}
	handler, err := a.notFoundLinkingHandler(context.Background(), linker, files)
	require.NoError(t, err)

	for _, tc := range []struct {
		path   string
		status int
		cache  string
	}{
		{"/" + bundle, http.StatusOK, "public, max-age=31536000, immutable"},
		{"/_expo/static/js/web/entry-gone.js", http.StatusNotFound, "no-store"},
		{"/_expo/static/css/gone-0123456789abcdef.css", http.StatusNotFound, "no-store"},
		{"/", http.StatusOK, "no-cache"},
		{"/settings/account/profile", http.StatusOK, "no-cache"},
		{"/sw.js", http.StatusOK, "no-cache"},
		{"/assets/favicon.png", http.StatusOK, ""},
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest("GET", tc.path, nil))
		require.Equal(t, tc.status, rec.Code, tc.path)
		require.Equal(t, tc.cache, rec.Header().Get("Cache-Control"), tc.path)
	}
}
