package spxrpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/stationshard"
)

const (
	stationCreatorTestNSID     = "place.stream.multistream.createTarget"
	stationCreatorTestPath     = "/xrpc/" + stationCreatorTestNSID
	stationCreatorTestPlayback = "/xrpc/place.stream.playback.getVideoPlaylist"
	stationCreatorTestIngress  = "ingress.example"
	stationCreatorTestOwner    = "owner.example"
	stationCreatorIngressDID   = "did:web:" + stationCreatorTestIngress
	stationCreatorOwnerDID     = "did:web:" + stationCreatorTestOwner
)

type stationCreatorTestSlots struct {
	owner string
	route string

	mu      sync.Mutex
	lookups []string
	routes  []string
}

func (s *stationCreatorTestSlots) Owner(_ context.Context, did string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookups = append(s.lookups, did)
	if s.owner == "" {
		return "", stationshard.ErrNotFound
	}
	return s.owner, nil
}

func (s *stationCreatorTestSlots) Route(_ context.Context, did string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = append(s.routes, did)
	return s.route, nil
}

func (s *stationCreatorTestSlots) calls() (lookups, routes []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.lookups...), append([]string(nil), s.routes...)
}

type stationCreatorTestDirectory struct {
	*identity.MockDirectory
	lookups atomic.Int32
}

func (d *stationCreatorTestDirectory) LookupDID(ctx context.Context, did syntax.DID) (*identity.Identity, error) {
	d.lookups.Add(1)
	return d.MockDirectory.LookupDID(ctx, did)
}

type stationCreatorTestNode struct {
	cli          *config.CLI
	slots        *stationCreatorTestSlots
	directory    *stationCreatorTestDirectory
	oauthLookups atomic.Int32
	requests     atomic.Int32
	redirects    atomic.Int32
	writes       atomic.Int32
}

type stationCreatorTestReceipt struct {
	Actor, Peer, Method, RawQuery, Body, ContentType string
	OAuth                                            bool
}

type stationCreatorTestPair struct {
	ingress, owner *stationCreatorTestNode
	entry          *httptest.Server
	oauth          *oauthTestClient
	pdsKey         atcrypto.PrivateKey
}

