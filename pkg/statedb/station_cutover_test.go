//go:build stationslate && linux && cgo

package statedb

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/stationdb"
)

func TestStationCutoverPreservesRotatedSession(t *testing.T) {
	ctx := t.Context()
	bucket := stationtest.Config(t)
	dir := t.TempDir()
	from := "sqlite://" + filepath.Join(dir, "state.sqlite")
	source, err := MakeDB(ctx, &config.CLI{DBURL: from}, nil, nil)
	require.NoError(t, err)
	const jkt = "session-import"
	require.NoError(t, source.CreateOAuthSession(jkt, &oatproxy.OAuthSession{
		DownstreamDPoPJKT:     jkt,
		DownstreamAccessToken: "ready",
		UpstreamRefreshToken:  "old",
	}))
	pool, err := source.DB.DB()
	require.NoError(t, err)
	require.NoError(t, pool.Close())

	keyPath := filepath.Join(dir, "key")
	var key [32]byte
	_, err = rand.Read(key[:])
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, key[:], 0600))
	cfg := &config.CLI{DBURL: "station://cutover-" + uuid.NewString(), StationStateKeyFile: keyPath}
	cfg.SetS3Config(bucket)
	_, err = CopyState(ctx, from, cfg, 10)
	require.NoError(t, err)
	reopen := func() *StatefulDB {
		t.Helper()
		s, err := MakeDB(ctx, cfg, nil, nil)
		require.NoError(t, err)
		db, err := s.DB.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return s
	}
	a, b := reopen(), reopen()
	first := loadStationTestOAuthSession(t, a, jkt)
	stale := loadStationTestOAuthSession(t, b, jkt)
	first.UpstreamRefreshToken = "rotated"
	require.NoError(t, a.UpdateOAuthSession(jkt, first))
	require.NoError(t, stale.CacheJTI("concurrent-proof"))
	submitted := *stale
	require.ErrorIs(t, b.UpdateOAuthSession(jkt, stale), stationdb.ErrConflict, "a stale verifier must not restore the old refresh token")
	require.Equal(t, submitted, *stale)
	_, err = CopyState(ctx, from, cfg, 10)
	require.ErrorContains(t, err, "empty namespace")
	after := loadStationTestOAuthSession(t, b, jkt)
	require.Equal(t, "rotated", after.UpstreamRefreshToken)
	require.Empty(t, after.DownstreamJTICache)
}

