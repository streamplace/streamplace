package spxrpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spaces/spacestest"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/stationshard"
)

func multistreamTestServer(t *testing.T) *Server {
	t.Helper()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(t.Context(), &config.CLI{DBURL: ":memory:"}, nil, mod)
	require.NoError(t, err)
	return &Server{statefulDB: state}
}

func TestMultistreamLegacyRPC(t *testing.T) {
	s := multistreamTestServer(t)
	ctx := context.WithValue(t.Context(), pdsAuthContextKey, "did:plc:multistreamrpc")
	input := &placestream.MultistreamCreateTarget_Input{MultistreamTarget: placestream.MultistreamTarget{
		Url: "rtmp://localhost/live/key", CreatedAt: "2026-11-09T00:00:00Z",
	}}
	created, err := s.handlePlaceStreamMultistreamCreateTarget(ctx, input)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(created.Uri, "at://did:plc:multistreamrpc/place.stream.multistream.target/"))
	rkey := created.Uri[strings.LastIndex(created.Uri, "/")+1:]
	name := "renamed"
	input.MultistreamTarget.Name = &name
	put := &placestream.MultistreamPutTarget_Input{Rkey: &rkey, MultistreamTarget: input.MultistreamTarget}
	other := context.WithValue(t.Context(), pdsAuthContextKey, "did:plc:other")
	_, err = s.handlePlaceStreamMultistreamPutTarget(other, put)
	require.Error(t, err)
	_, err = s.handlePlaceStreamMultistreamDeleteTarget(other, &placestream.MultistreamDeleteTarget_Input{Rkey: rkey})
	require.Error(t, err)
	updated, err := s.handlePlaceStreamMultistreamPutTarget(ctx, put)
	require.NoError(t, err)
	require.Equal(t, created.Uri, updated.Uri)
	listed, err := s.handlePlaceStreamMultistreamListTargets(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, listed.Targets, 1)
	require.Equal(t, name, *listed.Targets[0].Record.Val.(*placestream.MultistreamTarget).Name)
	_, err = s.handlePlaceStreamMultistreamDeleteTarget(ctx, &placestream.MultistreamDeleteTarget_Input{Rkey: rkey})
	require.NoError(t, err)
	listed, err = s.handlePlaceStreamMultistreamListTargets(ctx, "", 50)
	require.NoError(t, err)
	require.Empty(t, listed.Targets)
	// A cursor is an offset into the list, which the space path slices by.
	_, err = s.handlePlaceStreamMultistreamListTargets(ctx, "-1", 50)
	require.ErrorContains(t, err, "invalid cursor")
}

