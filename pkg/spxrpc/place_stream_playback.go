package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pion/webrtc/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/constants"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamPlaybackGetPlaybackServer(ctx context.Context, stream string) (*placestream.PlaybackGetPlaybackServer_Output, error) {
	addr := strings.TrimPrefix(s.cli.ServerDID(), "did:web:")
	return &placestream.PlaybackGetPlaybackServer_Output{
		Servers: []string{fmt.Sprintf("https://%s", addr)},
	}, nil
}

func (s *Server) handlePlaceStreamPlaybackWhep(ctx context.Context, rendition string, streamer string, r io.Reader, _contentType string) (io.Reader, error) {
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
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session != nil {
		viewer = session.DID
	} else {
		svc := GetServiceAuth(ctx)
		if svc != nil {
			// this is a signed request from a peer node, allow them to see unpublished streams
			viewer = streamer
		}
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "error reading body", err)
	}
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: string(body)}
	if streamer == viewer {
		// this user gets sent right to the origin in case we're unpublished
		origin, err := s.statefulDB.GetLatestBroadcastOriginForStreamer(streamer)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting broadcast origin", err)
		}
		myDID := s.cli.ServerDID()
		if origin != nil && origin.ServerDID != myDID {
			data, err := s.ProxyServiceRequest(ctx, origin.ServerDID, "POST", "place.stream.playback.whep",
				url.Values{"rendition": {rendition}, "streamer": {streamer}},
				bytes.NewReader([]byte(offer.SDP)), _contentType)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(data), nil
		}
	}
	answer, err := s.mm.WebRTCPlayback2(ctx, streamer, rendition, &offer, viewer)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error playing back: %s", err.Error()), err)
	}
	return bytes.NewReader([]byte(answer.SDP)), nil
}
