package statedb

import (
	"testing"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// literals instead of the pkg/atproto constants: that package imports
// statedb, so the test can't import it back
const (
	scopeBskyActorStatus = "repo?collection=app.bsky.actor.status"
	scopeBskyPostCreate  = "repo?collection=app.bsky.feed.post&action=create"
)

func TestGetSessionByDIDWithScope(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		did := "did:plc:scopetest"
		noBskyScope := "atproto blob:*/* include:place.stream.authFull"
		fullScope := noBskyScope + " " + scopeBskyPostCreate + " " + scopeBskyActorStatus

		mkSession := func(jkt, scope string, updatedAt time.Time, revoked bool) {
			session := &oatproxy.OAuthSession{
				DID:                   did,
				DownstreamDPoPJKT:     jkt,
				DownstreamScope:       scope,
				UpstreamScope:         scope,
				DownstreamAccessToken: "at-" + jkt,
			}
			if revoked {
				now := time.Now()
				session.RevokedAt = &now
			}
			require.NoError(t, state.CreateOAuthSession(jkt, session))
			require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).
				Where("downstream_dpop_jkt = ?", jkt).
				Update("updated_at", updatedAt).Error)
		}

		// no sessions at all
		_, err := state.GetSessionByDIDWithScope(did, scopeBskyActorStatus)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)

		// newest session declined the Bluesky scopes
		mkSession("jkt-declined", noBskyScope, time.Now(), false)
		_, err = state.GetSessionByDIDWithScope(did, scopeBskyActorStatus)
		require.ErrorIs(t, err, ErrNoSessionWithScope)

		// ...but it still satisfies the streamplace scope
		got, err := state.GetSessionByDIDWithScope(did, "include:place.stream.authFull")
		require.NoError(t, err)
		require.Equal(t, "jkt-declined", got.DownstreamDPoPJKT)

		// an older full-scope session on another device gets picked for
		// Bluesky writes even though the declined one is newer
		mkSession("jkt-full", fullScope, time.Now().Add(-time.Hour), false)
		got, err = state.GetSessionByDIDWithScope(did, scopeBskyActorStatus)
		require.NoError(t, err)
		require.Equal(t, "jkt-full", got.DownstreamDPoPJKT)

		// plain GetSessionByDID still returns the newest session
		got, err = state.GetSessionByDID(did)
		require.NoError(t, err)
		require.Equal(t, "jkt-declined", got.DownstreamDPoPJKT)

		// revoked sessions don't count
		mkSession("jkt-revoked", fullScope, time.Now().Add(time.Hour), true)
		got, err = state.GetSessionByDIDWithScope(did, scopeBskyActorStatus)
		require.NoError(t, err)
		require.Equal(t, "jkt-full", got.DownstreamDPoPJKT)

		// legacy sessions with no recorded scope count as full grants
		legacyDID := "did:plc:legacy"
		legacy := &oatproxy.OAuthSession{
			DID:               legacyDID,
			DownstreamDPoPJKT: "jkt-legacy",
		}
		require.NoError(t, state.CreateOAuthSession("jkt-legacy", legacy))
		got, err = state.GetSessionByDIDWithScope(legacyDID, scopeBskyActorStatus)
		require.NoError(t, err)
		require.Equal(t, "jkt-legacy", got.DownstreamDPoPJKT)
	})
}

// A linked session (QR login) borrows its root's upstream session, so a
// server-side job acting as the user must get the root even when the linked
// one was used more recently. The root is a row from before the column
// existed: its upstream_jkt is NULL rather than empty.
func TestGetSessionByDIDSkipsLinkedSessions(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		did := "did:plc:linktest"
		require.NoError(t, state.CreateOAuthSession("jkt-root", &oatproxy.OAuthSession{
			DID:               did,
			DownstreamDPoPJKT: "jkt-root",
		}))
		require.NoError(t, state.CreateOAuthSession("jkt-linked", &oatproxy.OAuthSession{
			DID:               did,
			DownstreamDPoPJKT: "jkt-linked",
			UpstreamJKT:       "jkt-root",
		}))
		require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).
			Where("downstream_dpop_jkt = ?", "jkt-root").
			Update("upstream_jkt", gorm.Expr("NULL")).Error)
		require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).
			Where("downstream_dpop_jkt = ?", "jkt-linked").
			Update("updated_at", time.Now().Add(time.Hour)).Error)

		got, err := state.GetSessionByDID(did)
		require.NoError(t, err)
		require.Equal(t, "jkt-root", got.DownstreamDPoPJKT)

		got, err = state.GetSessionByDIDWithScope(did, scopeBskyActorStatus)
		require.NoError(t, err)
		require.Equal(t, "jkt-root", got.DownstreamDPoPJKT)
	})
}

// Pushed authorization requests come without authentication, so making one
// prunes sessions that never logged anyone in; nothing else.
func TestCreateOAuthSessionPrunesAbandonedSessions(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		old := time.Now().Add(-2 * time.Hour)
		mk := func(session *oatproxy.OAuthSession, updatedAt time.Time) {
			require.NoError(t, state.CreateOAuthSession(session.DownstreamDPoPJKT, session))
			require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).
				Where("downstream_dpop_jkt = ?", session.DownstreamDPoPJKT).
				Update("updated_at", updatedAt).Error)
		}
		// a PAR nobody followed, and a QR login approved but never redeemed
		mk(&oatproxy.OAuthSession{DownstreamDPoPJKT: "jkt-abandoned", DownstreamPARRequestURI: "urn:abandoned"}, old)
		mk(&oatproxy.OAuthSession{DownstreamDPoPJKT: "jkt-unredeemed", DID: "did:plc:prune", UpstreamJKT: "jkt-ready", DownstreamAuthorizationCode: "code"}, old)
		// a login under way
		mk(&oatproxy.OAuthSession{DownstreamDPoPJKT: "jkt-recent", DownstreamPARRequestURI: "urn:recent"}, time.Now())
		// logged in, and logged out with upstream still good
		mk(&oatproxy.OAuthSession{DownstreamDPoPJKT: "jkt-ready", DID: "did:plc:prune", DownstreamAccessToken: "at", UpstreamRefreshToken: "rt"}, old)
		mk(&oatproxy.OAuthSession{DownstreamDPoPJKT: "jkt-ready-upstream", DID: "did:plc:prune", UpstreamRefreshToken: "rt"}, old)

		require.NoError(t, state.CreateOAuthSession("jkt-new", &oatproxy.OAuthSession{DownstreamDPoPJKT: "jkt-new"}))

		var left []string
		require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).
			Order("downstream_dpop_jkt").Pluck("downstream_dpop_jkt", &left).Error)
		require.Equal(t, []string{"jkt-new", "jkt-ready", "jkt-ready-upstream", "jkt-recent"}, left)
	})
}
