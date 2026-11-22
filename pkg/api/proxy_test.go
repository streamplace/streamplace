package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	a := &StreamplaceAPI{CLI: &config.CLI{ProxySecret: "s3cret"}}
	require.Equal(t, "10.0.0.1", a.clientIP(req), "no forwarded address: the peer")

	req.Header.Set("X-Proxy-Secret", "wrong")
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	require.Equal(t, "10.0.0.1", a.clientIP(req), "a wrong secret is ignored")

	req.Header.Set("X-Proxy-Secret", "s3cret")
	require.Equal(t, "203.0.113.7", a.clientIP(req), "the first forwarded address")

	// Without a configured secret the header is just client input.
	b := &StreamplaceAPI{CLI: &config.CLI{}}
	require.Equal(t, "10.0.0.1", b.clientIP(req))
}
