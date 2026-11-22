package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestDesktopIngestRejectsBrowserRequests(t *testing.T) {
	for _, site := range []string{"cross-site", "same-site", "same-origin"} {
		t.Run(site, func(t *testing.T) {
			a := &StreamplaceAPI{CLI: &config.CLI{UpstreamStation: "did:web:station.test"}}
			r := httptest.NewRequest(http.MethodPost, "/api/ingest/client-configuration", strings.NewReader(`{"authentication":"streamplace-desktop"}`))
			r.Header.Set("Sec-Fetch-Site", site)
			w := httptest.NewRecorder()
			a.HandleClientConfiguration(t.Context())(w, r)
			require.Equal(t, http.StatusForbidden, w.Code)
			w = httptest.NewRecorder()
			a.HandleFMP4Ingest(t.Context())(w, r, httprouter.Params{{Key: "key", Value: desktopPlaceholderKey}})
			require.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

func TestDesktopPlaceholderRequiresSingleUserNode(t *testing.T) {
	a := &StreamplaceAPI{CLI: &config.CLI{}}
	key, err := a.resolveIngestKey(t.Context(), desktopPlaceholderKey)
	require.NoError(t, err)
	require.Equal(t, desktopPlaceholderKey, key)
	_, err = a.MakeMediaSigner(t.Context(), key)
	require.ErrorContains(t, err, "not a multibase")
}
