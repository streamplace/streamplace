package spxrpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/vod"
)

// The lexgen stub streams its reader with no Content-Length; NewServer
// registers HandleGetClip over it so a client knows the clip's size up front.
func (s *Server) handlePlaceStreamLiveGetClip(ctx context.Context, end string, start string, streamer string) (io.Reader, error) {
	return nil, stubMisrouted("getClip")
}

// HandleGetClip serves place.stream.live.getClip: the streamer's published
// segments starting in [start, end], as a flat MP4. Any signed-in caller may
// clip — an OAuth session, or a service-auth token bound to this method, which
// is how a third-party app holding the rpc scope calls it through its PDS.
func (s *Server) HandleGetClip(c echo.Context) error {
	ctx := c.Request().Context()
	if GetCaller(ctx) == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "AuthenticationRequired")
	}
	start, err := time.Parse(time.RFC3339, c.QueryParam("start"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "start must be a datetime")
	}
	end, err := time.Parse(time.RFC3339, c.QueryParam("end"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "end must be a datetime")
	}
	if !end.After(start) || end.Sub(start) > vod.MaxClipDuration {
		return echo.NewHTTPError(http.StatusBadRequest, "end must be after start, by at most "+vod.MaxClipDuration.String())
	}
	did, err := s.resolveStreamer(ctx, c.QueryParam("streamer"))
	if err != nil {
		return err
	}
	if banned, err := s.accountBanned(did); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return echo.NewHTTPError(http.StatusForbidden, "StreamUnavailable")
	}

	clip, err := vod.FindClip(ctx, s.statefulDB, s.playbackStore, s.mm, did, start, end, false)
	switch {
	case errors.Is(err, vod.ErrClipNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "ClipNotFound")
	case errors.Is(err, vod.ErrClipUnavailable):
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ClipUnavailable")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	header, err := clip.Header(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h := c.Response().Header()
	h.Set("Content-Type", "video/mp4")
	h.Set("Content-Length", strconv.FormatInt(int64(len(header))+clip.BodySize(), 10))
	c.Response().WriteHeader(http.StatusOK)
	if _, err := c.Response().Write(header); err != nil {
		return nil
	}
	if err := clip.WriteSegments(ctx, c.Response()); err != nil {
		// The status is already sent; the short body tells the client.
		log.Warn(ctx, "getClip: writing clip body failed", "streamer", did, "error", err)
	}
	return nil
}
