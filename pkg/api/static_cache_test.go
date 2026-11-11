package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/linking"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
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

func TestFrontendStreamerHandleWithAssetSuffix(t *testing.T) {
	ctx := context.Background()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	db, err := mod.(*model.DBModel).DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	repo := &model.Repo{DID: "did:plc:streamer", Handle: "alice.zip"}
	require.NoError(t, mod.UpdateRepo(repo))
	record := &placestream.Livestream{
		Title:     "Stream from an asset-like handle",
		CreatedAt: "2026-11-01T00:00:00Z",
	}
	var buf bytes.Buffer
	require.NoError(t, record.MarshalCBOR(&buf))
	body := buf.Bytes()
	require.NoError(t, mod.CreateLivestream(ctx, &model.Livestream{
		URI:        "at://" + repo.DID + "/place.stream.livestream/stream",
		RepoDID:    repo.DID,
		Livestream: &body,
	}))

	index := []byte("<!doctype html><html><head><title>Streamplace</title></head><body></body></html>")
	linker, err := linking.NewLinker(ctx, index, nil, nil)
	require.NoError(t, err)
	a := &StreamplaceAPI{CLI: &config.CLI{}, Model: mod}
	handler, err := a.notFoundLinkingHandler(ctx, linker, fstest.MapFS{"index.html": {Data: index}})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest("GET", "/"+repo.Handle, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), record.Title)
}
