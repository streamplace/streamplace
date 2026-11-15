package spxrpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/spaces"
)

func TestSpaceNotifications(t *testing.T) {
	space := spaces.PersonalSpaceURI("did:plc:streamer")
	token := spaces.NotifyToken(space)
	s := &Server{bus: bus.NewBus()}
	changed := s.bus.Subscribe(space)
	post := func(handler echo.HandlerFunc, body string) (string, error) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		err := handler(echo.New().NewContext(req, rec))
		return rec.Body.String(), err
	}
	update := func(space, token, kind string) error {
		_, err := post(s.handleComAtprotoSpaceNotifySpaceUpdate, `{"space":"`+space+`","token":"`+token+`","update":{"$type":"`+kind+`"}}`)
		return err
	}

	// A registration is confirmed only with the token issued for its space.
	_, err := post(s.handleComAtprotoSpaceConfirmRegistration, `{"space":"`+spaces.PersonalSpaceURI("did:plc:other")+`","token":"`+token+`","challenge":"abc"}`)
	require.Error(t, err)
	echoed, err := post(s.handleComAtprotoSpaceConfirmRegistration, `{"space":"`+space+`","token":"`+token+`","challenge":"abc"}`)
	require.NoError(t, err)
	require.JSONEq(t, `{"challenge":"abc"}`, echoed)

	// Only that token wakes the space's followers.
	require.Error(t, update(space, "", "com.atproto.space.defs#repoWrite"))
	require.Error(t, update(space, spaces.NotifyToken("at://other"), "com.atproto.space.defs#repoWrite"))
	require.Empty(t, changed)
	require.NoError(t, update(space, token, "com.atproto.space.defs#repoWrite"))
	<-changed

	// The expiry notice wakes them too, and is refused so the PDS unregisters.
	require.Error(t, update(space, token, "com.atproto.space.notifySpaceUpdate#registrationExpired"))
	<-changed
}
