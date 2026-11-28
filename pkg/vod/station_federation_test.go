package vod

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/lestrrat-go/jwx/v2/jwk"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

const stationFederationTestDID = "did:plc:ewvi7nxzyoun6zhxrhs64oiz"

type stationXRPCFunc func(context.Context, string, string, string, map[string]any, any, any) error

func (f stationXRPCFunc) Do(ctx context.Context, kind, contentType, method string, params map[string]any, body, out any) error {
	return f(ctx, kind, contentType, method, params, body, out)
}

func TestStationOriginRetriedWriteAndWithdrawal(t *testing.T) {
	ctx := t.Context()
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	did, cid := stationFederationTestDID, bdasl.CID([]byte("asset"))
	collection := constants.PLACE_STREAM_MEDIA_ORIGIN
	uri := "at://" + did + "/" + collection + "/" + cid
	prefix := stationOriginPrefix(did)
	publicACK := prefix + "acked/public/" + cid
	withdrawnACK := prefix + "acked/withdrawn/" + cid
	origin := stationOrigin{CID: cid, Size: 123}
	wanted := placestream.MediaOrigin{LexiconTypeID: collection, Blob: cid, Size: origin.Size, MimeType: "video/mp4"}
	remote := wanted
	remote.Size--
	current := &remote
	var methods []string
	applyWrite := false
	client := stationXRPCFunc(func(_ context.Context, kind, contentType, method string, params map[string]any, body, out any) error {
		methods = append(methods, method)
		switch method {
		case "com.atproto.repo.getRecord":
			require.Equal(t, xrpc.Query, kind)
			require.Equal(t, map[string]any{"repo": did, "collection": collection, "rkey": cid}, params)
			if current == nil {
				return &xrpc.Error{StatusCode: http.StatusBadRequest, Wrapped: &xrpc.XRPCError{ErrStr: "RecordNotFound"}}
			}
			*out.(*comatproto.RepoGetRecord_Output) = comatproto.RepoGetRecord_Output{Uri: uri, Value: &glex.LexiconTypeDecoder{Val: current}}
			return nil
		case "com.atproto.repo.putRecord":
			require.Equal(t, xrpc.Procedure, kind)
			put := body.(*comatproto.RepoPutRecord_Input)
			require.Equal(t, &comatproto.RepoPutRecord_Input{Repo: did, Collection: collection, Rkey: cid, Record: &glex.LexiconTypeDecoder{Val: &wanted}}, put)
			if applyWrite {
				current = put.Record.Val.(*placestream.MediaOrigin)
			}
		case "com.atproto.repo.deleteRecord":
			require.Equal(t, xrpc.Procedure, kind)
			require.Equal(t, &comatproto.RepoDeleteRecord_Input{Repo: did, Collection: collection, Rkey: cid}, body)
			current = nil
		default:
			t.Fatalf("unexpected PDS call %s", method)
		}
		return errors.New("upstream disconnected after receiving request (do not log raw error)")
	})
	checkMarker := func(key string, want bool) {
		t.Helper()
		got, err := stationMarkerExists(ctx, store, key)
		require.NoError(t, err)
		require.Equal(t, want, got, key)
	}

	err = reconcileStationOrigin(ctx, store, client, did, cid, origin)
	require.EqualError(t, err, "station origin write not confirmed")
	checkMarker(publicACK, false)

	applyWrite = true
	methods = nil
	require.NoError(t, reconcileStationOrigin(ctx, store, client, did, cid, origin))
	require.Equal(t, []string{"com.atproto.repo.getRecord", "com.atproto.repo.putRecord", "com.atproto.repo.getRecord"}, methods)
	checkMarker(publicACK, true)

	// A restarted publisher may see a committed record without its ACK.
	require.NoError(t, store.Delete(ctx, publicACK))
	methods = nil
	require.NoError(t, reconcileStationOrigin(ctx, store, client, did, cid, origin))
	require.Equal(t, []string{"com.atproto.repo.getRecord"}, methods)
	checkMarker(publicACK, true)

	methods = nil
	origin.Withdrawn = true
	origin.terminal = true
	require.NoError(t, reconcileStationOrigin(ctx, store, client, did, cid, origin))
	require.Equal(t, []string{"com.atproto.repo.getRecord", "com.atproto.repo.deleteRecord", "com.atproto.repo.getRecord"}, methods)
	checkMarker(withdrawnACK, true)
	checkMarker(publicACK, true)

	// A withdrawn ACK wins over a stale public intent even without a public ACK.
	require.NoError(t, store.Delete(ctx, publicACK))
	for _, key := range []string{prefix + "public/" + cid, prefix + "withdrawn/" + cid} {
		require.NoError(t, writeStationMarker(ctx, store, key, []byte("{}")))
	}
	require.NoError(t, reconcileStationOrigins(ctx, store, did, func() (XRPCClient, error) {
		t.Fatal("withdrawn CID accessed the PDS after its withdrawal was ACKed")
		return nil, nil
	}, func(stationOrigin) (bool, error) {
		t.Fatal("withdrawn intent checked for publication")
		return false, nil
	}))
}

