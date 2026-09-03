package spxrpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/access"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
)

// fakeAccess is an access.Manager with a fixed viewer roster.
type fakeAccess struct {
	viewerMode string
	viewers    map[string]bool
	minted     []string
}

func (f *fakeAccess) Allowed(_ context.Context, did, role string) bool {
	return role == access.RoleViewer && f.viewers[did]
}
func (f *fakeAccess) Modes() map[string]string {
	return map[string]string{access.RoleViewer: f.viewerMode}
}
func (f *fakeAccess) ListGrants(context.Context, string) ([]placestream.AccessDefs_GrantView, error) {
	return nil, nil
}
func (f *fakeAccess) CreateGrant(context.Context, string, string, string, *string) (*placestream.AccessDefs_GrantView, error) {
	return nil, nil
}
func (f *fakeAccess) DeleteGrant(context.Context, string) error { return nil }
func (f *fakeAccess) UpdatePolicy(context.Context, []placestream.AccessDefs_RoleMode) error {
	return nil
}
func (f *fakeAccess) ViewerCookie(did string) *http.Cookie {
	f.minted = append(f.minted, did)
	return &http.Cookie{Name: "sp_access", Value: did}
}
func (f *fakeAccess) ViewerFromCookie(r *http.Request) (string, bool) {
	ck, err := r.Cookie("sp_access")
	if err != nil {
		return "", false
	}
	return ck.Value, true
}

// withSession stands in for OAuthMiddleware by injecting a session DID the
// way oatproxy does, so the gate sees an authenticated caller.
func withSession(did string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if did != "" {
				c.SetRequest(c.Request().WithContext(testSessionContext(c.Request().Context(), did)))
			}
			return next(c)
		}
	}
}

func gateRequest(t *testing.T, fa *fakeAccess, did, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	s := &Server{cli: &config.CLI{Access: fa}}
	e := echo.New()
	e.Use(withSession(did))
	e.Use(s.AccessGateMiddleware())
	e.GET("/xrpc/*", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	e.ServeHTTP(rec, req)
	return rec
}

func TestAccessGate(t *testing.T) {
	const feed = "/xrpc/place.stream.live.getLiveUsers"
	const status = "/xrpc/place.stream.access.getStatus"

	t.Run("open node passes everyone", func(t *testing.T) {
		fa := &fakeAccess{viewerMode: access.ModeOpen}
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", feed).Code)
	})

	t.Run("no controller passes everyone", func(t *testing.T) {
		s := &Server{cli: &config.CLI{}}
		e := echo.New()
		e.Use(s.AccessGateMiddleware())
		e.GET("/xrpc/*", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, feed, nil))
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("private node: anonymous gets 401 except on exempt methods", func(t *testing.T) {
		fa := &fakeAccess{viewerMode: access.ModeAllowlist}
		require.Equal(t, http.StatusUnauthorized, gateRequest(t, fa, "", feed).Code)
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", status).Code)
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", "/xrpc/place.stream.broadcast.getBroadcaster").Code)
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", "/xrpc/com.atproto.identity.resolveHandle").Code)
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", "/xrpc/place.stream.server.getServerTime").Code, "clock sync precedes OAuth")
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", "/xrpc/place.stream.branding.getBranding").Code, "the wall carries the node's branding")
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", "/xrpc/place.stream.branding.getBlob").Code)
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", "/xrpc/app.bsky.actor.getProfile").Code, "profile lookups stay open for the sign-in wall")
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", "/xrpc/app.bsky.actor.searchActorsTypeahead").Code)
		require.Equal(t, http.StatusUnauthorized, gateRequest(t, fa, "", "/xrpc/app.bsky.feed.getTimeline").Code, "the rest of the upstream proxy is gated")
		require.Equal(t, http.StatusUnauthorized, gateRequest(t, fa, "", "/xrpc/place.stream.live.searchActorsTypeahead").Code, "the node's own repo index stays private")
	})

	t.Run("private node: logged in but not a viewer gets 403", func(t *testing.T) {
		fa := &fakeAccess{viewerMode: access.ModeAllowlist}
		require.Equal(t, http.StatusForbidden, gateRequest(t, fa, "did:plc:stranger", feed).Code)
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "did:plc:stranger", status).Code)
		require.Empty(t, fa.minted)
	})

	t.Run("private node: the viewer cookie stands in for a session on anonymous-agent calls", func(t *testing.T) {
		fa := &fakeAccess{viewerMode: access.ModeAllowlist, viewers: map[string]bool{"did:plc:member": true}}
		member := &http.Cookie{Name: "sp_access", Value: "did:plc:member"}
		require.Equal(t, http.StatusOK, gateRequest(t, fa, "", feed, member).Code)
		// a revoked viewer's cookie stops working immediately
		revoked := &http.Cookie{Name: "sp_access", Value: "did:plc:revoked"}
		require.Equal(t, http.StatusForbidden, gateRequest(t, fa, "", feed, revoked).Code)
		// a session, when present, is what counts; the cookie can't upgrade it
		require.Equal(t, http.StatusForbidden, gateRequest(t, fa, "did:plc:stranger", feed, member).Code)
	})

	t.Run("private node: viewer passes and gets a cookie", func(t *testing.T) {
		fa := &fakeAccess{viewerMode: access.ModeAllowlist, viewers: map[string]bool{"did:plc:member": true}}
		rec := gateRequest(t, fa, "did:plc:member", feed)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Set-Cookie"), "sp_access=did:plc:member")
		require.Equal(t, []string{"did:plc:member"}, fa.minted)
	})
}

// testSessionContext mirrors what oatproxy.OAuthMiddleware puts on the
// context for an authenticated request.
func testSessionContext(ctx context.Context, did string) context.Context {
	// GetOAuthSession also builds the upstream XRPC client, which needs a
	// parseable DPoP key on the session; any EC key will do.
	raw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	key, err := jwk.FromRaw(raw)
	if err != nil {
		panic(err)
	}
	keyBs, err := json.Marshal(key)
	if err != nil {
		panic(err)
	}
	ctx = context.WithValue(ctx, oatproxy.OATProxyContextKey, &oatproxy.OATProxy{})
	return context.WithValue(ctx, oatproxy.OAuthSessionContextKey, &oatproxy.OAuthSession{
		DID:                    did,
		UpstreamDPoPPrivateJWK: string(keyBs),
	})
}
