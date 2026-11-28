package spxrpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
)

const stationTargetTestActor = "did:plc:stationtargetstreamer"

type stationTargetTestSlots struct {
	owner      string
	ownerReads atomic.Int32
	routeReads atomic.Int32
}

func (s *stationTargetTestSlots) Owner(_ context.Context, did string) (string, error) {
	s.ownerReads.Add(1)
	if did != stationTargetTestActor {
		return "", nil
	}
	return s.owner, nil
}

func (s *stationTargetTestSlots) Route(_ context.Context, did string) (string, error) {
	if did != stationTargetTestActor {
		return "", nil
	}
	s.routeReads.Add(1)
	return s.owner, nil
}

func TestStationActiveTargetsRejectUnauthorizedAndWrongOwner(t *testing.T) {
	key, err := jwk.FromRaw(bytes.Repeat([]byte{0x53}, 32))
	require.NoError(t, err)
	sender := &config.CLI{ServerHost: "sender.test", ServiceAuthKey: key}
	receiver := &config.CLI{
		ServerHost: "receiver.test", ServiceAuthKey: key,
		StationPeers: sender.ServerDID() + "=https://sender.test",
	}
	peers, err := receiver.StationPeerOrigins()
	require.NoError(t, err)
	wrongNode := &stationTargetTestSlots{owner: "did:web:actual-owner.test"}
	s := &Server{cli: receiver, creatorRoutes: &creatorRouter{slots: wrongNode, peers: peers}}
	e := echo.New()
	e.Pre(s.stationActiveTargetsDispatch)
	peer := httptest.NewTLSServer(e)
	defer peer.Close()

	sign := func(issuer, audience, method string, expires *jwt.NumericDate) string {
		t.Helper()
		claims := serviceAuthClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer: issuer, Subject: stationTargetTestActor, Audience: jwt.ClaimStrings{audience}, ExpiresAt: expires,
			},
			Lxm: method,
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(stationActiveTargetsKey(sender))
		require.NoError(t, err)
		return token
	}
	expires := jwt.NewNumericDate(time.Now().Add(time.Minute))
	bound := sign(sender.ServerDID(), receiver.ServerDID(), stationActiveTargetsMethod, expires)
	generic, err := CreateServiceToken(key, sender.ServerDID())
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		token  string
		suffix string
	}{
		{name: "user credentials only"},
		{name: "ordinary station credential", token: generic},
		{name: "unlisted sender", token: sign("did:web:unlisted.test", receiver.ServerDID(), stationActiveTargetsMethod, expires)},
		{name: "different audience", token: sign(sender.ServerDID(), wrongNode.owner, stationActiveTargetsMethod, expires)},
		{name: "different method", token: sign(sender.ServerDID(), receiver.ServerDID(), "place.stream.multistream.listTargets", expires)},
		{name: "expired", token: sign(sender.ServerDID(), receiver.ServerDID(), stationActiveTargetsMethod, jwt.NewNumericDate(time.Now().Add(-time.Minute)))},
		{name: "no expiry", token: sign(sender.ServerDID(), receiver.ServerDID(), stationActiveTargetsMethod, nil)},
		{name: "actor override", token: bound, suffix: "?actor=did:plc:someoneelse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, peer.URL+stationActiveTargetsPath+tc.suffix, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "DPoP user-credential")
			if tc.token != "" {
				req.Header.Set(stationActiveTargetsAuth, tc.token)
			}
			resp, err := peer.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusForbidden, resp.StatusCode)
			require.Equal(t, "private, no-store", resp.Header.Get("Cache-Control"))
			require.Zero(t, wrongNode.ownerReads.Load())
		})
	}

	// This node is allowlisted and authenticated, but its local ownership check
	// points elsewhere. It must not touch either node's targets or forward again.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, peer.URL+stationActiveTargetsPath, nil)
	require.NoError(t, err)
	req.Header.Set(stationActiveTargetsAuth, bound)
	resp, err := peer.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	require.Equal(t, "private, no-store", resp.Header.Get("Cache-Control"))
	require.EqualValues(t, 1, wrongNode.ownerReads.Load())
	require.Zero(t, wrongNode.routeReads.Load())

	sender.StationPeers = receiver.ServerDID() + "=" + peer.URL
	targets, err := fetchStationActiveMultistreamTargets(t.Context(), sender, nil, &stationTargetTestSlots{owner: receiver.ServerDID()}, peer.Client(), stationTargetTestActor)
	require.ErrorIs(t, err, errStationActiveTargetsUnavailable)
	require.Nil(t, targets)
	require.EqualValues(t, 2, wrongNode.ownerReads.Load(), "the actual peer token must authenticate before ownership is checked")
	require.Zero(t, wrongNode.routeReads.Load())
}

func TestStationActiveTargetsPeerFailureDoesNotFallBackOrRedirect(t *testing.T) {
	state := multistreamTestServer(t).statefulDB
	_, err := state.CreateMultistreamTarget(placestream.MultistreamCreateTarget_Input{MultistreamTarget: placestream.MultistreamTarget{
		Active: true, Url: "rtmps://egress.test:443/live/legacy-key", CreatedAt: "2026-11-09T00:00:00Z",
	}}, stationTargetTestActor)
	require.NoError(t, err)

	var redirectRequests atomic.Int32
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer redirect.Close()
	var peerRequests atomic.Int32
	var status atomic.Int32
	const ownerDID = "did:web:owner.test"
	peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != stationActiveTargetsPath || r.URL.RawQuery != "" || r.Header.Get(stationActiveTargetsAuth) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		peerRequests.Add(1)
		if status.Load() == http.StatusTemporaryRedirect {
			http.Redirect(w, r, redirect.URL+stationActiveTargetsPath, http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "upstream error might contain a secret")
	}))
	defer peer.Close()
	key, err := jwk.FromRaw(bytes.Repeat([]byte{0x53}, 32))
	require.NoError(t, err)
	cli := &config.CLI{ServerHost: "sender.test", ServiceAuthKey: key, StationPeers: ownerDID + "=" + peer.URL}
	slots := &stationTargetTestSlots{owner: ownerDID}

	for _, code := range []int32{http.StatusServiceUnavailable, http.StatusTemporaryRedirect} {
		status.Store(code)
		targets, err := fetchStationActiveMultistreamTargets(t.Context(), cli, state, slots, peer.Client(), stationTargetTestActor)
		require.EqualError(t, err, "station active multistream targets unavailable")
		require.Nil(t, targets, "stale local SQL targets must not replace the owner's result")
	}
	require.EqualValues(t, 2, slots.routeReads.Load())
	require.EqualValues(t, 2, peerRequests.Load())
	require.Zero(t, redirectRequests.Load())
}