func TestStationOriginSourceDeleteReplay(t *testing.T) {
	ctx := t.Context()
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	did, cid := stationFederationTestDID, bdasl.CID([]byte("re-added source"))
	uri := "at://did:plc:alice/place.stream.video/video"
	originURI := "at://" + did + "/" + constants.PLACE_STREAM_MEDIA_ORIGIN + "/" + cid
	prefix := stationOriginPrefix(did)
	require.NoError(t, writeStationMarker(ctx, store, prefix+"public/"+cid,
		[]byte(fmt.Sprintf(`{"cid":%q,"uri":%q,"size":12}`, cid, uri))))
	require.NoError(t, AdvertiseStationVideo(ctx, store, did, cid, uri, "2222222222223"))
	require.NoError(t, RetractStationRecord(ctx, store, did, uri, "2222222222224", ""))
	const parentRev = "222222222222f"
	const parentCID = "station-parent-cid"
	remote := &placestream.MediaOrigin{LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN, Blob: cid, Size: 12, MimeType: transferMimeType}
	client := stationXRPCFunc(func(_ context.Context, _, _, method string, _ map[string]any, body, out any) error {
		switch method {
		case "com.atproto.repo.getRecord":
			if remote == nil {
				return &xrpc.Error{StatusCode: http.StatusBadRequest, Wrapped: &xrpc.XRPCError{ErrStr: "RecordNotFound"}}
			}
			*out.(*comatproto.RepoGetRecord_Output) = comatproto.RepoGetRecord_Output{Uri: originURI, Value: &glex.LexiconTypeDecoder{Val: remote}}
		case "com.atproto.sync.getLatestCommit":
			*out.(*comatproto.SyncGetLatestCommit_Output) = comatproto.SyncGetLatestCommit_Output{Rev: parentRev, Cid: parentCID}
		case "com.atproto.repo.deleteRecord":
			require.Equal(t, parentCID, *body.(*comatproto.RepoDeleteRecord_Input).SwapCommit)
			remote = nil
		case "com.atproto.repo.putRecord":
			remote = body.(*comatproto.RepoPutRecord_Input).Record.Val.(*placestream.MediaOrigin)
			*out.(*comatproto.RepoPutRecord_Output) = comatproto.RepoPutRecord_Output{Uri: originURI, Cid: "record-cid"}
		default:
			t.Fatalf("unexpected PDS request %s", method)
		}
		return nil
	})
	reconcile := func() {
		t.Helper()
		require.NoError(t, reconcileStationOrigins(ctx, store, did, func() (XRPCClient, error) { return client, nil },
			func(stationOrigin) (bool, error) { return false, nil }))
	}
	reconcile()
	require.Nil(t, remote)
	require.NoError(t, AdvertiseStationVideo(ctx, store, did, cid, uri, "2222222222225"))
	require.NoError(t, RetractStationRecord(ctx, store, did, originURI, "222222222222g", parentRev))
	reconcile()
	require.NotNil(t, remote)
}

func TestStationOriginWithdrawalAppearsDuringPreflight(t *testing.T) {
	ctx := t.Context()
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	did, cid := stationFederationTestDID, bdasl.CID([]byte("withdrawn during get"))
	prefix := stationOriginPrefix(did)
	client := stationXRPCFunc(func(ctx context.Context, _, _, method string, _ map[string]any, _, _ any) error {
		require.Equal(t, "com.atproto.repo.getRecord", method)
		require.NoError(t, writeStationMarker(ctx, store, prefix+"withdrawn/"+cid, []byte("{}")))
		return &xrpc.Error{StatusCode: http.StatusBadRequest, Wrapped: &xrpc.XRPCError{ErrStr: "RecordNotFound"}}
	})
	require.NoError(t, reconcileStationOrigin(ctx, store, client, did, cid, stationOrigin{CID: cid, Size: 123}))
	acked, err := stationMarkerExists(ctx, store, prefix+"acked/public/"+cid)
	require.NoError(t, err)
	require.False(t, acked)
}

func TestStationOriginRemoteModeration(t *testing.T) {
	const did = "did:plc:videoauthor"
	const uri = "at://" + did + "/place.stream.video/video"
	for _, tc := range []struct {
		name, record, account string
		blocked               bool
	}{
		{name: "record label", record: atproto.LabelWarn, blocked: true},
		{name: "account ban", account: atproto.LabelTakedown, blocked: true},
		{name: "non-ban account label", account: atproto.LabelWarn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocked, err := stationOriginModerated(func(subject string) ([]*comatproto.LabelDefs_Label, error) {
				value := tc.record
				if subject == did {
					value = tc.account
				} else {
					require.Equal(t, uri, subject)
				}
				if value == "" {
					return nil, nil
				}
				return []*comatproto.LabelDefs_Label{{Uri: subject, Val: value}}, nil
			}, uri)
			require.NoError(t, err)
			require.Equal(t, tc.blocked, blocked)
		})
	}
}

