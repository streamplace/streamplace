package spxrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamMediaCreateUpload(ctx context.Context, body *placestream.MediaCreateUpload_Input) (*placestream.MediaCreateUpload_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	// Labeler enforcement: a banned account can't start new uploads.
	if banned, err := s.accountBanned(session.DID); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return nil, echo.NewHTTPError(http.StatusForbidden, "account is not permitted to upload videos")
	}
	// Beta gate: in production the operator names a trusted invite
	// issuer via --beta-invite-did and only accounts with a "vod"
	// invite from that repo can upload. With no issuer configured we
	// fall back to the --allowed-streams allowlist that livestreaming
	// uses (which itself is open by default for dev / self-hosted).
	if err := s.allowVODUpload(ctx, session.DID); err != nil {
		return nil, echo.NewHTTPError(http.StatusForbidden, err.Error())
	}
	if s.uploadManager == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "upload manager not configured")
	}
	if !strings.HasPrefix(body.MimeType, "video/") {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "mimeType must be a video/* type")
	}

	filename := ""
	if body.Filename != nil {
		filename = *body.Filename
	}

	baseURL, err := s.requestBaseURL(ctx)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	res, err := s.uploadManager.Create(ctx, session.DID, body.MimeType, filename, body.Size, baseURL)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// If the client created a draft up front (place.stream.vod.createDraft),
	// associate this upload with it so processing fills that draft rather than
	// creating a new one. This lets the user edit metadata while the upload
	// runs and supports re-upload (a new createUpload with the same draftUri
	// re-points the draft at the new upload).
	if body.DraftUri != nil && *body.DraftUri != "" {
		if err := s.statefulDB.SetDraftOriginUpload(ctx, *body.DraftUri, session.DID, res.UploadID); err != nil {
			// A missing/stale draft isn't fatal: the upload proceeds and the
			// TUS-completion path will create a fresh draft as before.
			log.Warn(ctx, "createUpload: failed to associate draft with upload", "draftUri", *body.DraftUri, "uploadId", res.UploadID, "error", err)
		}
	}

	return &placestream.MediaCreateUpload_Output{
		UploadId:    res.UploadID,
		UploadUrl:   res.UploadURL,
		UploadToken: res.UploadToken,
		ExpiresAt:   res.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}, nil
}

func (s *Server) handlePlaceStreamMediaGetUploadStatus(ctx context.Context, uploadId string) (*placestream.MediaGetUploadStatus_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	if uploadId == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uploadId is required")
	}
	upload, err := s.statefulDB.GetUpload(ctx, uploadId)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if upload == nil || upload.RepoDID != session.DID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "upload not found")
	}

	out := &placestream.MediaGetUploadStatus_Output{}

	switch upload.ProcessingStatus {
	case "done":
		out.Status = "done"
		if upload.DurationMS > 0 {
			d := upload.DurationMS
			out.DurationMs = &d
		}
		if upload.TrackURIs != "" {
			var refs []struct {
				URI string `json:"uri"`
				CID string `json:"cid"`
			}
			if err := json.Unmarshal([]byte(upload.TrackURIs), &refs); err == nil {
				for _, r := range refs {
					out.Tracks = append(out.Tracks, &placestream.MediaGetUploadStatus_TrackRef{
						Uri: r.URI,
						Cid: r.CID,
					})
				}
			}
		}
	case "error":
		out.Status = "error"
		if upload.ProcessingError != "" {
			msg := upload.ProcessingError
			out.Error = &msg
		}
	case "processing":
		out.Status = "processing"
		p := int64(upload.ProcessingProgress)
		out.Progress = &p
	default:
		// "" or any other value: upload not yet fully received
		out.Status = "pending"
	}

	return out, nil
}

// requestBaseURL returns the scheme+host of the inbound HTTP request, used
// to construct user-facing URLs that the same client can reach.
func (s *Server) requestBaseURL(ctx context.Context) (string, error) {
	ec, ok := ctx.Value(echoContextKey).(echo.Context)
	if !ok {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "no echo context")
	}
	req := ec.Request()
	scheme := "https"
	if !s.cli.Secure {
		scheme = "http"
	}
	if fwd := req.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	host := req.Host
	if fwd := req.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host, nil
}
