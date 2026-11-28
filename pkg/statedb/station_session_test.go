package statedb

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/constants"
)

func TestStationRepoScopeGrants(t *testing.T) {
	counts, origins := constants.PLACE_STREAM_MEDIA_VIEW_COUNT, constants.PLACE_STREAM_MEDIA_ORIGIN
	cu, cud := []string{"create", "update"}, []string{"create", "update", "delete"}
	for _, tc := range []struct {
		name       string
		scope      string
		collection string
		actions    []string
		allowed    bool
	}{
		{"split grants", "repo:" + counts + "?action=create repo?collection=" + counts + "&action=update", counts, cu, true},
		{"create only", "repo:" + counts + "?action=create", counts, cu, false},
		{"update only", "repo:" + counts + "?action=update", counts, cu, false},
		{"origin cannot grant counts", "repo:" + origins, counts, cu, false},
		{"no cross-collection union", "repo:" + counts + "?action=create repo:" + origins + "?action=update", counts, cu, false},
		{"origin requires delete", "repo:" + origins + "?action=create&action=update", origins, cud, false},
		{"collection wildcard", "repo:*", counts, cu, true},
		{"collection wildcard still requires actions", "repo:*?action=create", counts, cu, false},
		{"authFull counts", "include:place.stream.authFull", counts, cu, true},
		{"authFull origin", "include:place.stream.authFull", origins, cud, true},
		{"authFull unrelated", "include:place.stream.authFull", "app.bsky.feed.post", cu, false},
		{"audience-bound include", "include:place.stream.authFull?aud=did:web:other.example%23service", origins, cud, false},
		{"wrong include", "include:place.stream.authOther", origins, cud, false},
		{"empty action list", "repo:" + counts, counts, nil, false},
		{"empty requested action", "include:place.stream.authFull repo:" + counts, counts, []string{"create", ""}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.allowed, stationRepoScopeGranted(tc.scope, tc.collection, tc.actions))
		})
	}
}

func TestStationRepoClientRechecksViewCountGrantAfterRefresh(t *testing.T) {
	const did, pds = "did:web:station.example", "https://pds.example"
	ctx := t.Context()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "oauth.db")))
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sql.Close() })
	require.NoError(t, db.AutoMigrate(&oatproxy.OAuthSession{}))

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	key, err := jwk.FromRaw(private)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(key)
	require.NoError(t, err)
	future := time.Now().UTC().Add(time.Hour)
	counts := constants.PLACE_STREAM_MEDIA_VIEW_COUNT
	upstreamGrant := "repo:" + counts + "?action=create&action=update"
	session := oatproxy.OAuthSession{
		DID: did, PDSUrl: pds, DownstreamDPoPJKT: "station-jkt", DownstreamAccessToken: "downstream",
		DownstreamScope: upstreamGrant, UpstreamAuthServerIssuer: pds, UpstreamDPoPPrivateJWK: string(keyJSON),
		UpstreamAccessToken: "upstream", UpstreamAccessTokenExp: &future, UpstreamScope: upstreamGrant,
		UpstreamRefreshToken: "refresh",
	}
	require.NoError(t, db.Create(&session).Error)
	reloadedScope := "repo:" + counts + "?action=create"
	reloads := 0
	state := &StatefulDB{DB: db, OATProxy: oatproxy.New(&oatproxy.Config{
		GetOAuthSession: func(jkt string) (*oatproxy.OAuthSession, error) {
			require.Equal(t, session.DownstreamDPoPJKT, jkt)
			reloads++
			refreshed := session
			refreshed.UpstreamScope = reloadedScope
			return &refreshed, nil
		},
	})}
	client, err := state.StationRepoClient(ctx, did, pds, counts, "create", "update")
	require.ErrorContains(t, err, "create+update grants on place.stream.media.viewCount")
	require.Nil(t, client)
	require.Equal(t, 1, reloads)

	reloadedScope = upstreamGrant
	client, err = state.StationRepoClient(ctx, did, pds, counts, "create", "update")
	require.NoError(t, err)
	require.NotNil(t, client)
}
