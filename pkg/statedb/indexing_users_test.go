package statedb

import (
	"testing"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
)

func TestIndexingUsersRequireCompletedUsableSessions(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		now := time.Now().UTC()
		future, past := now.Add(time.Hour), now.Add(-time.Hour)
		cases := []struct {
			name     string
			change   func(*oatproxy.OAuthSession)
			eligible bool
		}{
			{"ready", func(*oatproxy.OAuthSession) {}, true},
			{"pending", func(s *oatproxy.OAuthSession) { s.DownstreamAccessToken = ""; s.DownstreamAuthorizationCode = "code" }, false},
			{"upstream revoked", func(s *oatproxy.OAuthSession) { s.RevokedAt = &now }, false},
			{"downstream logged out", func(s *oatproxy.OAuthSession) { s.DownstreamRevokedAt = &now }, false},
			{"expired without refresh", func(s *oatproxy.OAuthSession) { s.UpstreamAccessTokenExp = &past; s.UpstreamRefreshToken = "" }, false},
			{"expired refreshable", func(s *oatproxy.OAuthSession) { s.UpstreamAccessTokenExp = &past }, true},
			{"unexpired without refresh", func(s *oatproxy.OAuthSession) { s.UpstreamRefreshToken = "" }, true},
			{"missing credentials", func(s *oatproxy.OAuthSession) { s.UpstreamAccessToken = ""; s.UpstreamRefreshToken = "" }, false},
			{"missing expiry without refresh", func(s *oatproxy.OAuthSession) { s.UpstreamAccessTokenExp = nil; s.UpstreamRefreshToken = "" }, false},
			{"logout before completion", func(s *oatproxy.OAuthSession) { s.DownstreamAccessToken = ""; s.DownstreamRevokedAt = &now }, false},
		}
		var expected []string
		for _, tc := range cases {
			session := &oatproxy.OAuthSession{
				DID: "did:plc:" + tc.name, Handle: tc.name + ".test",
				DownstreamDPoPJKT: tc.name, DownstreamAccessToken: "downstream",
				UpstreamAccessToken: "upstream", UpstreamAccessTokenExp: &future,
				UpstreamRefreshToken: "refresh",
			}
			tc.change(session)
			require.NoError(t, state.CreateOAuthSession(session.DownstreamDPoPJKT, session))
			for _, identifier := range []string{session.DID, session.Handle} {
				did, err := state.IndexingDID(ctx, identifier)
				require.NoError(t, err, tc.name)
				if tc.eligible {
					require.Equal(t, session.DID, did, tc.name)
				} else {
					require.Empty(t, did, tc.name)
				}
			}
			if tc.eligible {
				expected = append(expected, session.DID)
			}
		}
		dids, err := state.IndexingDIDs(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, expected, dids)

		// One revoked device cannot hide another completed local login; two
		// devices for one user must not schedule two sweeps of that repo.
		require.NoError(t, state.CreateOAuthSession("second-device", &oatproxy.OAuthSession{
			DID: "did:plc:ready", DownstreamDPoPJKT: "second-device",
			DownstreamAccessToken: "second", UpstreamRefreshToken: "refresh",
		}))
		dids, err = state.IndexingDIDs(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, expected, dids)
		require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).
			Where("downstream_dpop_jkt = ?", "ready").Update("downstream_revoked_at", now).Error)
		did, err := state.IndexingDID(ctx, "did:plc:ready")
		require.NoError(t, err)
		require.Equal(t, "did:plc:ready", did)
		require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).
			Where("downstream_dpop_jkt = ?", "second-device").Update("revoked_at", now).Error)
		did, err = state.IndexingDID(ctx, "did:plc:ready")
		require.NoError(t, err)
		require.Empty(t, did)
	})
}