func newStationCreatorTestPair(t *testing.T, ingressSlots, ownerSlots *stationCreatorTestSlots) *stationCreatorTestPair {
	t.Helper()
	key, err := jwk.FromRaw([]byte("station creator routing shared test key"))
	require.NoError(t, err)
	pdsKey := newPDSAuthTestKey(t)
	pdsIdentity := pdsAuthTestIdentity(t, pdsKey)
	if ingressSlots == nil {
		ingressSlots = &stationCreatorTestSlots{owner: stationCreatorOwnerDID, route: stationCreatorIngressDID}
	}
	if ownerSlots == nil {
		ownerSlots = &stationCreatorTestSlots{owner: stationCreatorOwnerDID, route: stationCreatorOwnerDID}
	}
	newNode := func(host string, slots *stationCreatorTestSlots) *stationCreatorTestNode {
		dir := &stationCreatorTestDirectory{MockDirectory: identity.NewMockDirectory()}
		dir.Insert(pdsIdentity)
		return &stationCreatorTestNode{
			cli: &config.CLI{
				BroadcasterHost: "broadcast.example",
				ServerHost:      host,
				StationDID:      "did:web:" + pdsAuthTestHost,
				ServiceAuthKey:  key,
			},
			slots:     slots,
			directory: dir,
		}
	}
	ingress := newNode(stationCreatorTestIngress, ingressSlots)
	owner := newNode(stationCreatorTestOwner, ownerSlots)
	ingressEcho, ownerEcho := echo.New(), echo.New()
	ready := make(chan struct{})
	defer close(ready)
	serve := func(e *echo.Echo, node *stationCreatorTestNode) *httptest.Server {
		return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-ready
			node.requests.Add(1)
			e.ServeHTTP(w, r)
		}))
	}
	entry := serve(ingressEcho, ingress)
	t.Cleanup(entry.Close)
	destination := serve(ownerEcho, owner)
	t.Cleanup(destination.Close)
	setup := func(e *echo.Echo, node, peer *stationCreatorTestNode, peerServer *httptest.Server) *oauthTestClient {
		endpoint, err := url.Parse(peerServer.URL)
		require.NoError(t, err)
		peerClient := *peerServer.Client()
		peerClient.Timeout = 2 * time.Second
		op, client := newOAuthTestProxy(t, func() { node.oauthLookups.Add(1) })
		s := &Server{
			cli:               node.cli,
			identityDirectory: func() identity.Directory { return node.directory },
			identityRefreshes: newIdentityRefreshCache(),
			op:                op,
			creatorRoutes: &creatorRouter{
				slots:  node.slots,
				peers:  map[string]*url.URL{peer.cli.ServerDID(): endpoint},
				client: &peerClient,
			},
		}
		s.useAuthMiddleware(e)
		e.POST(stationCreatorTestPath, func(c echo.Context) error {
			r := c.Request()
			caller := GetCaller(r.Context())
			if caller == nil {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			if c.QueryParam("redirect") == "1" {
				return c.Redirect(http.StatusSeeOther, "/redirected")
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return err
			}
			receipt := stationCreatorTestReceipt{
				Actor: caller.DID, OAuth: caller.OAuth != nil, Method: r.Method,
				RawQuery: r.URL.RawQuery, Body: string(body), ContentType: r.Header.Get("Content-Type"),
			}
			if service := GetServiceAuth(r.Context()); service != nil {
				receipt.Peer = service.DID
			}
			node.writes.Add(1)
			return c.JSON(http.StatusCreated, receipt)
		})
		e.GET(stationCreatorTestPlayback, func(c echo.Context) error {
			return c.String(http.StatusOK, node.cli.ServerDID())
		})
		e.Any("/redirected", func(c echo.Context) error {
			node.redirects.Add(1)
			return c.NoContent(http.StatusNoContent)
		})
		return client
	}
	setup(ingressEcho, ingress, owner, destination)
	oauth := setup(ownerEcho, owner, ingress, entry)
	entry.Client().Timeout = 3 * time.Second
	entry.Client().CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &stationCreatorTestPair{ingress: ingress, owner: owner, entry: entry, oauth: oauth, pdsKey: pdsKey}
}

func (p *stationCreatorTestPair) request(t *testing.T, method, target, body string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, p.entry.URL+target, strings.NewReader(body))
	require.NoError(t, err)
	req.Host = pdsAuthTestHost
	if headers != nil {
		req.Header = headers.Clone()
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := p.entry.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	rec := httptest.NewRecorder()
	for name, values := range res.Header {
		rec.Header()[name] = values
	}
	rec.WriteHeader(res.StatusCode)
	_, err = io.Copy(rec, res.Body)
	require.NoError(t, err)
	return rec
}

func (p *stationCreatorTestPair) bearer(t *testing.T, audience, method string) http.Header {
	t.Helper()
	token := signPDSAuthTestToken(t, p.pdsKey, audience, method, time.Minute)
	return http.Header{echo.HeaderAuthorization: {"Bearer " + token}}
}

func (p *stationCreatorTestPair) requireNoWrites(t *testing.T) {
	t.Helper()
	require.Zero(t, p.ingress.writes.Load())
	require.Zero(t, p.owner.writes.Load())
}

func (p *stationCreatorTestPair) requireNoClaims(t *testing.T) {
	t.Helper()
	_, ingressRoutes := p.ingress.slots.calls()
	require.Empty(t, ingressRoutes)
	_, ownerRoutes := p.owner.slots.calls()
	require.Empty(t, ownerRoutes)
	p.requireNoWrites(t)
}

