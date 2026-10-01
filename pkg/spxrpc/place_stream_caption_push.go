package spxrpc

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/text/language"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/placestream"
)

// A maximum batch needs under 1 MiB even with four-byte Unicode and JSON escapes.
const captionPushBodyLimit = "2M"

func captionPushBodyLimitMiddleware() echo.MiddlewareFunc {
	return echomiddleware.BodyLimitWithConfig(echomiddleware.BodyLimitConfig{
		Limit: captionPushBodyLimit,
		Skipper: func(c echo.Context) bool {
			return c.Request().URL.Path != "/xrpc/place.stream.caption.pushCaptions"
		},
	})
}

func (s *Server) captionPusher(ctx context.Context) (string, error) {
	if session, _ := oatproxy.GetOAuthSession(ctx); session != nil {
		return session.DID, nil
	}
	ec, _ := ctx.Value(echoContextKey).(echo.Context)
	if ec == nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "streamer authorization required")
	}
	auth := strings.Fields(ec.Request().Header.Get("Authorization"))
	if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "streamer authorization required")
	}
	did, _, err := media.AuthenticateStreamKey(ctx, s.cli, s.model, s.ATSync, auth[1], true)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "invalid stream key", err)
	}
	return did, nil
}

func (s *Server) handlePlaceStreamCaptionPushCaptions(ctx context.Context, body *placestream.CaptionPushCaptions_Input) (*placestream.CaptionPushCaptions_Output, error) {
	did, err := s.captionPusher(ctx)
	if err != nil {
		return nil, err
	}
	if body.Streamer != nil && *body.Streamer != "" && *body.Streamer != did {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Forbidden")
	}
	if len(body.Cues) > 100 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "caption batch exceeds 100 cues")
	}
	if body.Language == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "caption language required")
	}
	if _, err := language.Parse(body.Language); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid caption language", err)
	}
	policy, live := s.mm.OriginCaptionPolicy(did)
	if !live {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "StreamNotLive")
	}
	source := captions.SourceHuman
	if body.Source != nil {
		source = captions.Source(*body.Source)
	}
	if source != captions.SourceHuman && source != captions.SourceAuto {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid caption source")
	}
	origin := captions.PushedOrigin(policy)
	track := captions.Track{ID: captions.TrackID(origin, source, body.Language), Language: body.Language, Kind: captions.KindCaptions, Source: source, Origin: origin, Author: did, Label: "Captions"}
	cues := make([]captions.Cue, 0, len(body.Cues))
	for _, input := range body.Cues {
		if len(input.Text) > 2000 || (input.Id != nil && len(*input.Id) > 64) {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "caption cue exceeds lexicon limits")
		}
		start, err := time.Parse(time.RFC3339Nano, input.StartTime)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cue start", err)
		}
		end, err := time.Parse(time.RFC3339Nano, input.EndTime)
		if err != nil || !end.After(start) {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cue end")
		}
		id := uuid.NewString()
		if input.Id != nil && *input.Id != "" {
			id = *input.Id
		}
		final := input.Final == nil || *input.Final
		cues = append(cues, captions.Cue{ID: id, Start: start, End: end, Text: input.Text, Final: final})
	}
	if origin == captions.OriginCanonical {
		if err := s.mm.PushCanonicalCaptions(did, track, cues); err != nil {
			return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "caption master unavailable", err)
		}
	} else {
		for _, cue := range cues {
			s.bus.Captions.Publish(did, track, cue)
		}
	}
	return &placestream.CaptionPushCaptions_Output{}, nil
}
