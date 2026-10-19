package statedb

import (
	"testing"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/model"
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

func TestLoadOAuthSessionReadyPDSBinding(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		const did = "did:plc:oauth-ready-binding"
		require.NoError(t, state.model.UpdateRepo(&model.Repo{DID: did, PDS: "https://actual-pds.example"}))
		ready := &oatproxy.OAuthSession{DID: did, DownstreamDPoPJKT: "ready-binding", DownstreamAccessToken: "access", PDSUrl: "https://old-pds.example"}
		require.NoError(t, state.CreateOAuthSession(ready.DownstreamDPoPJKT, ready))
		_, err := state.LoadOAuthSession(ready.DownstreamDPoPJKT)
		require.ErrorContains(t, err, "pds mismatch", "ready sessions must execute the previously unreachable PDS check")
		now := time.Now()
		ready.DownstreamRevokedAt = &now
		require.NoError(t, state.UpdateOAuthSession(ready.DownstreamDPoPJKT, ready))
		_, err = state.LoadOAuthSession(ready.DownstreamDPoPJKT)
		require.ErrorContains(t, err, "pds mismatch", "retained upstream sessions are also PDS-bound")
		state.CLI.UpstreamStation = "did:web:station.example"
		delegated, err := state.LoadOAuthSession(ready.DownstreamDPoPJKT)
		require.NoError(t, err)
		require.Equal(t, ready.PDSUrl, delegated.PDSUrl, "station delegation does not overwrite indexed identity")
		repo, err := state.model.GetRepo(did)
		require.NoError(t, err)
		require.Equal(t, "https://actual-pds.example", repo.PDS)
	})
}

func TestLoadOAuthSessionBeforeInitialSync(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		session := &oatproxy.OAuthSession{DID: "did:plc:new-oauth-user", DownstreamDPoPJKT: "initial-sync", DownstreamAccessToken: "access", PDSUrl: "https://pds.example"}
		require.NoError(t, state.CreateOAuthSession(session.DownstreamDPoPJKT, session))
		loaded, err := state.LoadOAuthSession(session.DownstreamDPoPJKT)
		require.NoError(t, err, "successful login is persisted before initial asynchronous indexing")
		require.Equal(t, session.DID, loaded.DID)
		missing, err := state.LoadOAuthSession("never-created")
		require.NoError(t, err)
		require.Nil(t, missing)
	})
}
