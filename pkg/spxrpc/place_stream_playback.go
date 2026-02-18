package spxrpc

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/pion/webrtc/v4"
)

func (s *Server) handlePlaceStreamPlaybackWhep(ctx context.Context, rendition string, streamer string, r io.Reader, contentType string) (io.Reader, error) {
	if streamer == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "streamer is required")
	}
	if rendition == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "rendition is required")
	}
	repo, err := s.ATSync.SyncBlueskyRepoCached(ctx, streamer)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "error reading body", err)
	}
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: string(body)}
	answer, err := s.mm.WebRTCPlayback2(ctx, repo.DID, rendition, &offer)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error playing back", err)
	}
	return bytes.NewReader([]byte(answer.SDP)), nil
}
