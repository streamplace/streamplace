package spxrpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/access"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

const (
	testStreamer = "did:plc:streamerstreamerstream"
	testNodeHost = "node.example"
	testNodeDID  = "did:web:node.example"
)

type serviceAuthFixture struct {
	s    *Server
	key  atcrypto.PrivateKey
	sdb  *statedb.StatefulDB
	fake *fakeAccess
}

func newServiceAuthFixture(t *testing.T) *serviceAuthFixture {
	t.Helper()
	key, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	pub, err := key.PublicKey()
	require.NoError(t, err)
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID:    syntax.DID(testStreamer),
		Handle: syntax.Handle("streamer.example"),
		Keys: map[string]identity.VerificationMethod{
			"atproto": {Type: "Multikey", PublicKeyMultibase: pub.Multibase()},
		},
	})
	sdb, err := statedb.MakeDB(context.Background(), &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	fa := &fakeAccess{viewerMode: access.ModeOpen, viewers: map[string]bool{}}
	s := &Server{
		cli:         &config.CLI{BroadcasterHost: "broadcaster.example", ServerHost: testNodeHost, Access: fa},
		statefulDB:  sdb,
		userAuthDir: &dir,
	}
	return &serviceAuthFixture{s: s, key: key, sdb: sdb, fake: fa}
}

func (f *serviceAuthFixture) token(t *testing.T, aud, method string, ttl time.Duration) string {
	t.Helper()
	return signServiceAuth(t, testStreamer, aud, method, ttl, f.key)
}