func TestStationCreatorRouteForward(t *testing.T) {
	const rawQuery = "z=%2f&repeat=first&repeat=second&blank=&a=one+two"
	const body = "{ \"multistreamTarget\": {\"url\":\"rtmps://example.test/live\"} }\n"
	target := stationCreatorTestPath + "?" + rawQuery

	for _, auth := range []string{"OAuth", "PDS"} {
		t.Run(auth, func(t *testing.T) {
			p := newStationCreatorTestPair(t, nil, nil)
			var rec *httptest.ResponseRecorder
			actor := pdsAuthTestUser.String()
			ownerRequests := int32(1)
			if auth == "OAuth" {
				actor = pdsAuthTestOAuth
				ownerRequests = 2
				call := func(nonce string) *httptest.ResponseRecorder {
					return p.request(t, http.MethodPost, target, body, http.Header{
						echo.HeaderAuthorization: {"DPoP " + p.oauth.token},
						"DPoP":                   {p.oauth.proofFor(t, http.MethodPost, "https://"+pdsAuthTestHost+stationCreatorTestPath, nonce)},
					})
				}
				challenge := call("")
				require.Equal(t, http.StatusUnauthorized, challenge.Code, challenge.Body.String())
				require.Contains(t, challenge.Header().Get("WWW-Authenticate"), "use_dpop_nonce")
				nonce := challenge.Header().Get("DPoP-Nonce")
				require.NotEmpty(t, nonce)
				require.EqualValues(t, 1, p.owner.oauthLookups.Load())
				p.requireNoClaims(t)
				rec = call(nonce)
				require.NotEmpty(t, rec.Header().Get("DPoP-Nonce"))
				require.EqualValues(t, 2, p.owner.oauthLookups.Load(), "one OAuth validation per attempt")
				require.Zero(t, p.owner.directory.lookups.Load())
			} else {
				rec = p.request(t, http.MethodPost, target, body, p.bearer(t, p.ingress.cli.ServerDID(), stationCreatorTestNSID))
				require.EqualValues(t, 1, p.owner.directory.lookups.Load(), "PDS token must be validated at the owner")
				require.Zero(t, p.owner.oauthLookups.Load())
			}

			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			var got stationCreatorTestReceipt
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			require.Equal(t, stationCreatorTestReceipt{
				Actor: actor, Peer: p.ingress.cli.ServerDID(), Method: http.MethodPost,
				RawQuery: rawQuery, Body: body, ContentType: "application/json", OAuth: auth == "OAuth",
			}, got)
			require.Zero(t, p.ingress.oauthLookups.Load(), "ingress must not process the DPoP proof")
			require.Zero(t, p.ingress.directory.lookups.Load(), "ingress must not validate the PDS token")
			require.Zero(t, p.ingress.writes.Load())
			require.EqualValues(t, 1, p.owner.writes.Load())
			require.Equal(t, ownerRequests, p.owner.requests.Load())
			_, ingressRoutes := p.ingress.slots.calls()
			require.Empty(t, ingressRoutes)
			_, ownerRoutes := p.owner.slots.calls()
			require.Equal(t, []string{actor}, ownerRoutes)
		})
	}
}

func TestStationCreatorRouteRejectsBadAuthBeforeClaim(t *testing.T) {
	for _, tc := range []struct {
		name     string
		slots    *stationCreatorTestSlots
		peerHits int32
	}{
		{name: "unassigned", slots: &stationCreatorTestSlots{route: stationCreatorIngressDID}},
		{name: "remote owner", peerHits: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newStationCreatorTestPair(t, tc.slots, nil)
			// The claims are parseable and name a real actor, but the key is wrong.
			token := signPDSAuthTestToken(t, newPDSAuthTestKey(t), p.ingress.cli.ServerDID(), stationCreatorTestNSID, time.Minute)
			rec := p.request(t, http.MethodPost, stationCreatorTestPath, "{}", http.Header{
				echo.HeaderAuthorization: {"Bearer " + token},
			})
			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
			require.Equal(t, tc.peerHits, p.owner.requests.Load())
			lookups, _ := p.ingress.slots.calls()
			require.Equal(t, []string{pdsAuthTestUser.String()}, lookups)
			p.requireNoClaims(t)
		})
	}
}

func TestStationCreatorRouteAuthenticatesHopAndBindsActor(t *testing.T) {
	for _, tc := range []struct {
		name, issuer, hint string
		authLookups        int32
	}{
		{name: "forged hop", hint: pdsAuthTestUser.String()},
		{name: "signed by unknown peer", issuer: "did:web:unknown.example", hint: pdsAuthTestUser.String()},
		{name: "authenticated actor differs from hint", issuer: stationCreatorOwnerDID, hint: pdsAuthTestOAuth, authLookups: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newStationCreatorTestPair(t, &stationCreatorTestSlots{
				owner: stationCreatorIngressDID, route: stationCreatorIngressDID,
			}, nil)
			headers := p.bearer(t, p.owner.cli.ServerDID(), stationCreatorTestNSID)
			headers.Set(creatorHopHeader, tc.hint)
			if tc.issuer != "" {
				token, err := CreateServiceToken(p.owner.cli.ServiceAuthKey, tc.issuer)
				require.NoError(t, err)
				headers.Set(serviceAuthHeader, token)
			}
			rec := p.request(t, http.MethodPost, stationCreatorTestPath, "{}", headers)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			require.Equal(t, tc.authLookups, p.ingress.directory.lookups.Load())
			require.Zero(t, p.owner.requests.Load())
			p.requireNoClaims(t)
		})
	}
}