func TestMultistreamSpaceTargets(t *testing.T) {
	s := multistreamTestServer(t)
	ctx := t.Context()
	did := "did:plc:multistreamspace"
	space := &spacestest.Fake{}
	target := func(active bool) *placestream.MultistreamTarget {
		return &placestream.MultistreamTarget{Url: "rtmp://localhost/live/key", CreatedAt: "2026-11-11T00:00:00Z", Active: active}
	}

	_, err := s.putSpaceTarget(ctx, space, did, "missing", target(false))
	require.ErrorContains(t, err, "not found")

	var views []*placestream.MultistreamDefs_TargetView
	for range statedb.MAX_ACTIVE_MULTISTREAM_TARGETS {
		view, err := s.putSpaceTarget(ctx, space, did, "", target(true))
		require.NoError(t, err)
		views = append(views, view)
	}
	_, err = s.putSpaceTarget(ctx, space, did, "", target(true))
	require.ErrorContains(t, err, "active")
	inactive, err := s.putSpaceTarget(ctx, space, did, "", target(false))
	require.NoError(t, err)
	rkey := func(view *placestream.MultistreamDefs_TargetView) string {
		return view.Uri[strings.LastIndex(view.Uri, "/")+1:]
	}
	// One more cannot be switched on, but an active one can still be edited.
	_, err = s.putSpaceTarget(ctx, space, did, rkey(inactive), target(true))
	require.ErrorContains(t, err, "active")
	_, err = s.putSpaceTarget(ctx, space, did, rkey(views[0]), target(true))
	require.NoError(t, err)

	// The list is in rkey order, pages by offset, and carries statedb's events.
	require.NoError(t, s.statefulDB.CreateMultistreamEvent(views[1].Uri, "connected", "active"))
	listed, err := s.listSpaceTargets(ctx, space, did, 2, 1)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, views[1].Uri, listed[0].Uri)
	require.Equal(t, "active", listed[0].LatestEvent.Status)
	require.Equal(t, views[2].Uri, listed[1].Uri)
	require.Nil(t, listed[1].LatestEvent)
	// Nothing reached statedb's own table.
	rows, err := s.statefulDB.ListMultistreamTargets(did, 100, 0, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestStationTargetSpaceGrant(t *testing.T) {
	const did = "did:plc:multistreamrpc"
	const personal = "space:place.stream.personal"
	for _, tc := range []struct {
		name  string
		scope string
		want  bool
	}{
		{"expanded grant", "atproto " + personal + "?authority=" + did + "&collection=place.stream.multistream.target&manage=create", true},
		{"unexpanded bare grant", personal, false},
		{"legacy session", "", false},
		{"unexpanded include", "atproto include:place.stream.authFull", false},
		{"prefix collision", personal + ".other", false},
		{"not a space token", "rpc:" + personal, false},
		{"other authority", personal + "?authority=did:plc:someoneelse", false},
		{"other key", personal + "?skey=other", false},
		{"other collection", personal + "?collection=place.stream.key", false},
		{"read only", personal + "?action=read", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, spaces.CollectionGrant(tc.scope, did, spaces.TargetCollection))
		})
	}
}

type multistreamTestTransport func(*http.Request) (*http.Response, error)

