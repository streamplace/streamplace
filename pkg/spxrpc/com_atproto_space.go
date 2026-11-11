package spxrpc

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spmetrics"
)

// handleComAtprotoSpaceNotify takes a PDS's word that a personal space was
// written to or deleted, and wakes whatever on this node follows that space.
// It serves com.atproto.space.notifyWrite and notifySpaceDeleted.
func (s *Server) handleComAtprotoSpaceNotify(c echo.Context) error {
	var body struct {
		Space string `json:"space"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid notification")
	}
	// A PDS signs as the space's authority, which can only speak for its own space.
	caller := GetCaller(c.Request().Context())
	if caller == nil || body.Space != spaces.PersonalSpaceURI(caller.DID) {
		return echo.NewHTTPError(http.StatusUnauthorized, "not this space's authority")
	}
	spmetrics.SpaceNotificationsTotal.Inc()
	s.bus.Publish(spaces.ChangedTopic(caller.DID), struct{}{})
	return c.JSON(http.StatusOK, struct{}{})
}
