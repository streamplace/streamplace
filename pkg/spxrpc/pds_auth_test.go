package spxrpc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	placestream "stream.place/streamplace/pkg/placestream"
)

const (
	pdsAuthTestHost   = "stream.example"
	pdsAuthTestUser   = syntax.DID("did:plc:pdsauthtestuser")
	pdsAuthTestOAuth  = "did:plc:oauthtestuser"
	pdsAuthTestMethod = "place.stream.multistream.listTargets"
)

var pdsAuthTestServiceAud = "did:web:" + pdsAuthTestHost + "#" + atproto.StreamplaceServiceID

func pdsAuthTestIdentity(t *testing.T, priv atcrypto.PrivateKey) identity.Identity {
	t.Helper()
	pub, err := priv.PublicKey()
	require.NoError(t, err)
	return identity.Identity{
		DID:    pdsAuthTestUser,
		Handle: syntax.Handle("pdsauth.test"),
		Keys: map[string]identity.VerificationMethod{
			"atproto": {Type: "Multikey", PublicKeyMultibase: pub.Multibase()},
		},
	}
}

func newPDSAuthTestKey(t *testing.T) atcrypto.PrivateKey {
	t.Helper()
	priv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	return priv
}

func pdsAuthTestHandler(c echo.Context) error {
	caller := GetCaller(c.Request().Context())
	if caller == nil {
		return c.String(http.StatusUnauthorized, "")
	}
	if caller.OAuth != nil {
		return c.String(http.StatusOK, "oauth:"+caller.DID)
	}
	return c.String(http.StatusOK, "pds:"+caller.DID)
}

func newPDSAuthTestServer(dir identity.Directory) *echo.Echo {
	s := &Server{
		cli:               &config.CLI{BroadcasterHost: pdsAuthTestHost, ServerHost: pdsAuthTestHost},
		identityDirectory: func() identity.Directory { return dir },
		identityRefreshes: newIdentityRefreshCache(),
	}
	e := echo.New()
	e.Use(s.PDSAuthMiddleware())
	e.GET("/xrpc/"+pdsAuthTestMethod, pdsAuthTestHandler)
	return e
}

func signPDSAuthTestToken(t *testing.T, priv atcrypto.PrivateKey, aud string, lxm string, ttl time.Duration) string {
	t.Helper()
	alg := "ES256K"
	if _, ok := priv.(*atcrypto.PrivateKeyP256); ok {
		alg = "ES256"
	}
	now := time.Now()
	claims := map[string]any{
		"iss": pdsAuthTestUser.String(),
		"aud": aud,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
		"jti": base64.RawURLEncoding.EncodeToString([]byte(now.String())),
	}
	if lxm != "" {
		claims["lxm"] = lxm
	}
	return signPDSAuthTestJWT(t, priv, map[string]any{"alg": alg, "typ": "JWT"}, claims)
}

