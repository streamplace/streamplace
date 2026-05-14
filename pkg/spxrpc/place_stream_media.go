package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	placestream "stream.place/streamplace/pkg/streamplace"
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
	if s.uploadManager == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "upload manager not configured")
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

	return &placestream.MediaCreateUpload_Output{
		UploadId:    res.UploadID,
		UploadUrl:   res.UploadURL,
		UploadToken: res.UploadToken,
		ExpiresAt:   res.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}, nil
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
