package spxrpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// TestQRLoginLifecycle drives attempts through the handlers against a real
// oatproxy and state database: only an approval ever exposes an
// authorization, and each attempt completes at most once, before it expires.
func TestQRLoginLifecycle(t *testing.T) {
	ctx := t.Context()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	op := oatproxy.New(&oatproxy.Config{
		Host:               "stream.example",
		CreateOAuthSession: sdb.CreateOAuthSession,
		UpdateOAuthSession: sdb.UpdateOAuthSession,
		GetOAuthSession:    sdb.LoadOAuthSession,
		Slog:               slog.New(slog.DiscardHandler),
	})
	cli := &config.CLI{BroadcasterHost: "stream.example", AppBundleID: "tv.aquareum.dev"}
	s := &Server{cli: cli, statefulDB: sdb, op: op}

	// the phone, signed in to this node
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jwkKey, err := jwk.FromRaw(key)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(jwkKey)
	require.NoError(t, err)
	phone := &oatproxy.OAuthSession{
		DID:                    "did:plc:phone",
		DownstreamDPoPJKT:      "phone-jkt",
		DownstreamAccessToken:  "phone-access",
		UpstreamDPoPPrivateJWK: string(keyJSON),
	}
	require.NoError(t, sdb.CreateOAuthSession(phone.DownstreamDPoPJKT, phone))
	approver := context.WithValue(ctx, oatproxy.OATProxyContextKey, op)
	approver = context.WithValue(approver, oatproxy.OAuthSessionContextKey, phone)

	// a browser makes its pushed authorization request, then asks for a QR code
	start := func(t *testing.T, s *Server, jkt string) (*placestream.LoginCreateQRLogin_Output, error) {
		requestURI := "urn:ietf:params:oauth:request_uri:" + uuid.NewString() + "-" + jkt
		require.NoError(t, sdb.CreateOAuthSession(jkt, &oatproxy.OAuthSession{
			DownstreamDPoPJKT:       jkt,
			DownstreamPARRequestURI: requestURI,
			DownstreamState:         jkt + "-state",
		}))
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("User-Agent", "Firefox")
		req.RemoteAddr = "203.0.113.7:1234"
		c := echo.New().NewContext(req, httptest.NewRecorder())
		return s.handlePlaceStreamLoginCreateQRLogin(context.WithValue(ctx, echoContextKey, c), &placestream.LoginCreateQRLogin_Input{RequestUri: requestURI})
	}
	complete := func(id string, approve bool) (*placestream.LoginDefs_QrLogin, error) {
		return s.handlePlaceStreamLoginCompleteQRLogin(approver, &placestream.LoginCompleteQRLogin_Input{Id: id, Approve: approve})
	}
	requireNotLinked := func(t *testing.T, jkt string) {
		browser, err := sdb.LoadOAuthSession(jkt)
		require.NoError(t, err)
		require.Equal(t, oatproxy.OAuthSessionStatePARCreated, browser.Status())
	}

	t.Run("unavailable without an app", func(t *testing.T) {
		noApp := &Server{cli: &config.CLI{BroadcasterHost: "stream.example"}, statefulDB: sdb, op: op}
		_, err := start(t, noApp, "browser-noapp")
		he := requireHTTPError(t, err)
		require.Equal(t, http.StatusNotImplemented, he.Code)
		// on the wire it's a named XRPC error, which the browser hides QR
		// login for (rather than retrying)
		e := echo.New()
		rec := httptest.NewRecorder()
		e.DefaultHTTPErrorHandler(he, e.NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec))
		var body xrpc.XRPCError
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "QRLoginUnavailable", body.ErrStr)
	})

	t.Run("approve", func(t *testing.T) {
		out, err := start(t, s, "browser-approve")
		require.NoError(t, err)
		require.Equal(t, "https://stream.example/login/qr/"+out.Id, out.Url)

		got, err := s.handlePlaceStreamLoginGetQRLogin(ctx, out.Id)
		require.NoError(t, err)
		require.Equal(t, statedb.QRLoginStatusPending, got.Status)
		require.Nil(t, got.Authorization)
		require.Equal(t, "Firefox", got.UserAgent)
		require.Equal(t, "203.0.113.7", got.Ip)

		_, err = s.handlePlaceStreamLoginCompleteQRLogin(ctx, &placestream.LoginCompleteQRLogin_Input{Id: out.Id, Approve: true})
		require.Equal(t, http.StatusUnauthorized, requireHTTPError(t, err).Code)

		done, err := complete(out.Id, true)
		require.NoError(t, err)
		require.Equal(t, statedb.QRLoginStatusApproved, done.Status)
		browser, err := sdb.LoadOAuthSession("browser-approve")
		require.NoError(t, err)
		require.Equal(t, "phone-jkt", browser.UpstreamJKT)
		want := &placestream.LoginDefs_AuthorizationResponse{
			Code:  browser.DownstreamAuthorizationCode,
			State: "browser-approve-state",
			Iss:   "https://stream.example",
		}
		require.Equal(t, want, done.Authorization)

		got, err = s.handlePlaceStreamLoginGetQRLogin(ctx, out.Id)
		require.NoError(t, err)
		require.Equal(t, statedb.QRLoginStatusApproved, got.Status)
		require.Equal(t, want, got.Authorization)

		for _, approve := range []bool{true, false} {
			_, err = complete(out.Id, approve)
			he := requireHTTPError(t, err)
			require.Equal(t, http.StatusConflict, he.Code)
			require.Equal(t, "AlreadyCompleted", he.Message)
		}
	})

	t.Run("deny", func(t *testing.T) {
		out, err := start(t, s, "browser-deny")
		require.NoError(t, err)
		done, err := complete(out.Id, false)
		require.NoError(t, err)
		require.Equal(t, statedb.QRLoginStatusDenied, done.Status)
		require.Nil(t, done.Authorization)

		_, err = complete(out.Id, true)
		require.Equal(t, http.StatusConflict, requireHTTPError(t, err).Code)
		got, err := s.handlePlaceStreamLoginGetQRLogin(ctx, out.Id)
		require.NoError(t, err)
		require.Equal(t, statedb.QRLoginStatusDenied, got.Status)
		require.Nil(t, got.Authorization)
		requireNotLinked(t, "browser-deny")
	})

	t.Run("expiry", func(t *testing.T) {
		out, err := start(t, s, "browser-expire")
		require.NoError(t, err)
		require.NoError(t, sdb.DB.Model(&statedb.QRLogin{}).
			Where("id = ?", out.Id).
			Update("expires_at", time.Now().UTC().Add(-time.Second)).Error)

		got, err := s.handlePlaceStreamLoginGetQRLogin(ctx, out.Id)
		require.NoError(t, err)
		require.Equal(t, statedb.QRLoginStatusExpired, got.Status)
		_, err = complete(out.Id, true)
		require.Equal(t, http.StatusConflict, requireHTTPError(t, err).Code)
		requireNotLinked(t, "browser-expire")
	})

	t.Run("unknown id", func(t *testing.T) {
		_, err := s.handlePlaceStreamLoginGetQRLogin(ctx, "nope")
		require.Equal(t, http.StatusNotFound, requireHTTPError(t, err).Code)
		_, err = complete("nope", true)
		require.Equal(t, http.StatusNotFound, requireHTTPError(t, err).Code)
	})
}