func TestStationOAuthSessionProofReconciliation(t *testing.T) {
	ctx := t.Context()
	bucket := stationtest.Config(t)
	prefix := "oauth-proof-" + uuid.NewString()
	var key [32]byte
	_, err := rand.Read(key[:])
	require.NoError(t, err)
	open := func() *StatefulDB {
		t.Helper()
		db, store, err := openStationDB(ctx, bucket, prefix, key[:])
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return &StatefulDB{DB: db, stationStore: store}
	}
	a := open()
	require.NoError(t, a.DB.AutoMigrate(&oatproxy.OAuthSession{}))
	b := open()
	loadPair := func(t *testing.T) (*oatproxy.OAuthSession, *oatproxy.OAuthSession) {
		t.Helper()
		jkt := t.Name()
		expiry := time.Now().UTC().Add(time.Hour)
		base := &oatproxy.OAuthSession{
			DownstreamDPoPJKT:      jkt,
			DownstreamAccessToken:  "downstream-token",
			UpstreamAccessTokenExp: &expiry,
			UpstreamScope:          "atproto",
		}
		require.NoError(t, base.CacheJTI("prior-proof"))
		require.NoError(t, a.CreateOAuthSession(jkt, base))
		return loadStationTestOAuthSession(t, a, jkt), loadStationTestOAuthSession(t, b, jkt)
	}
	entries := func(t *testing.T, session *oatproxy.OAuthSession) []oatproxy.JTICacheEntry {
		t.Helper()
		var result []oatproxy.JTICacheEntry
		require.NoError(t, json.Unmarshal([]byte(session.DownstreamJTICache), &result))
		return result
	}

	for _, tc := range []struct {
		name   string
		proofs []string
		skew   time.Duration
		accept bool
	}{
		{name: "distinct", proofs: []string{"second-proof"}, accept: true},
		{name: "duplicate", proofs: []string{"first-proof"}},
		{name: "multiple-new", proofs: []string{"extra-proof", "second-proof"}},
		{name: "expired", proofs: []string{"second-proof"}, skew: -time.Hour},
		{name: "future", proofs: []string{"second-proof"}, skew: time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, stale := loadPair(t)
			jkt := first.DownstreamDPoPJKT
			require.NoError(t, first.CacheJTI("first-proof"))
			for _, proof := range tc.proofs {
				require.NoError(t, stale.CacheJTI(proof))
			}
			proposed := entries(t, stale)
			if tc.skew != 0 {
				proposed[len(proposed)-1].Time = time.Now().Add(tc.skew)
				encoded, err := json.Marshal(proposed)
				require.NoError(t, err)
				stale.DownstreamJTICache = string(encoded)
			}
			submitted := *stale
			// Both verifiers read and appended before either commit. Force A to win the first CAS.
			require.NoError(t, a.UpdateOAuthSession(jkt, first))
			err := b.UpdateOAuthSession(jkt, stale)
			if !tc.accept {
				require.ErrorIs(t, err, stationdb.ErrConflict)
				require.Equal(t, submitted, *stale)
				after := loadStationTestOAuthSession(t, a, jkt)
				require.Equal(t, first.DownstreamJTICache, after.DownstreamJTICache)
				require.True(t, first.UpdatedAt.Equal(after.UpdatedAt))
				return
			}
			require.NoError(t, err)
			want := append(entries(t, first), proposed[len(proposed)-1])
			after := loadStationTestOAuthSession(t, a, jkt)
			require.Equal(t, want, entries(t, after))
			require.Equal(t, after.DownstreamJTICache, stale.DownstreamJTICache)
			require.True(t, stale.UpdatedAt.After(first.UpdatedAt))
			require.True(t, stale.UpdatedAt.Equal(after.UpdatedAt))
		})
	}

	revokedAt := time.Now().UTC()
	for _, tc := range []struct {
		name   string
		change func(*oatproxy.OAuthSession)
	}{
		{"upstream-revoked", func(s *oatproxy.OAuthSession) { s.RevokedAt = &revokedAt }},
		{"downstream-revoked", func(s *oatproxy.OAuthSession) { s.DownstreamRevokedAt = &revokedAt }},
		{"grant-changed", func(s *oatproxy.OAuthSession) { s.UpstreamScope = "atproto blob:*/*" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, stale := loadPair(t)
			jkt := first.DownstreamDPoPJKT
			require.NoError(t, stale.CacheJTI("second-proof"))
			submitted := *stale
			tc.change(first)
			require.NoError(t, a.UpdateOAuthSession(jkt, first))
			require.ErrorIs(t, b.UpdateOAuthSession(jkt, stale), stationdb.ErrConflict)
			require.Equal(t, submitted, *stale)
			after := loadStationTestOAuthSession(t, b, jkt)
			require.Equal(t, first.DownstreamJTICache, after.DownstreamJTICache)
			require.True(t, first.UpdatedAt.Equal(after.UpdatedAt))
			require.Equal(t, first.Status(), after.Status())
			require.Equal(t, first.UpstreamScope, after.UpstreamScope)
		})
	}
}

func loadStationTestOAuthSession(t *testing.T, state *StatefulDB, jkt string) *oatproxy.OAuthSession {
	t.Helper()
	var session oatproxy.OAuthSession
	require.NoError(t, state.DB.Where("downstream_dpop_jkt = ?", jkt).First(&session).Error)
	return &session
}
