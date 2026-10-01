package spxrpc

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
)

const (
	pdsAuthTestHost   = "stream.example"
	pdsAuthTestUser   = syntax.DID("did:plc:pdsauthtestuser")
	pdsAuthTestMethod = "place.stream.multistream.listTargets"
)

func newPDSAuthTestServer(t *testing.T) (*echo.Echo, atcrypto.PrivateKey) {
	t.Helper()
	priv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	pub, err := priv.PublicKey()
	require.NoError(t, err)

	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID:    pdsAuthTestUser,
		Handle: syntax.Handle("pdsauth.test"),
		Keys: map[string]identity.VerificationMethod{
			"atproto": {Type: "Multikey", PublicKeyMultibase: pub.Multibase()},
		},
	})

	s := &Server{
		cli:               &config.CLI{BroadcasterHost: pdsAuthTestHost, ServerHost: pdsAuthTestHost},
		identityDirectory: func() identity.Directory { return &dir },
	}
	e := echo.New()
	e.Use(s.PDSAuthMiddleware())
	e.GET("/xrpc/"+pdsAuthTestMethod, func(c echo.Context) error {
		caller := GetCaller(c.Request().Context())
		if caller == nil {
			return c.String(http.StatusUnauthorized, "")
		}
		if caller.OAuth != nil {
			return c.String(http.StatusInternalServerError, "unexpected oauth session")
		}
		return c.String(http.StatusOK, caller.DID)
	})
	return e, priv
}

func signPDSAuthTestToken(t *testing.T, priv atcrypto.PrivateKey, aud string, lxm string, ttl time.Duration) string {
	t.Helper()
	var nsid *syntax.NSID
	if lxm != "" {
		parsed := syntax.NSID(lxm)
		nsid = &parsed
	}
	token, err := auth.SignServiceAuth(pdsAuthTestUser, aud, ttl, nsid, priv)
	require.NoError(t, err)
	return token
}

func callPDSAuthTest(e *echo.Echo, header string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/xrpc/"+pdsAuthTestMethod, nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestPDSAuthMiddleware(t *testing.T) {
	e, priv := newPDSAuthTestServer(t)
	other, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)

	serviceAud := "did:web:" + pdsAuthTestHost + "#" + atproto.StreamplaceServiceID
	bareAud := "did:web:" + pdsAuthTestHost

	for _, tc := range []struct {
		name   string
		header string
		ok     bool
	}{
		{"service id audience", "Bearer " + signPDSAuthTestToken(t, priv, serviceAud, pdsAuthTestMethod, time.Minute), true},
		{"bare did audience", "Bearer " + signPDSAuthTestToken(t, priv, bareAud, pdsAuthTestMethod, time.Minute), true},
		{"no header", "", false},
		{"not a bearer token", "DPoP " + signPDSAuthTestToken(t, priv, serviceAud, pdsAuthTestMethod, time.Minute), false},
		{"foreign audience", "Bearer " + signPDSAuthTestToken(t, priv, "did:web:elsewhere.example#streamplace", pdsAuthTestMethod, time.Minute), false},
		{"other service id", "Bearer " + signPDSAuthTestToken(t, priv, bareAud+"#bsky_fg", pdsAuthTestMethod, time.Minute), false},
		{"lxm for another method", "Bearer " + signPDSAuthTestToken(t, priv, serviceAud, "place.stream.multistream.putTarget", time.Minute), false},
		{"missing lxm", "Bearer " + signPDSAuthTestToken(t, priv, serviceAud, "", time.Minute), false},
		{"expired", "Bearer " + signPDSAuthTestToken(t, priv, serviceAud, pdsAuthTestMethod, -time.Minute), false},
		{"wrong signing key", "Bearer " + signPDSAuthTestToken(t, other, serviceAud, pdsAuthTestMethod, time.Minute), false},
		{"garbage token", "Bearer not-a-jwt", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := callPDSAuthTest(e, tc.header)
			if tc.ok {
				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, pdsAuthTestUser.String(), rec.Body.String())
			} else {
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			}
		})
	}
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