func TestStationCreatorRouteFailsClosedOnOwnerChange(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		ingress, owner       *stationCreatorTestSlots
		status               int
		peerHits, ownerAuths int32
	}{
		{
			name: "unknown owner", ingress: &stationCreatorTestSlots{owner: "did:web:unknown.example"},
			status: http.StatusServiceUnavailable,
		},
		{
			name: "stale owner", owner: &stationCreatorTestSlots{},
			status: http.StatusConflict, peerHits: 1,
		},
		{
			name: "loop", owner: &stationCreatorTestSlots{owner: stationCreatorIngressDID},
			status: http.StatusConflict, peerHits: 1,
		},
		{
			name:   "owner changes after authentication",
			owner:  &stationCreatorTestSlots{owner: stationCreatorOwnerDID, route: stationCreatorIngressDID},
			status: http.StatusConflict, peerHits: 1, ownerAuths: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newStationCreatorTestPair(t, tc.ingress, tc.owner)
			rec := p.request(t, http.MethodPost, stationCreatorTestPath, "{}", p.bearer(t, p.ingress.cli.ServerDID(), stationCreatorTestNSID))
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			require.EqualValues(t, 1, p.ingress.requests.Load(), "must not forward back to ingress")
			require.Equal(t, tc.peerHits, p.owner.requests.Load())
			require.Zero(t, p.ingress.directory.lookups.Load())
			require.Equal(t, tc.ownerAuths, p.owner.directory.lookups.Load())
			_, ingressRoutes := p.ingress.slots.calls()
			require.Empty(t, ingressRoutes)
			_, ownerRoutes := p.owner.slots.calls()
			if tc.ownerAuths == 0 {
				require.Empty(t, ownerRoutes)
			} else {
				require.Equal(t, []string{pdsAuthTestUser.String()}, ownerRoutes)
			}
			p.requireNoWrites(t)
		})
	}
}

func TestStationCreatorRouteDoesNotFollowOrReturnRedirect(t *testing.T) {
	p := newStationCreatorTestPair(t, nil, nil)
	headers := p.bearer(t, p.ingress.cli.ServerDID(), stationCreatorTestNSID)
	rec := p.request(t, http.MethodPost, stationCreatorTestPath+"?redirect=1", "{}", headers)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Empty(t, rec.Header().Get("Location"))
	require.Empty(t, rec.Header().Get(echo.HeaderAuthorization))
	require.Empty(t, rec.Header().Get(serviceAuthHeader))
	require.NotContains(t, rec.Body.String(), strings.TrimPrefix(headers.Get(echo.HeaderAuthorization), "Bearer "))
	require.EqualValues(t, 1, p.owner.directory.lookups.Load())
	require.EqualValues(t, 1, p.owner.requests.Load())
	require.Zero(t, p.owner.redirects.Load(), "redirect destination must never receive credentials")
	p.requireNoWrites(t)
}

func TestStationCreatorRouteDoesNotRoutePlayback(t *testing.T) {
	p := newStationCreatorTestPair(t, nil, nil)
	headers := p.bearer(t, p.ingress.cli.ServerDID(), strings.TrimPrefix(stationCreatorTestPlayback, "/xrpc/"))
	rec := p.request(t, http.MethodGet, stationCreatorTestPlayback, "", headers)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, p.ingress.cli.ServerDID(), rec.Body.String())
	require.Zero(t, p.owner.requests.Load())
	require.EqualValues(t, 1, p.ingress.directory.lookups.Load())
	lookups, _ := p.ingress.slots.calls()
	require.Empty(t, lookups)
	p.requireNoClaims(t)
}
