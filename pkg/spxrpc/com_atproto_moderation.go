package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
)

func (s *Server) handleComAtprotoModerationCreateReport(ctx context.Context, body *comatprototypes.ModerationCreateReport_Input) (*comatprototypes.ModerationCreateReport_Output, error) {
	c, ok := ctx.Value(echoContextKey).(echo.Context)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "echo context not found")
	}

	atprotoProxy := c.Request().Header.Get("Atproto-Proxy")
	if atprotoProxy == "" {
		if len(s.cli.Labelers) > 0 {
			atprotoProxy = fmt.Sprintf("%s#atproto_labeler", s.cli.Labelers[0])
		} else {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Atproto-Proxy header is required (where are you sending this report?)")
		}
	}

	log.Log(ctx, "handleComAtprotoModerationCreateReport", "body", body)

	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	if body.Reason == nil {
		empty := ""
		body.Reason = &empty
	}

	if body.Subject == nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "subject is required")
	}

	var did string

	if body.Subject.AdminDefs_RepoRef != nil {
		d, err := syntax.ParseDID(body.Subject.AdminDefs_RepoRef.Did)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid subject did")
		}
		did = d.String()
	} else if body.Subject.RepoStrongRef != nil {
		aturi, err := syntax.ParseATURI(body.Subject.RepoStrongRef.Uri)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid subject uri")
		}
		did = aturi.Authority().String()
		// if it's chat, we want the clip from the streamer, not from the chatter
		if aturi.Collection() == "place.stream.chat.message" {
			msg, err := s.model.GetChatMessage(body.Subject.RepoStrongRef.Uri)
			if err != nil {
				log.Error(ctx, "failed to get chat message for chat report", "error", err)
			} else {
				did = msg.StreamerRepoDID
			}
		}
	} else {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid subject")
	}

	clipID, err := makeClip(ctx, s.cli, s.model, did)
	if err != nil {
		// we still want the report to go through!
		log.Error(ctx, "failed to make clip for report", "error", err)
	} else {
		clipURL := fmt.Sprintf("https://%s/api/clip/%s/%s.mp4", s.cli.BroadcasterHost, did, clipID)
		newReason := fmt.Sprintf("%s\n\nClip: %s", *body.Reason, clipURL)
		body.Reason = &newReason
	}

	client.SetHeaders(map[string]string{
		"Atproto-Proxy": atprotoProxy,
	})

	var output comatprototypes.ModerationCreateReport_Output
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.moderation.createReport", nil, body, &output)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return &output, nil
}

func makeClip(ctx context.Context, cli *config.CLI, mod model.Model, did string) (string, error) {
	after := time.Now().Add(-time.Duration(60) * time.Second)

	uu, err := uuid.NewV7()
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to generate uuid")
	}

	fd, err := cli.DataFileCreate([]string{did, "clips", fmt.Sprintf("%s.mp4", uu.String())}, false)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to create data file")
	}
	defer fd.Close()

	err = media.ClipUser(ctx, mod, cli, did, fd, nil, &after)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to clip user")
	}
	return uu.String(), nil
}
