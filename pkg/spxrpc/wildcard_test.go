package spxrpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
)

func TestWildcardPreservesUpstreamConflict(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"InvalidSwap","message":"record changed"}`)
	}))
	defer upstream.Close()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jwkKey, err := jwk.FromRaw(key)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(jwkKey)
	require.NoError(t, err)
	op := oatproxy.New(&oatproxy.Config{Host: "stream.example"})
	session := &oatproxy.OAuthSession{DID: "did:plc:streamer", PDSUrl: upstream.URL, UpstreamAccessToken: "test-token", UpstreamDPoPPrivateJWK: string(keyJSON)}
	req := httptest.NewRequest(http.MethodPost, "/xrpc/com.atproto.repo.putRecord", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), oatproxy.OATProxyContextKey, op)
	ctx = context.WithValue(ctx, oatproxy.OAuthSessionContextKey, session)
	req = req.WithContext(ctx)
	e := echo.New()
	e.POST("/xrpc/com.atproto.repo.putRecord", (&Server{}).HandleWildcard)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var response map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "InvalidSwap", response["error"])
	require.Equal(t, "record changed", response["message"])
}
