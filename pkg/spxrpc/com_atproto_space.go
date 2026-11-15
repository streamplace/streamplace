package spxrpc

import (
	"crypto/hmac"
	"net/http"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spmetrics"
)

// spaceNotification is what a PDS posts to a node that asked to follow a
// space (spaces.Follow). It carries no service auth: the token vouches for it.
type spaceNotification struct {
	Space     string `json:"space"`
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
	Update    struct {
		Type string `json:"$type"`
	} `json:"update"`
}

// bindSpaceNotification refuses a token this node did not issue for the space.
func bindSpaceNotification(c echo.Context) (*spaceNotification, error) {
	var body spaceNotification
	if err := c.Bind(&body); err != nil || !hmac.Equal([]byte(body.Token), []byte(spaces.NotifyToken(body.Space))) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "unknown space notification token")
	}
	return &body, nil
}

// handleComAtprotoSpaceConfirmRegistration tells a PDS that this node asked
// for the registration it is about to make.
func (s *Server) handleComAtprotoSpaceConfirmRegistration(c echo.Context) error {
	body, err := bindSpaceNotification(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"challenge": body.Challenge})
}

// handleComAtprotoSpaceNotifySpaceUpdate wakes whatever on this node follows
// the space. Updates carry no records, so every kind means: read it again.
func (s *Server) handleComAtprotoSpaceNotifySpaceUpdate(c echo.Context) error {
	body, err := bindSpaceNotification(c)
	if err != nil {
		return err
	}
	spmetrics.SpaceNotificationsTotal.Inc()
	s.bus.Publish(body.Space, struct{}{})
	if body.Update.Type == "com.atproto.space.notifySpaceUpdate#registrationExpired" {
		// Refusing this notice unregisters. A stream still following registers again.
		return echo.NewHTTPError(http.StatusBadRequest, "registration not renewed")
	}
	return c.JSON(http.StatusOK, struct{}{})
}
