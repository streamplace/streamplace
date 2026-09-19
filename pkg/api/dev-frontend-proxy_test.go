package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// deadPort returns a loopback address nothing is listening on.
func deadPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// A slow `pnpm app start` used to leave the browser on the reverse proxy's bare
// 502. It must now get the polling page. This drives the real proxy and error
// handler over a closed port.
func TestDevFrontendProxyLoadingPage(t *testing.T) {
	addr := deadPort(t)
	u, err := url.Parse("http://" + addr)
	require.NoError(t, err)
	proxy := &httputil.ReverseProxy{
		Rewrite:      func(r *httputil.ProxyRequest) { r.SetURL(u) },
		ErrorHandler: devFrontendProxyErrorHandler(context.Background(), renderDevFrontendProxyPage(u.String())),
	}

	t.Run("navigation gets the polling page", func(t *testing.T) {
		r := httptest.NewRequest("GET", "http://node.test/", nil)
		r.Header.Set("Accept", "text/html,application/xhtml+xml")
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, r)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
		require.Contains(t, rec.Body.String(), devFrontendProxyStatusPath)
		require.Contains(t, rec.Body.String(), u.String())
	})

	t.Run("non-navigation does not get the page", func(t *testing.T) {
		r := httptest.NewRequest("GET", "http://node.test/xrpc/foo", nil)
		r.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, r)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		require.NotContains(t, rec.Body.String(), devFrontendProxyStatusPath)
	})
}

// The page reloads only once the port accepts connections, so the status
// endpoint must track the listener.
func TestDevFrontendProxyStatus(t *testing.T) {
	a := &StreamplaceAPI{}
	addr := deadPort(t)
	u, err := url.Parse("http://" + addr)
	require.NoError(t, err)
	handler := a.HandleDevFrontendProxyStatus(context.Background(), u)
	call := func() int {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest("GET", devFrontendProxyStatusPath, nil), nil)
		return rec.Code
	}

	require.Equal(t, http.StatusServiceUnavailable, call())

	l, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	defer l.Close()

	require.Equal(t, http.StatusOK, call())
}