func signPDSAuthTestJWT(t *testing.T, priv atcrypto.PrivateKey, header map[string]any, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	claimsJSON, err := json.Marshal(claims)
	require.NoError(t, err)
	input := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	sig, err := priv.HashAndSign([]byte(input))
	require.NoError(t, err)
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func callPDSAuthTest(e *echo.Echo, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "http://"+pdsAuthTestHost+"/xrpc/"+pdsAuthTestMethod, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func TestPDSAuthMiddleware(t *testing.T) {
	priv := newPDSAuthTestKey(t)
	other := newPDSAuthTestKey(t)
	dir := identity.NewMockDirectory()
	dir.Insert(pdsAuthTestIdentity(t, priv))
	e := newPDSAuthTestServer(&dir)

	p256, err := atcrypto.GeneratePrivateKeyP256()
	require.NoError(t, err)
	p256Dir := identity.NewMockDirectory()
	p256Dir.Insert(pdsAuthTestIdentity(t, p256))
	p256Server := newPDSAuthTestServer(&p256Dir)

	bareAud := "did:web:" + pdsAuthTestHost
	now := time.Now()
	validClaims := map[string]any{"iss": pdsAuthTestUser.String(), "aud": pdsAuthTestServiceAud, "lxm": pdsAuthTestMethod, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()}
	withClaim := func(k string, v any) map[string]any {
		out := map[string]any{}
		for key, val := range validClaims {
			out[key] = val
		}
		if v == nil {
			delete(out, k)
		} else {
			out[k] = v
		}
		return out
	}

	for _, tc := range []struct {
		name    string
		server  *echo.Echo
		headers map[string]string
		ok      bool
	}{
		{"p256 signing key", p256Server, bearer(signPDSAuthTestToken(t, p256, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)), true},
		{"audience list", nil, bearer(signPDSAuthTestJWT(t, priv, map[string]any{"alg": "ES256K"}, withClaim("aud", []string{"did:web:elsewhere.example", pdsAuthTestServiceAud}))), true},
		{"alg none", nil, bearer(signPDSAuthTestJWT(t, priv, map[string]any{"alg": "none"}, validClaims)), false},
		{"missing exp", nil, bearer(signPDSAuthTestJWT(t, priv, map[string]any{"alg": "ES256K"}, withClaim("exp", nil))), false},
		{"issued in the future", nil, bearer(signPDSAuthTestJWT(t, priv, map[string]any{"alg": "ES256K"}, withClaim("iat", now.Add(time.Hour).Unix()))), false},
		{"other issuer", nil, bearer(signPDSAuthTestJWT(t, priv, map[string]any{"alg": "ES256K"}, withClaim("iss", "did:plc:someoneelse"))), false},
		{"service id audience", nil, bearer(signPDSAuthTestToken(t, priv, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)), true},
		{"bare did audience", nil, bearer(signPDSAuthTestToken(t, priv, bareAud, pdsAuthTestMethod, time.Minute)), true},
		{"no header", nil, nil, false},
		{"not a bearer token", nil, map[string]string{"Authorization": "DPoP " + signPDSAuthTestToken(t, priv, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)}, false},
		{"foreign audience", nil, bearer(signPDSAuthTestToken(t, priv, "did:web:elsewhere.example#streamplace", pdsAuthTestMethod, time.Minute)), false},
		{"other service id", nil, bearer(signPDSAuthTestToken(t, priv, bareAud+"#bsky_fg", pdsAuthTestMethod, time.Minute)), false},
		{"lxm for another method", nil, bearer(signPDSAuthTestToken(t, priv, pdsAuthTestServiceAud, "place.stream.multistream.putTarget", time.Minute)), false},
		{"missing lxm", nil, bearer(signPDSAuthTestToken(t, priv, pdsAuthTestServiceAud, "", time.Minute)), false},
		{"expired", nil, bearer(signPDSAuthTestToken(t, priv, pdsAuthTestServiceAud, pdsAuthTestMethod, -time.Minute)), false},
		{"wrong signing key", nil, bearer(signPDSAuthTestToken(t, other, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)), false},
		{"garbage token", nil, bearer("not-a-jwt"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.server
			if server == nil {
				server = e
			}
			rec := callPDSAuthTest(server, tc.headers)
			if tc.ok {
				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, "pds:"+pdsAuthTestUser.String(), rec.Body.String())
			} else {
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			}
		})
	}
}

type rotatingDirectory struct {
	identity.MockDirectory
	mu      sync.Mutex
	rotated identity.Identity
	purges  int
}

func (d *rotatingDirectory) Purge(ctx context.Context, a syntax.AtIdentifier) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.purges++
	d.Insert(d.rotated)
	return nil
}

