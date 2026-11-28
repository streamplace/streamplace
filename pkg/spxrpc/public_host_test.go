package spxrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestIsServerPDSForwardedHost(t *testing.T) {
	s := &Server{cli: &config.CLI{BroadcasterHost: "stream.example", ServerHost: "node.example"}}
	check := func(host, forwarded string) bool {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/xrpc/com.atproto.sync.getRepo", nil)
		req.Host = host
		if forwarded != "" {
			req.Header.Set("X-Forwarded-Host", forwarded)
		}
		ec := echo.New().NewContext(req, httptest.NewRecorder())
		return s.isServerPDS(context.WithValue(context.Background(), echoContextKey, ec))
	}

	require.True(t, check("node.example", ""))
	require.True(t, check("node.example", "node.example"))
	require.False(t, check("node.example", "stream.example"),
		"the worker proxies broadcaster requests to the node's own host")
	require.False(t, check("node.example", "unknown.example"))
}
