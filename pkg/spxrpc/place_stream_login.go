package spxrpc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/util"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

const qrLoginTTL = 5 * time.Minute

// maxQRLoginUserAgent bounds the User-Agent an unauthenticated caller gets to
// store and show the approver.
const maxQRLoginUserAgent = 512

func (s *Server) handlePlaceStreamLoginCreateQRLogin(ctx context.Context, body *placestream.LoginCreateQRLogin_Input) (*placestream.LoginCreateQRLogin_Output, error) {
	// Only the Streamplace app approves QR logins, and linking a session needs
	// this node to be a confidential OAuth client, which public mode isn't.
	if s.cli.AppBundleID == "" || s.cli.PublicOAuth {
		return nil, echo.NewHTTPError(http.StatusNotImplemented, "QRLoginUnavailable")
	}
	if body.RequestUri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "requestUri is required")
	}
	c, ok := ctx.Value(echoContextKey).(echo.Context)
	if !ok {
		return nil, fmt.Errorf("no echo context")
	}
	userAgent := c.Request().UserAgent()
	if len(userAgent) > maxQRLoginUserAgent {
		userAgent = strings.ToValidUTF8(userAgent[:maxQRLoginUserAgent], "")
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("failed to generate qr login id: %w", err)
	}
	now := time.Now().UTC()
	l := &statedb.QRLogin{
		ID:         base64.RawURLEncoding.EncodeToString(id),
		RequestURI: body.RequestUri,
		UserAgent:  userAgent,
		IP:         c.RealIP(),
		Status:     statedb.QRLoginStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(qrLoginTTL),
	}
	if err := s.statefulDB.CreateQRLogin(ctx, l); err != nil {
		return nil, fmt.Errorf("failed to create qr login: %w", err)
	}
	return &placestream.LoginCreateQRLogin_Output{
		Id: l.ID,
		// the OAuth issuer's host, as in the client metadata's redirect URIs
		Url:       fmt.Sprintf("https://%s/login/qr/%s", s.cli.BroadcasterHost, l.ID),
		ExpiresAt: l.ExpiresAt.Format(util.ISO8601),
	}, nil
}

func (s *Server) handlePlaceStreamLoginGetQRLogin(ctx context.Context, id string) (*placestream.LoginDefs_QrLogin, error) {
	l, err := s.statefulDB.GetQRLogin(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get qr login: %w", err)
	}
	if l == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "NotFound")
	}
	return qrLoginView(l), nil
}

func (s *Server) handlePlaceStreamLoginCompleteQRLogin(ctx context.Context, body *placestream.LoginCompleteQRLogin_Input) (*placestream.LoginDefs_QrLogin, error) {
	// approving hands the browser the caller's own upstream session, so the
	// caller must have one here: PDS service auth doesn't do
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	l, err := s.statefulDB.GetQRLogin(ctx, body.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to get qr login: %w", err)
	}
	if l == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "NotFound")
	}
	if l.CurrentStatus() != statedb.QRLoginStatusPending {
		return nil, echo.NewHTTPError(http.StatusConflict, "AlreadyCompleted")
	}
	var auth *oatproxy.AuthorizationResponse
	if body.Approve {
		// LinkSession completes the pushed authorization request at most
		// once, so a racing second approval fails here or below
		auth, err = s.op.LinkSession(ctx, l.RequestURI, session)
		if err != nil {
			return nil, fmt.Errorf("failed to link session: %w", err)
		}
	}
	completed, err := s.statefulDB.CompleteQRLogin(ctx, l.ID, session.DID, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to complete qr login: %w", err)
	}
	if !completed {
		return nil, echo.NewHTTPError(http.StatusConflict, "AlreadyCompleted")
	}
	l, err = s.statefulDB.GetQRLogin(ctx, l.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get qr login: %w", err)
	}
	return qrLoginView(l), nil
}

func qrLoginView(l *statedb.QRLogin) *placestream.LoginDefs_QrLogin {
	out := &placestream.LoginDefs_QrLogin{
		Id:        l.ID,
		Status:    l.CurrentStatus(),
		UserAgent: l.UserAgent,
		Ip:        l.IP,
		CreatedAt: l.CreatedAt.UTC().Format(util.ISO8601),
		ExpiresAt: l.ExpiresAt.UTC().Format(util.ISO8601),
	}
	if l.Status == statedb.QRLoginStatusApproved {
		out.Authorization = &placestream.LoginDefs_AuthorizationResponse{
			Code:  l.Code,
			State: l.State,
			Iss:   l.Iss,
		}
	}
	return out
}
