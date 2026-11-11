package spxrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/spaces"
)

func TestSpaceNotify(t *testing.T) {
	const did = "did:plc:streamer"
	s := &Server{bus: bus.NewBus()}
	changed := s.bus.Subscribe(spaces.ChangedTopic(did))
	notify := func(issuer, space string) error {
		req := httptest.NewRequest(http.MethodPost, "/xrpc/com.atproto.space.notifyWrite", strings.NewReader(`{"space":"`+space+`","repo":"`+did+`"}`))
		req.Header.Set("Content-Type", "application/json")
		if issuer != "" {
			req = req.WithContext(context.WithValue(req.Context(), pdsAuthContextKey, issuer))
		}
		return s.handleComAtprotoSpaceNotify(echo.New().NewContext(req, httptest.NewRecorder()))
	}

	// Only the space's own authority may wake its followers.
	require.Error(t, notify("", spaces.PersonalSpaceURI(did)))
	require.Error(t, notify("did:plc:other", spaces.PersonalSpaceURI(did)))
	require.Empty(t, changed)

	require.NoError(t, notify(did, spaces.PersonalSpaceURI(did)))
	<-changed
}
