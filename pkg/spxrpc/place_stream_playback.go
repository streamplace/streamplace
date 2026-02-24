package spxrpc

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/pion/webrtc/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
)

func (s *Server) handlePlaceStreamPlaybackWhep(ctx context.Context, rendition string, streamer string, r io.Reader, _contentType string) (io.Reader, error) {

	if streamer == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "streamer is required")
	}
	if rendition == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "rendition is required")
	}
	viewer := ""
	repo, err := s.ATSync.SyncBlueskyRepoCached(ctx, streamer)
	if err != nil {
		return nil, err
	}
	streamer = repo.DID
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session != nil {
		viewer = session.DID
	} else {
		svc := GetServiceAuth(ctx)
		if svc != nil {
			log.Warn(ctx, "service auth present", "service_did", svc.DID)
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
		log.Warn(ctx, "origin", "origin", origin)
		myDID := s.cli.ServerDID()
		if origin != nil && origin.ServerDID != myDID {
			log.Warn(ctx, "proxying to origin", "origin", origin.ServerDID, "myDID", myDID)
			token, err := CreateServiceToken(s.cli.ServiceAuthKey, s.cli.ServerDID())
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusInternalServerError, "error creating service token", err)
			}
			parsedOrigin := origin.ServerDID
			if len(parsedOrigin) > len("did:web:") && parsedOrigin[:len("did:web:")] == "did:web:" {
				parsedOrigin = parsedOrigin[len("did:web:"):]
			}
			url := "https://" + parsedOrigin + "/xrpc/place.stream.playback.whep?rendition=" + rendition + "&streamer=" + streamer
			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte(offer.SDP)))
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to construct origin request", err)
			}
			req.Header.Set("Content-Type", _contentType)
			req.Header.Set("X-Streamplace-Service-Auth", token)
			resp, err := aqhttp.Client.Do(req)
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusBadGateway, "error proxying to origin", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return nil, echo.NewHTTPError(resp.StatusCode, "origin error: "+string(body))
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusInternalServerError, "error reading origin response", err)
			}
			return bytes.NewReader(data), nil
		}
	}
	answer, err := s.mm.WebRTCPlayback2(ctx, repo.DID, rendition, &offer, viewer)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error playing back", err)
	}
	return bytes.NewReader([]byte(answer.SDP)), nil
}