// signServiceAuth makes a service-auth JWT the way a PDS does
// (com.atproto.server.getServiceAuth). Not indigo's auth.SignServiceAuth:
// importing that package takes over golang-jwt's ES256 and breaks DPoP.
func signServiceAuth(t *testing.T, iss, aud any, method string, ttl time.Duration, key atcrypto.PrivateKey) string {
	t.Helper()
	alg := "ES256K"
	if _, ok := key.(*atcrypto.PrivateKeyP256); ok {
		alg = "ES256"
	}
	now := time.Now()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signing := enc(map[string]any{"typ": "JWT", "alg": alg}) + "." + enc(map[string]any{
		"iss": iss, "aud": aud, "lxm": method, "jti": "test",
		"iat": now.Unix(), "exp": now.Add(ttl).Unix(),
	})
	sig, err := key.HashAndSign([]byte(signing))
	require.NoError(t, err)
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// callerFor runs path through the middleware and reports who it attributed
// the request to ("" for anonymous).
func (f *serviceAuthFixture) callerFor(t *testing.T, path, authHeader string) string {
	t.Helper()
	e := echo.New()
	e.Use(f.s.UserServiceAuthMiddleware())
	e.Any("/*", func(c echo.Context) error {
		if usa := GetUserServiceAuth(c.Request().Context()); usa != nil {
			return c.String(http.StatusOK, usa.DID)
		}
		return c.String(http.StatusOK, "")
	})
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestUserServiceAuthMiddleware(t *testing.T) {
	f := newServiceAuthFixture(t)
	start := "place.stream.live.startLivestream"
	path := "/xrpc/" + start

	t.Run("valid token for this node and method", func(t *testing.T) {
		require.Equal(t, testStreamer, f.callerFor(t, path, "Bearer "+f.token(t, testNodeDID, start, time.Minute)))
	})
	t.Run("the broadcaster's DID is an accepted audience too", func(t *testing.T) {
		require.Equal(t, testStreamer, f.callerFor(t, path, "Bearer "+f.token(t, "did:web:broadcaster.example", start, time.Minute)))
	})
	t.Run("a host with a port in did:web's encoding", func(t *testing.T) {
		f.s.cli.ServerHost = "localhost:38080"
		defer func() { f.s.cli.ServerHost = testNodeHost }()
		require.Equal(t, testStreamer, f.callerFor(t, path, "Bearer "+f.token(t, "did:web:localhost%3A38080", start, time.Minute)))
		require.Equal(t, testStreamer, f.callerFor(t, path, "Bearer "+f.token(t, "did:web:localhost:38080", start, time.Minute)))
	})
	t.Run("token for another method", func(t *testing.T) {
		tok := f.token(t, testNodeDID, "place.stream.live.stopLivestream", time.Minute)
		require.Empty(t, f.callerFor(t, path, "Bearer "+tok))
	})
	t.Run("token for another service", func(t *testing.T) {
		require.Empty(t, f.callerFor(t, path, "Bearer "+f.token(t, "did:web:elsewhere.example", start, time.Minute)))
	})
	t.Run("expired token", func(t *testing.T) {
		require.Empty(t, f.callerFor(t, path, "Bearer "+f.token(t, testNodeDID, start, -time.Minute)))
	})
	t.Run("token signed by a key that is not the account's", func(t *testing.T) {
		other, err := atcrypto.GeneratePrivateKeyK256()
		require.NoError(t, err)
		tok := signServiceAuth(t, testStreamer, testNodeDID, start, time.Minute, other)
		require.Empty(t, f.callerFor(t, path, "Bearer "+tok))
	})
	t.Run("aud as an array", func(t *testing.T) {
		tok := signServiceAuth(t, testStreamer, []string{"did:web:elsewhere.example", testNodeDID}, start, time.Minute, f.key)
		require.Equal(t, testStreamer, f.callerFor(t, path, "Bearer "+tok))
	})
	t.Run("a P-256 account key (ES256)", func(t *testing.T) {
		p256, err := atcrypto.GeneratePrivateKeyP256()
		require.NoError(t, err)
		pub, err := p256.PublicKey()
		require.NoError(t, err)
		did := "did:plc:p256p256p256p256p256p256"
		f.s.userAuthDir.(*identity.MockDirectory).Insert(identity.Identity{
			DID: syntax.DID(did), Handle: syntax.Handle("p256.example"),
			Keys: map[string]identity.VerificationMethod{"atproto": {Type: "Multikey", PublicKeyMultibase: pub.Multibase()}},
		})
		require.Equal(t, did, f.callerFor(t, path, "Bearer "+signServiceAuth(t, did, testNodeDID, start, time.Minute, p256)))
	})
	t.Run("not a JWT, or not a bearer at all", func(t *testing.T) {
		require.Empty(t, f.callerFor(t, path, "Bearer not-a-jwt"))
		require.Empty(t, f.callerFor(t, path, "DPoP whatever"))
		require.Empty(t, f.callerFor(t, path, ""))
	})
	t.Run("only XRPC paths", func(t *testing.T) {
		require.Empty(t, f.callerFor(t, "/api/websocket/x", "Bearer "+f.token(t, testNodeDID, start, time.Minute)))
	})
}

func requireHTTPErrorCode(t *testing.T, err error, code int, contains string) {
	t.Helper()
	var he *echo.HTTPError
	require.True(t, errors.As(err, &he), "want an HTTP error, got: %v", err)
	require.Equal(t, code, he.Code)
	require.Contains(t, he.Message, contains)
}

func withUserServiceAuth(did string) context.Context {
	return context.WithValue(context.Background(), userServiceAuthContextKey, &UserServiceAuth{DID: did})
}

func TestStartStopLivestreamWithServiceAuth(t *testing.T) {
	f := newServiceAuthFixture(t)

	t.Run("anonymous callers still need a session", func(t *testing.T) {
		_, err := f.s.handlePlaceStreamLiveStartLivestream(context.Background(), &placestream.LiveStartLivestream_Input{Streamer: testStreamer})
		requireHTTPErrorCode(t, err, http.StatusUnauthorized, "not authorized")
		_, err = f.s.handlePlaceStreamLiveStopLivestream(context.Background(), &placestream.LiveStopLivestream_Input{})
		requireHTTPErrorCode(t, err, http.StatusUnauthorized, "oauth session required")
	})

	ctx := withUserServiceAuth(testStreamer)
	t.Run("a token for someone else cannot start their stream", func(t *testing.T) {
		_, err := f.s.handlePlaceStreamLiveStartLivestream(ctx, &placestream.LiveStartLivestream_Input{Streamer: "did:plc:someoneelse"})
		requireHTTPErrorCode(t, err, http.StatusForbidden, "not the streamer")
	})
	t.Run("without a stored session the node asks for one", func(t *testing.T) {
		_, err := f.s.handlePlaceStreamLiveStartLivestream(ctx, &placestream.LiveStartLivestream_Input{Streamer: testStreamer})
		requireHTTPErrorCode(t, err, http.StatusForbidden, "NodeSessionRequired")
		_, err = f.s.handlePlaceStreamLiveStopLivestream(ctx, &placestream.LiveStopLivestream_Input{})
		requireHTTPErrorCode(t, err, http.StatusForbidden, "NodeSessionRequired")
	})
	t.Run("with a stored session the node acts for the streamer", func(t *testing.T) {
		require.NoError(t, f.sdb.CreateOAuthSession("jkt-streamer", &oatproxy.OAuthSession{
			DID:               testStreamer,
			DownstreamDPoPJKT: "jkt-streamer",
			DownstreamScope:   "atproto",
			UpstreamScope:     "atproto",
		}))
		// Past the auth checks, the node goes to build the stored session's
		// client; this server has no OAuth proxy, so that is where it stops.
		_, err := f.s.handlePlaceStreamLiveStartLivestream(ctx, &placestream.LiveStartLivestream_Input{Streamer: testStreamer})
		requireHTTPErrorCode(t, err, http.StatusInternalServerError, "error getting streamer client")
		_, err = f.s.handlePlaceStreamLiveStopLivestream(ctx, &placestream.LiveStopLivestream_Input{})
		requireHTTPErrorCode(t, err, http.StatusInternalServerError, "error getting streamer client")
	})
}

func TestPostClient(t *testing.T) {
	f := newServiceAuthFixture(t)
	ctx := context.Background()
	post := "atproto repo?collection=app.bsky.feed.post&action=create"
	own := &xrpc.Client{Host: "https://pds.example"}

	// The caller's own OAuth session decides, and posts with its client.
	require.Equal(t, statedb.UserClient(own), f.s.postClient(ctx, testStreamer, &oatproxy.OAuthSession{DownstreamScope: post, UpstreamScope: post}, own))
	require.Nil(t, f.s.postClient(ctx, testStreamer, &oatproxy.OAuthSession{DownstreamScope: "atproto", UpstreamScope: "atproto"}, own))

	// No caller session (service auth, a peer node), no stored session.
	require.Nil(t, f.s.postClient(ctx, testStreamer, nil, own))

	// Development credentials carry every scope: they log in at the
	// account's PDS, which this node hasn't indexed, so no client here --
	// but the attempt is made rather than refused for lack of scope.
	f.s.cli.DevAccountCreds = map[string]string{"did:plc:dev": "pw"}
	require.Nil(t, f.s.postClient(ctx, "did:plc:dev", nil, own))
}

func TestGetStatusWithServiceAuth(t *testing.T) {
	f := newServiceAuthFixture(t)

	out, err := f.s.handlePlaceStreamAccessGetStatus(context.Background(), "")
	require.NoError(t, err)
	require.Nil(t, out.Did)
	require.Nil(t, out.StoredSession, "nothing to report for an anonymous caller")

	ctx := withUserServiceAuth(testStreamer)
	out, err = f.s.handlePlaceStreamAccessGetStatus(ctx, "")
	require.NoError(t, err)
	require.Equal(t, testStreamer, *out.Did)
	require.NotNil(t, out.StoredSession)
	require.False(t, *out.StoredSession)

	require.NoError(t, f.sdb.CreateOAuthSession("jkt-status", &oatproxy.OAuthSession{
		DID: testStreamer, DownstreamDPoPJKT: "jkt-status", DownstreamScope: "atproto", UpstreamScope: "atproto",
	}))
	out, err = f.s.handlePlaceStreamAccessGetStatus(ctx, "")
	require.NoError(t, err)
	require.True(t, *out.StoredSession)
}

func TestAccessGateWithServiceAuth(t *testing.T) {
	f := newServiceAuthFixture(t)
	f.fake.viewerMode = access.ModeAllowlist
	f.fake.viewers[testStreamer] = true

	run := func(t *testing.T, header string) *httptest.ResponseRecorder {
		e := echo.New()
		e.Use(f.s.UserServiceAuthMiddleware())
		e.Use(f.s.AccessGateMiddleware())
		e.Any("/xrpc/*", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
		req := httptest.NewRequest(http.MethodGet, "/xrpc/place.stream.ingest.getIngestUrls", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusUnauthorized, run(t, "").Code)
	rec := run(t, "Bearer "+f.token(t, testNodeDID, "place.stream.ingest.getIngestUrls", time.Minute))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Values("Set-Cookie"), "a per-call token is no browser session")
	require.Empty(t, f.fake.minted)
}
