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
				DID:               did,
				DownstreamDPoPJKT: jkt,
				DownstreamScope:   scope,
				UpstreamScope:     scope,
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
