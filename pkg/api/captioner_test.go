package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/stt"
)

func TestCaptionerModelAvailability(t *testing.T) {
	previous := stt.ModelFiles
	t.Cleanup(func() { stt.ModelFiles = previous })
	stt.ModelFiles = nil
	handler := captionerAssets()
	request := httptest.NewRequest(http.MethodGet, captionerBasePath+"ggml-tiny-q5_1.bin", nil)
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, request)
	require.Equal(t, http.StatusServiceUnavailable, missing.Code)
	require.Empty(t, missing.Header().Get("Cache-Control"))

	stt.ModelFiles = fstest.MapFS{"ggml-tiny-q5_1.bin": &fstest.MapFile{Data: []byte("model")}}
	available := httptest.NewRecorder()
	handler.ServeHTTP(available, request)
	require.Equal(t, http.StatusOK, available.Code)
	require.Equal(t, "model", available.Body.String())
	require.Equal(t, "public, max-age=31536000, immutable", available.Header().Get("Cache-Control"))
	require.Equal(t, "same-origin", available.Header().Get("Cross-Origin-Opener-Policy"))
	require.Equal(t, "credentialless", available.Header().Get("Cross-Origin-Embedder-Policy"))

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, captionerBasePath+"../licenses.txt", nil))
	require.Equal(t, http.StatusNotFound, unknown.Code)
}

func TestCaptionerIsolationOptIn(t *testing.T) {
	handler := captionerIsolation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, path := range []string{"/captioner", "/live?deviceCaptions=1"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, "same-origin", response.Header().Get("Cross-Origin-Opener-Policy"))
		require.Equal(t, "credentialless", response.Header().Get("Cross-Origin-Embedder-Policy"))
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Empty(t, response.Header().Get("Cross-Origin-Opener-Policy"))
}
