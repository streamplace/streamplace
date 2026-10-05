package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rivo/uniseg"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func (s *Server) HandleComAtprotoModerationCreateReport(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleComAtprotoModerationCreateReport")
	defer span.End()
	var body indigoatproto.ModerationCreateReport_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	out, err := s.handleComAtprotoModerationCreateReport(ctx, &body)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (s *Server) handleComAtprotoModerationCreateReport(ctx context.Context, body *indigoatproto.ModerationCreateReport_Input) (*indigoatproto.ModerationCreateReport_Output, error) {
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
			did = ""
			msg, err := s.model.GetChatMessage(body.Subject.RepoStrongRef.Uri)
			if err != nil {
				log.Error(ctx, "failed to get chat message for chat report", "error", err)
			} else if msg != nil && msg.CID == body.Subject.RepoStrongRef.Cid && msg.StreamerRepoDID != "" {
				did = msg.StreamerRepoDID
				chatContext := fmt.Sprintf("in chat of %s", did)
				repo, err := s.model.GetRepo(did)
				if err != nil {
					log.Warn(ctx, "failed to get streamer for chat report", "error", err)
				} else if repo != nil && repo.Handle != "" {
					chatContext = fmt.Sprintf("in chat of @%s (%s)", repo.Handle, did)
				}
				if reportReasonFits(*body.Reason, chatContext) {
					if *body.Reason != "" {
						chatContext = *body.Reason + "\n\n" + chatContext
					}
					body.Reason = &chatContext
				}
			}
		}
	} else {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid subject")
	}

	if did != "" {
		clipPrefix := fmt.Sprintf("Clip: https://%s/api/clip/%s/", s.cli.BroadcasterHost, did)
		if reportReasonFits(*body.Reason, clipPrefix+uuid.Nil.String()+".mp4") {
			clipID, err := makeClip(ctx, s.cli, s.mm, did)
			if err != nil {
				// we still want the report to go through!
				log.Error(ctx, "failed to make clip for report", "error", err)
			} else {
				newReason := clipPrefix + clipID + ".mp4"
				if *body.Reason != "" {
					newReason = *body.Reason + "\n\n" + newReason
				}
				body.Reason = &newReason
			}
		}
	}

	client.SetHeaders(map[string]string{
		"Atproto-Proxy": atprotoProxy,
	})

	var output indigoatproto.ModerationCreateReport_Output
	err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.moderation.createReport", nil, body, &output)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return &output, nil
}

// Extra context must not push a valid comment past createReport's limits.
func reportReasonFits(reason, extra string) bool {
	separator := 0
	if reason != "" {
		separator = 2
	}
	return len(reason)+separator+len(extra) <= 20000 &&
		uniseg.GraphemeClusterCount(reason)+separator+uniseg.GraphemeClusterCount(extra) <= 2000
}

func makeClip(ctx context.Context, cli *config.CLI, mm *media.MediaManager, did string) (string, error) {
	after := time.Now().Add(-time.Duration(60) * time.Second)

	uu, err := uuid.NewV7()
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to generate uuid")
	}

	fd, err := cli.DataFileCreate([]string{did, "clips", fmt.Sprintf("%s.mp4", uu.String())}, false)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to create data file")
	}
	defer func() {
		_ = fd.Close()
		if err != nil {
			if removeErr := os.Remove(fd.Name()); removeErr != nil {
				log.Error(ctx, "failed to remove incomplete report clip", "error", removeErr)
			}
		}
	}()

	err = mm.ClipUser(ctx, did, fd, nil, &after)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to clip user")
	}
	return uu.String(), nil
}
