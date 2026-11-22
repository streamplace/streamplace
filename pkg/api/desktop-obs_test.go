package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestDesktopOBSRequestConsumedOnce(t *testing.T) {
	for _, site := range []string{"", "none"} {
		t.Run("fetch-site="+site, func(t *testing.T) {
			a := &StreamplaceAPI{CLI: &config.CLI{UpstreamStation: "did:web:stream.place"}}
			ctx := context.Background()
			poll := func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				a.HandleDesktopOBSRequest(ctx)(w, httptest.NewRequest(http.MethodGet, "/api/desktop/obs-request", nil))
				return w
			}
			require.Equal(t, http.StatusNoContent, poll().Code)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/desktop/obs", nil)
			if site != "" {
				r.Header.Set("Sec-Fetch-Site", site)
			}
			a.HandleDesktopOBS(ctx)(w, r)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.Contains(t, w.Body.String(), "Finish connecting OBS in Streamplace Desktop.")
			require.NotContains(t, w.Body.String(), "{{.Title}}")
			consumed := poll()
			require.Equal(t, http.StatusOK, consumed.Code)
			require.Equal(t, "application/json", consumed.Header().Get("Content-Type"))
			require.Equal(t, "no-store", consumed.Header().Get("Cache-Control"))
			require.JSONEq(t, "{}", consumed.Body.String())
			require.Equal(t, http.StatusNoContent, poll().Code)
		})
	}
}

func TestDesktopOBSRejectsCrossSiteRequests(t *testing.T) {
	for _, site := range []string{"cross-site", "same-site", "same-origin"} {
		t.Run(site, func(t *testing.T) {
			a := &StreamplaceAPI{CLI: &config.CLI{UpstreamStation: "did:web:stream.place"}}
			ctx := context.Background()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/desktop/obs", nil)
			r.Header.Set("Sec-Fetch-Site", site)
			a.HandleDesktopOBS(ctx)(w, r)
			require.Equal(t, http.StatusForbidden, w.Code)
			w = httptest.NewRecorder()
			a.HandleDesktopOBSRequest(ctx)(w, httptest.NewRequest(http.MethodGet, "/api/desktop/obs-request", nil))
			require.Equal(t, http.StatusNoContent, w.Code)
		})
	}
}

func TestDesktopOBSRequiresSingleUserNode(t *testing.T) {
	a := &StreamplaceAPI{CLI: &config.CLI{}}
	ctx := context.Background()
	w := httptest.NewRecorder()
	a.HandleDesktopOBS(ctx)(w, httptest.NewRequest(http.MethodGet, "/desktop/obs", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	a.desktopObsPending.Store(true)
	w = httptest.NewRecorder()
	a.HandleDesktopOBSRequest(ctx)(w, httptest.NewRequest(http.MethodGet, "/api/desktop/obs-request", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.True(t, a.desktopObsPending.Load())
}
