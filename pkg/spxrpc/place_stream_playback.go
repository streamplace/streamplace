package spxrpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/psession"
)

func (s *Server) handlePlaceStreamPlaybackWhep(ctx context.Context, rendition string, sid string, streamer string, r io.Reader, _contentType string) (io.Reader, error) {
	if alias, ok := s.aliases[streamer]; ok {
		streamer = alias
	}

	if streamer == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "streamer is required")
	}
	if rendition == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "rendition is required")
	}
	viewer := ""
	if !s.cli.WideOpen && !strings.HasPrefix(streamer, constants.DID_KEY_PREFIX) {
		repo, err := s.ATSync.SyncBlueskyRepoCached(ctx, streamer)
		if err != nil {
			return nil, err
		}
		streamer = repo.DID
	}
	// prelive is the streamer's own playback session, which opens their
	// pre-live stream: verified here for their own client, by the forwarding
	// node for a signed request from a peer.
	prelive := ""
	session := GetCaller(ctx)
	if session != nil {
		viewer = session.DID
		if sid != "" && viewer == streamer {
			c, ok := ctx.Value(echoContextKey).(echo.Context)
			if !ok {
				return nil, fmt.Errorf("no request context")
			}
			sess, err := s.verifySession(ctx, sid, streamer, s.sessionClient(c))
			if err != nil || sess.Scope != psession.ScopeOwner {
				return nil, errBadSession
			}
			prelive = sess.ID
		}
	} else if svc := GetServiceAuth(ctx); svc != nil {
		viewer = streamer
		prelive = psession.ID(sid)
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "error reading body", err)
	}
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: string(body)}
	if prelive != "" {
		// pre-live media is only on the origin
		origin, err := s.statefulDB.GetLatestBroadcastOriginForStreamer(streamer)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting broadcast origin", err)
		}
		myDID := s.cli.ServerDID()
		if origin != nil && origin.ServerDID != myDID {
			data, err := s.ProxyServiceRequest(ctx, origin.ServerDID, "POST", "place.stream.playback.whep",
				url.Values{"rendition": {rendition}, "streamer": {streamer}, "sid": {sid}},
				bytes.NewReader([]byte(offer.SDP)), _contentType)
			var httpErr *echo.HTTPError
			if errors.As(err, &httpErr) && httpErr.Code == http.StatusConflict {
				return nil, errPlaybackElsewhere
			}
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(data), nil
		}
	}
	answer, err := s.mm.WebRTCPlayback2(ctx, streamer, rendition, &offer, viewer, prelive)
	if errors.Is(err, media.ErrPlaybackStartedElsewhere) {
		return nil, errPlaybackElsewhere
	}
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error playing back: %s", err.Error()), err)
	}
	return bytes.NewReader([]byte(answer.SDP)), nil
}