func TestStationRepoClientUsesBoundOriginGrant(t *testing.T) {
	ctx := t.Context()
	did := stationFederationTestDID
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "oauth.db")))
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sql.Close() })
	require.NoError(t, db.AutoMigrate(&oatproxy.OAuthSession{}))
	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "DPoP eligible-token" || r.Header.Get("DPoP") == "" || r.URL.Path != "/xrpc/com.atproto.repo.getRecord" {
			t.Errorf("station used the wrong OAuth session or target: %s", r.URL.Path)
			http.Error(w, "wrong station authorization", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uri":"at://station-record"}`))
	}))
	t.Cleanup(pds.Close)

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	key, err := jwk.FromRaw(private)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(key)
	require.NoError(t, err)
	now := time.Now().UTC()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	const grant = "atproto repo:place.stream.media.origin?action=create repo?collection=place.stream.media.origin&action=update&action=delete"
	base := oatproxy.OAuthSession{
		DID: did, PDSUrl: pds.URL + "/", DownstreamAccessToken: "downstream",
		UpstreamAuthServerIssuer: pds.URL, UpstreamDPoPPrivateJWK: string(keyJSON),
		UpstreamAccessToken: "eligible-token", UpstreamAccessTokenExp: &future, UpstreamScope: grant,
		UpdatedAt: past,
	}
	for _, tc := range []struct {
		id     string
		change func(*oatproxy.OAuthSession)
	}{
		{id: "legacy", change: func(s *oatproxy.OAuthSession) {
			s.UpstreamScope = ""
			s.DownstreamScope = "include:place.stream.authFull"
		}},
		{id: "partial", change: func(s *oatproxy.OAuthSession) {
			s.UpstreamScope = "repo:place.stream.media.origin?action=create&action=update"
		}},
		{id: "wrong-pds", change: func(s *oatproxy.OAuthSession) { s.PDSUrl = "https://different.example" }},
		{id: "wrong-account", change: func(s *oatproxy.OAuthSession) { s.DID = "did:plc:other" }},
		{id: "upstream-revoked", change: func(s *oatproxy.OAuthSession) { s.RevokedAt = &now }},
		{id: "downstream-revoked", change: func(s *oatproxy.OAuthSession) { s.DownstreamRevokedAt = &now }},
		{id: "pending", change: func(s *oatproxy.OAuthSession) { s.DownstreamAccessToken = "" }},
		{id: "expired", change: func(s *oatproxy.OAuthSession) { s.UpstreamAccessTokenExp = &past }},
		{id: "malformed-key", change: func(s *oatproxy.OAuthSession) { s.UpstreamDPoPPrivateJWK = "not a private key" }},
		{id: "scope-lost", change: func(s *oatproxy.OAuthSession) {
			s.UpstreamScope = "include:place.stream.authFull"
			s.UpstreamRefreshToken = "refresh"
		}},
	} {
		s := base
		s.DownstreamDPoPJKT, s.UpstreamAccessToken, s.UpdatedAt = tc.id, "wrong-token", now
		tc.change(&s)
		require.NoError(t, db.Create(&s).Error)
	}
	base.DownstreamDPoPJKT = "eligible"
	require.NoError(t, db.Create(&base).Error)
	var reloaded []string
	state := &statedb.StatefulDB{DB: db, OATProxy: oatproxy.New(&oatproxy.Config{
		HTTPClient: pds.Client(),
		GetOAuthSession: func(id string) (*oatproxy.OAuthSession, error) {
			reloaded = append(reloaded, id)
			var s oatproxy.OAuthSession
			err := db.Where("downstream_dpop_jkt = ?", id).First(&s).Error
			s.UpstreamScope = "atproto"
			return &s, err
		},
	})}
	client, err := state.StationRepoClient(ctx, did, pds.URL, constants.PLACE_STREAM_MEDIA_ORIGIN, "create", "update", "delete")
	require.NoError(t, err)
	require.Equal(t, []string{"scope-lost"}, reloaded)
	var out comatproto.RepoGetRecord_Output
	require.NoError(t, client.Do(ctx, xrpc.Query, "", "com.atproto.repo.getRecord", nil, nil, &out))
	require.Equal(t, "at://station-record", out.Uri)

	require.NoError(t, db.Delete(&base).Error)
	_, err = state.StationRepoClient(ctx, did, pds.URL, constants.PLACE_STREAM_MEDIA_ORIGIN, "create", "update", "delete")
	require.ErrorContains(t, err, "no usable station OAuth session")
}
