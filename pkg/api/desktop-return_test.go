package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
)

func TestDesktopOAuthReturnRelay(t *testing.T) {
	const token = "desktop-relay-test-0123456789"
	api := &StreamplaceAPI{}
	returnHandler := api.HandleDesktopOAuthReturn(context.Background())
	callbackHandler := api.HandleDesktopOAuthCallback(context.Background())
	params := httprouter.Params{{Key: "token", Value: token}}

	returnRec := httptest.NewRecorder()
	returnHandler(
		returnRec,
		httptest.NewRequest(http.MethodGet, "/app-return/desktop/"+token+"?code=code-1&state=state-1&iss=issuer-1&junk=dropped", nil),
		params,
	)
	require.Equal(t, http.StatusOK, returnRec.Code)
	require.Contains(t, returnRec.Body.String(), "logged in")

	callbackRec := httptest.NewRecorder()
	callbackHandler(
		callbackRec,
		httptest.NewRequest(http.MethodGet, "/api/desktop/oauth-callback/"+token, nil),
		params,
	)
	require.Equal(t, http.StatusOK, callbackRec.Code)
	var out map[string]string
	require.NoError(t, json.Unmarshal(callbackRec.Body.Bytes(), &out))
	require.Equal(t, "code-1", out["code"])
	require.Equal(t, "state-1", out["state"])
	require.Equal(t, "issuer-1", out["iss"])
	require.NotContains(t, out, "junk")

	// The callback is handed out once.
	callbackRec = httptest.NewRecorder()
	callbackHandler(
		callbackRec,
		httptest.NewRequest(http.MethodGet, "/api/desktop/oauth-callback/"+token, nil),
		params,
	)
	require.Equal(t, http.StatusNotFound, callbackRec.Code)
}

func TestDesktopOAuthReturnValidation(t *testing.T) {
	api := &StreamplaceAPI{}
	returnHandler := api.HandleDesktopOAuthReturn(context.Background())
	callbackHandler := api.HandleDesktopOAuthCallback(context.Background())

	invalid := httptest.NewRecorder()
	returnHandler(
		invalid,
		httptest.NewRequest(http.MethodGet, "/app-return/desktop/short?code=c", nil),
		httprouter.Params{{Key: "token", Value: "short"}},
	)
	require.Equal(t, http.StatusBadRequest, invalid.Code)

	missing := httptest.NewRecorder()
	returnHandler(
		missing,
		httptest.NewRequest(http.MethodGet, "/app-return/desktop/desktop-relay-test-0123456789", nil),
		httprouter.Params{{Key: "token", Value: "desktop-relay-test-0123456789"}},
	)
	require.Equal(t, http.StatusBadRequest, missing.Code)

	noPending := httptest.NewRecorder()
	callbackHandler(
		noPending,
		httptest.NewRequest(http.MethodGet, "/api/desktop/oauth-callback/desktop-relay-test-0123456789", nil),
		httprouter.Params{{Key: "token", Value: "desktop-relay-test-0123456789"}},
	)
	require.Equal(t, http.StatusNotFound, noPending.Code)
}

// The public return route can't be used to grow node memory without bound.
func TestDesktopOAuthReturnBounded(t *testing.T) {
	desktopOAuthStore.Lock()
	desktopOAuthStore.pending = map[string]desktopOAuthPending{}
	desktopOAuthStore.Unlock()
	t.Cleanup(func() {
		desktopOAuthStore.Lock()
		desktopOAuthStore.pending = map[string]desktopOAuthPending{}
		desktopOAuthStore.Unlock()
	})
	returnHandler := (&StreamplaceAPI{}).HandleDesktopOAuthReturn(context.Background())
	send := func(token, query string) int {
		rec := httptest.NewRecorder()
		returnHandler(rec, httptest.NewRequest(http.MethodGet, "/app-return/desktop/"+token+"?"+query, nil),
			httprouter.Params{{Key: "token", Value: token}})
		return rec.Code
	}
	long := strings.Repeat("a", desktopReturnMaxParamLen+1)
	require.Equal(t, http.StatusBadRequest, send("desktop-bounded-test-long", "code="+long))
	for i := range desktopReturnMaxPending {
		require.Equal(t, http.StatusOK, send(fmt.Sprintf("desktop-bounded-test-%04d", i), "code=c"))
	}
	require.Equal(t, http.StatusTooManyRequests, send("desktop-bounded-test-full", "code=c"))
	// A repeated return for an already pending login still lands.
	require.Equal(t, http.StatusOK, send("desktop-bounded-test-0000", "code=c2"))
}