func (f multistreamTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestStationMultistreamSpaceOnly(t *testing.T) {
	cfg := stationtest.Config(t)
	namespace := "multistream-rpc-" + uuid.NewString()
	prefix := "stations/" + namespace + "/"
	objects := blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		keys, err := objects.List(ctx, prefix)
		require.NoError(t, err)
		for _, key := range keys {
			require.True(t, strings.HasPrefix(key, prefix))
			require.NoError(t, objects.Delete(ctx, key))
		}
	})
	var key [32]byte
	_, err := rand.Read(key[:])
	require.NoError(t, err)
	keyFile := filepath.Join(t.TempDir(), "station.key")
	require.NoError(t, os.WriteFile(keyFile, key[:], 0600))
	cli := &config.CLI{DBURL: "station://" + namespace, StationStateKeyFile: keyFile, ServerHost: stationCreatorTestOwner}
	cli.SetS3Config(cfg)
	state, err := statedb.MakeDB(t.Context(), cli, nil, nil)
	require.NoError(t, err)
	db, err := state.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	shards, err := stationshard.Open(stationshard.Config{S3: cfg, Prefix: prefix + "shards", Owner: stationCreatorOwnerDID})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, shards.Close()) })
	state.StationShards = shards

	const (
		did        = "did:plc:stationmultistream"
		other      = "did:plc:othermultistream"
		grant      = "space:place.stream.personal?authority=" + did + "&skey=self&collection=" + spaces.TargetCollection
		listPath   = "/xrpc/place.stream.multistream.listTargets"
		createPath = "/xrpc/place.stream.multistream.createTarget"
		putPath    = "/xrpc/place.stream.multistream.putTarget"
		deletePath = "/xrpc/place.stream.multistream.deleteTarget"
	)
	target := placestream.MultistreamTarget{Url: "rtmp://example.com/live/private-station-key", CreatedAt: "2026-11-11T00:00:00Z"}
	input := placestream.MultistreamCreateTarget_Input{MultistreamTarget: target}
	legacy, err := state.CreateMultistreamTarget(input, did)
	require.NoError(t, err)
	legacyKey := strings.TrimPrefix(legacy.Uri, spaces.TargetURI(did, ""))

	space := &spacestest.Fake{SpaceExists: true}
	failMove := true
	keyPair, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	dpopKey, err := jwk.FromRaw(keyPair)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(dpopKey)
	require.NoError(t, err)
	op := oatproxy.New(&oatproxy.Config{Host: stationCreatorTestOwner, HTTPClient: &http.Client{
		Transport: multistreamTestTransport(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "request-pds.example", r.URL.Host)
			require.Equal(t, "DPoP request-token", r.Header.Get("Authorization"))
			require.NotEmpty(t, r.Header.Get("DPoP"))
			method := strings.TrimPrefix(r.URL.Path, "/xrpc/")
			rec := httptest.NewRecorder()
			if failMove && method == "com.atproto.space.createRecord" {
				space.Calls = append(space.Calls, method)
				rec.WriteHeader(http.StatusForbidden)
				_, err := rec.WriteString(`{"error":"insufficient_scope"}`)
				return rec.Result(), err
			}
			var body, reply json.RawMessage
			if r.Body != nil {
				defer r.Body.Close()
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			}
			require.NoError(t, space.Do(r.Context(), r.Method, "", method, nil, body, &reply))
			_, err := rec.Write(reply)
			return rec.Result(), err
		}),
	}})
	s := &Server{
		cli:           cli,
		statefulDB:    state,
		creatorRoutes: &creatorRouter{slots: &stationCreatorTestSlots{route: stationCreatorOwnerDID}},
	}
	authorize := func(t *testing.T, path, actions string) (context.Context, error) {
		t.Helper()
		ctx := context.WithValue(t.Context(), creatorRouteContextKey{}, creatorRoute{did: did})
		ctx = context.WithValue(ctx, oatproxy.OATProxyContextKey, op)
		ctx = context.WithValue(ctx, oatproxy.OAuthSessionContextKey, &oatproxy.OAuthSession{
			DID: did, PDSUrl: "https://request-pds.example", UpstreamDPoPPrivateJWK: string(keyJSON),
			UpstreamAccessToken: "request-token", UpstreamScope: grant + actions,
		})
		req := httptest.NewRequest(creatorMethod(path), path, nil).WithContext(ctx)
		c := echo.New().NewContext(req, httptest.NewRecorder())
		var authed context.Context
		err := s.creatorAuthorize(func(c echo.Context) error {
			authed = c.Request().Context()
			return nil
		})(c)
		return authed, err
	}
	authorized := func(t *testing.T, path, actions string) context.Context {
		t.Helper()
		authed, err := authorize(t, path, actions)
		require.NoError(t, err)
		require.NotNil(t, authed)
		require.True(t, creatorSpaceOnly(authed))
		return authed
	}
	expectStatus := func(t *testing.T, want int, err error) {
		t.Helper()
		var got *echo.HTTPError
		require.ErrorAs(t, err, &got)
		require.Equal(t, want, got.Code)
	}
	readCtx := authorized(t, listPath, "&action=read")
	listed, err := s.handlePlaceStreamMultistreamListTargets(readCtx, "", 50)
	expectStatus(t, http.StatusForbidden, err)
	require.Nil(t, listed)
	require.Empty(t, space.Calls)
	rows, err := state.ListMultistreamTargets(did, 50, 0, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, legacy.Uri, rows[0].Uri)

	fullCtx := authorized(t, listPath, "")
	_, err = s.handlePlaceStreamMultistreamListTargets(fullCtx, "", 50)
	expectStatus(t, http.StatusInternalServerError, err)
	require.Equal(t, []string{"com.atproto.space.createRecord"}, space.Calls)
	space.Calls = nil
	_, err = s.handlePlaceStreamMultistreamListTargets(readCtx, "", 50)
	expectStatus(t, http.StatusForbidden, err)
	require.Empty(t, space.Calls, "a restricted grant cannot resume a partial move")
	failMove = false
	listed, err = s.handlePlaceStreamMultistreamListTargets(fullCtx, "", 50)
	require.NoError(t, err)
	require.Len(t, listed.Targets, 1)
	require.Equal(t, legacy.Uri, listed.Targets[0].Uri)
	require.Equal(t, []string{"com.atproto.space.listRepoOps", "com.atproto.space.createRecord", "com.atproto.space.listRecords"}, space.Calls)
	space.Calls = nil

	stale, err := state.CreateMultistreamTarget(input, did)
	require.NoError(t, err)
	_, err = state.CreateMultistreamTarget(input, other)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		status int
		change func(*oatproxy.OAuthSession)
	}{
		{"scope lost on refresh", http.StatusForbidden, func(session *oatproxy.OAuthSession) {
			session.DownstreamScope, session.UpstreamScope = session.UpstreamScope, ""
		}},
		{"wrong authority", http.StatusForbidden, func(session *oatproxy.OAuthSession) {
			session.UpstreamScope = strings.Replace(session.UpstreamScope, "authority="+did, "authority="+other, 1)
		}},
		{"account changed", http.StatusForbidden, func(session *oatproxy.OAuthSession) { session.DID = other }},
		{"unusable client", http.StatusUnauthorized, func(session *oatproxy.OAuthSession) { session.UpstreamDPoPPrivateJWK = "invalid" }},
		{"missing token", http.StatusServiceUnavailable, func(session *oatproxy.OAuthSession) { session.UpstreamAccessToken = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := *GetCaller(readCtx).OAuth
			tc.change(&session)
			ctx := context.WithValue(readCtx, oatproxy.OAuthSessionContextKey, &session)
			out, err := s.handlePlaceStreamMultistreamListTargets(ctx, "", 50)
			expectStatus(t, tc.status, err)
			require.Nil(t, out)
			require.Empty(t, space.Calls)
		})
	}
	bearerCtx := context.WithValue(readCtx, oatproxy.OAuthSessionContextKey, struct{}{})
	bearerCtx = context.WithValue(bearerCtx, pdsAuthContextKey, did)
	_, err = s.handlePlaceStreamMultistreamListTargets(bearerCtx, "", 50)
	expectStatus(t, http.StatusUnauthorized, err)

	create := placestream.MultistreamCreateTarget_Input{MultistreamTarget: placestream.MultistreamTarget{
		Url: "rtmp://example.com/live/space-key", CreatedAt: target.CreatedAt,
	}}
	put := placestream.MultistreamPutTarget_Input{Rkey: &legacyKey, MultistreamTarget: create.MultistreamTarget}
	_, err = authorize(t, createPath, "&action=create")
	expectStatus(t, http.StatusForbidden, err)
	_, err = authorize(t, putPath, "&action=update")
	expectStatus(t, http.StatusForbidden, err)
	require.Empty(t, space.Calls, "quota reads must be authorized before calling the PDS")

	created, err := s.handlePlaceStreamMultistreamCreateTarget(authorized(t, createPath, "&action=read&action=create"), &create)
	require.NoError(t, err)
	_, err = s.handlePlaceStreamMultistreamPutTarget(authorized(t, putPath, "&action=read&action=update"), &put)
	require.NoError(t, err)
	_, err = s.handlePlaceStreamMultistreamDeleteTarget(authorized(t, deletePath, "&action=delete"), &placestream.MultistreamDeleteTarget_Input{Rkey: legacyKey})
	require.NoError(t, err)
	listed, err = s.handlePlaceStreamMultistreamListTargets(readCtx, "", 50)
	require.NoError(t, err)
	require.Len(t, listed.Targets, 1)
	require.Equal(t, created.Uri, listed.Targets[0].Uri)
	require.Equal(t, []string{
		"com.atproto.space.listRecords", "com.atproto.space.putRecord",
		"com.atproto.space.listRecords", "com.atproto.space.putRecord",
		"com.atproto.space.deleteRecord", "com.atproto.space.listRecords",
	}, space.Calls)
	moved, err := state.StationTargetsMoved(t.Context(), did)
	require.NoError(t, err)
	require.True(t, moved)
	rows, err = state.ListMultistreamTargets(did, 50, 0, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, stale.Uri, rows[0].Uri)

	require.NoError(t, shards.Close())
	space.Calls = nil
	for _, ctx := range []context.Context{readCtx, fullCtx} {
		listed, err := s.handlePlaceStreamMultistreamListTargets(ctx, "", 50)
		expectStatus(t, http.StatusInternalServerError, err)
		require.Nil(t, listed)
	}
	require.Empty(t, space.Calls)
}
