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

func TestRequestBaseURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://stream.place/xrpc/place.stream.media.createUpload", nil)
	req.Header.Set("X-Forwarded-Host", "stream.place")
	ec := echo.New().NewContext(req, httptest.NewRecorder())
	ctx := context.WithValue(context.Background(), echoContextKey, ec)

	// Single node: the public host the client used.
	s := &Server{cli: &config.CLI{BroadcasterHost: "stream.place", ServerHost: "stream.place", Secure: true}}
	got, err := s.requestBaseURL(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://stream.place", got)

	// A node with its own host advertises it, so upload chunks skip the edge.
	s.cli.ServerHost = "node.stream.place"
	got, err = s.requestBaseURL(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://node.stream.place", got)
}
