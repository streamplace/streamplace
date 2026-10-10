package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// The version manifest's shape is a contract with the overlays that poll it
// (/overlay/<name>): they compare the raw response and reload on any change,
// and they display the version. A renamed field or a cached response breaks
// that silently, so pin both here.
func TestHandleVersion(t *testing.T) {
	build := &config.BuildFlags{
		Version:   "v1.2.3-abcdef0",
		BuildTime: time.Date(2026, 10, 9, 15, 4, 41, 0, time.UTC).Unix(),
		UUID:      "0199c0de-0000-7000-8000-000000000000",
	}
	a := StreamplaceAPI{CLI: &config.CLI{Build: build}}

	w := httptest.NewRecorder()
	a.HandleVersion(context.Background())(w, httptest.NewRequest("GET", "/api/version", nil))

	require.Equal(t, 200, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	var got VersionManifest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, VersionManifest{
		Version:   "v1.2.3-abcdef0",
		BuildTime: "2026-10-09T15:04:41Z",
		UUID:      "0199c0de-0000-7000-8000-000000000000",
	}, got)
}

func TestHandleVersionWithoutBuildFlags(t *testing.T) {
	a := StreamplaceAPI{CLI: &config.CLI{}}

	w := httptest.NewRecorder()
	a.HandleVersion(context.Background())(w, httptest.NewRequest("GET", "/api/version", nil))

	require.Equal(t, 500, w.Code)
}