func TestPDSAuthMiddlewareKeyRotation(t *testing.T) {
	stale := newPDSAuthTestKey(t)
	rotated := newPDSAuthTestKey(t)
	dir := &rotatingDirectory{MockDirectory: identity.NewMockDirectory(), rotated: pdsAuthTestIdentity(t, rotated)}
	dir.Insert(pdsAuthTestIdentity(t, stale))
	e := newPDSAuthTestServer(dir)

	rec := callPDSAuthTest(e, bearer(signPDSAuthTestToken(t, rotated, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "pds:"+pdsAuthTestUser.String(), rec.Body.String())
	require.Equal(t, 1, dir.purges)

	rec = callPDSAuthTest(e, bearer(signPDSAuthTestToken(t, stale, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPDSAuthMiddlewareThrottlesIdentityRefresh(t *testing.T) {
	victim := newPDSAuthTestKey(t)
	forger := newPDSAuthTestKey(t)
	dir := &rotatingDirectory{MockDirectory: identity.NewMockDirectory(), rotated: pdsAuthTestIdentity(t, victim)}
	dir.Insert(pdsAuthTestIdentity(t, victim))
	e := newPDSAuthTestServer(dir)

	for range 20 {
		rec := callPDSAuthTest(e, bearer(signPDSAuthTestToken(t, forger, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}
	require.Equal(t, 1, dir.purges)

	rec := callPDSAuthTest(e, bearer(signPDSAuthTestToken(t, victim, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, dir.purges)
}

func TestPDSAuthCallerCannotTriggerIndependentWrites(t *testing.T) {
	ctx := context.WithValue(context.Background(), pdsAuthContextKey, pdsAuthTestUser.String())
	require.NotNil(t, GetCaller(ctx))
	s := &Server{cli: &config.CLI{BroadcasterHost: pdsAuthTestHost, ServerHost: pdsAuthTestHost}}

	requireUnauthorized := func(t *testing.T, err error) {
		t.Helper()
		var he *echo.HTTPError
		require.ErrorAs(t, err, &he)
		require.Equal(t, http.StatusUnauthorized, he.Code)
	}

	t.Run("publishDraft", func(t *testing.T) {
		_, err := s.handlePlaceStreamVodPublishDraft(ctx, &placestream.VodPublishDraft_Input{Uri: "at://did:plc:pdsauthtestuser/place.stream.vod.draftVideo/abc"})
		requireUnauthorized(t, err)
	})
	t.Run("createUpload", func(t *testing.T) {
		_, err := s.handlePlaceStreamMediaCreateUpload(ctx, &placestream.MediaCreateUpload_Input{})
		requireUnauthorized(t, err)
	})
	t.Run("delegated moderation", func(t *testing.T) {
		_, err := s.GetDelegatedModerationContext(ctx, pdsAuthTestUser.String(), "livestream.manage")
		requireUnauthorized(t, err)
	})
}

type oauthTestClient struct {
	key   *ecdsa.PrivateKey
	pub   jwk.Key
	jkt   string
	token string
}

func newOAuthTestStack(t *testing.T, pdsDir identity.Directory) (*echo.Echo, *oauthTestClient) {
	t.Helper()
	downstreamRaw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	downstream, err := jwk.FromRaw(downstreamRaw)
	require.NoError(t, err)

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientPub, err := jwk.FromRaw(clientKey.Public())
	require.NoError(t, err)
	thumb, err := clientPub.Thumbprint(crypto.SHA256)
	require.NoError(t, err)
	jkt := base64.RawURLEncoding.EncodeToString(thumb)

	upstreamRaw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	upstream, err := jwk.FromRaw(upstreamRaw)
	require.NoError(t, err)
	upstreamJSON, err := json.Marshal(upstream)
	require.NoError(t, err)

	now := time.Now()
	access, err := jwt.NewBuilder().
		JwtID("access-token").
		Subject(pdsAuthTestOAuth).
		Issuer("https://"+pdsAuthTestHost).
		Audience([]string{"did:web:" + pdsAuthTestHost}).
		IssuedAt(now).
		NotBefore(now).
		Expiration(now.Add(time.Hour)).
		Claim("cnf", map[string]any{"jkt": jkt}).
		Build()
	require.NoError(t, err)
	signedAccess, err := jwt.Sign(access, jwt.WithKey(jwa.ES256, downstream))
	require.NoError(t, err)

	upstreamExp := now.Add(time.Hour)
	var mu sync.Mutex
	sessions := map[string]*oatproxy.OAuthSession{
		jkt: {
			DID:                         pdsAuthTestOAuth,
			PDSUrl:                      "https://pds.example",
			DownstreamDPoPJKT:           jkt,
			DownstreamDPoPNoncePad:      "noncepad-test",
			DownstreamAccessToken:       string(signedAccess),
			UpstreamDPoPPrivateJWK:      string(upstreamJSON),
			UpstreamAccessTokenExp:      &upstreamExp,
			UpstreamAccessTokenLifetime: 3600,
		},
	}
	op := oatproxy.New(&oatproxy.Config{
		Host:          pdsAuthTestHost,
		DownstreamJWK: downstream,
		GetOAuthSession: func(id string) (*oatproxy.OAuthSession, error) {
			mu.Lock()
			defer mu.Unlock()
			return sessions[id], nil
		},
		UpdateOAuthSession: func(id string, session *oatproxy.OAuthSession) error {
			mu.Lock()
			defer mu.Unlock()
			sessions[id] = session
			return nil
		},
	})

	s := &Server{
		cli:               &config.CLI{BroadcasterHost: pdsAuthTestHost, ServerHost: pdsAuthTestHost},
		identityDirectory: func() identity.Directory { return pdsDir },
		identityRefreshes: newIdentityRefreshCache(),
		op:                op,
	}
	e := echo.New()
	s.useAuthMiddleware(e)
	e.GET("/xrpc/"+pdsAuthTestMethod, pdsAuthTestHandler)
	return e, &oauthTestClient{key: clientKey, pub: clientPub, jkt: jkt, token: string(signedAccess)}
}

func (c *oauthTestClient) proof(t *testing.T, nonce string) string {
	t.Helper()
	ath := sha256.Sum256([]byte(c.token))
	claims, err := jwt.NewBuilder().
		JwtID(base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))).
		IssuedAt(time.Now()).
		Claim("htm", http.MethodGet).
		Claim("htu", "https://"+pdsAuthTestHost+"/xrpc/"+pdsAuthTestMethod).
		Claim("nonce", nonce).
		Claim("ath", base64.RawURLEncoding.EncodeToString(ath[:])).
		Build()
	require.NoError(t, err)
	hdrs := jws.NewHeaders()
	require.NoError(t, hdrs.Set(jws.TypeKey, "dpop+jwt"))
	require.NoError(t, hdrs.Set(jws.JWKKey, c.pub))
	signed, err := jwt.Sign(claims, jwt.WithKey(jwa.ES256, c.key, jws.WithProtectedHeaders(hdrs)))
	require.NoError(t, err)
	return string(signed)
}

func (c *oauthTestClient) call(t *testing.T, e *echo.Echo) *httptest.ResponseRecorder {
	t.Helper()
	rec := callPDSAuthTest(e, map[string]string{"Authorization": "DPoP " + c.token, "DPoP": c.proof(t, "")})
	nonce := rec.Header().Get("DPoP-Nonce")
	require.NotEmpty(t, nonce)
	return callPDSAuthTest(e, map[string]string{"Authorization": "DPoP " + c.token, "DPoP": c.proof(t, nonce)})
}

func TestAuthMiddlewareStack(t *testing.T) {
	priv := newPDSAuthTestKey(t)
	dir := identity.NewMockDirectory()
	dir.Insert(pdsAuthTestIdentity(t, priv))
	e, client := newOAuthTestStack(t, &dir)

	t.Run("oatproxy session", func(t *testing.T) {
		rec := client.call(t, e)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "oauth:"+pdsAuthTestOAuth, rec.Body.String())
	})

	t.Run("pds service auth", func(t *testing.T) {
		rec := callPDSAuthTest(e, bearer(signPDSAuthTestToken(t, priv, pdsAuthTestServiceAud, pdsAuthTestMethod, time.Minute)))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "pds:"+pdsAuthTestUser.String(), rec.Body.String())
	})

	t.Run("unknown dpop key", func(t *testing.T) {
		other := *client
		otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		other.key = otherKey
		other.pub, err = jwk.FromRaw(otherKey.Public())
		require.NoError(t, err)
		rec := callPDSAuthTest(e, map[string]string{"Authorization": "DPoP " + other.token, "DPoP": other.proof(t, "")})
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.Empty(t, rec.Body.String())
	})

	t.Run("no credentials", func(t *testing.T) {
		rec := callPDSAuthTest(e, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.Empty(t, rec.Body.String())
	})
}

func TestDIDDocAdvertisesStreamplaceService(t *testing.T) {
	doc := atproto.DIDDoc(pdsAuthTestHost, "zQ3shRvJrnKp4SWvoxkFyZqijahHcmvJLdViKzkwQKj7cWxVm")
	services, ok := doc["service"].([]map[string]any)
	require.True(t, ok)
	require.Contains(t, services, map[string]any{
		"id":              "#" + atproto.StreamplaceServiceID,
		"type":            "StreamplaceServer",
		"serviceEndpoint": "https://" + pdsAuthTestHost,
	})
}
