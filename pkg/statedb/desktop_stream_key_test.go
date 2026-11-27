package statedb

import (
	"testing"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/spkey"
)

func TestDesktopIngestKey(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		_, err := state.DesktopIngestKey(ctx)
		require.ErrorContains(t, err, "log in to Streamplace Desktop")
		did := "did:plc:desktop"
		priv, pub, err := spkey.GenerateStreamKeyForDID(did)
		require.NoError(t, err)
		require.NoError(t, state.DB.Create(&DesktopStreamKey{DID: did, PrivateKey: priv, SigningKey: pub.DIDKey(), RKey: "desktop-key", CreatedAt: time.Now().UTC(), Published: true}).Error)
		for _, jkt := range []string{"device-one", "device-two"} {
			require.NoError(t, state.CreateOAuthSession(jkt, &oatproxy.OAuthSession{DID: did, DownstreamDPoPJKT: jkt, DownstreamAccessToken: jkt, UpstreamRefreshToken: "refresh"}))
			key, err := state.DesktopIngestKey(ctx)
			require.NoError(t, err)
			require.Equal(t, priv, key)
		}
		indexed, err := state.model.GetSigningKey(ctx, pub.DIDKey(), did)
		require.NoError(t, err)
		require.NotNil(t, indexed)
		require.Equal(t, "desktop-key", indexed.RKey)
		require.NoError(t, state.CreateOAuthSession("other", &oatproxy.OAuthSession{DID: "did:plc:other", DownstreamDPoPJKT: "other", DownstreamAccessToken: "other", UpstreamRefreshToken: "refresh"}))
		_, err = state.DesktopIngestKey(ctx)
		require.ErrorContains(t, err, "more than one account logged in")
		require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).Where("downstream_dpop_jkt = ?", "other").Update("downstream_revoked_at", time.Now()).Error)
		key, err := state.DesktopIngestKey(ctx)
		require.NoError(t, err)
		require.Equal(t, priv, key)

		now := time.Now().UTC()
		indexed.RevokedAt = &now
		require.NoError(t, state.model.UpdateSigningKey(indexed))
		require.NoError(t, state.DB.Model(&oatproxy.OAuthSession{}).Where("repo_did = ?", did).Update("downstream_revoked_at", now).Error)
		_, err = state.DesktopStreamKey(ctx, did)
		require.ErrorContains(t, err, "get desktop session")
		var replacement DesktopStreamKey
		require.NoError(t, state.DB.First(&replacement, "did = ?", did).Error)
		require.NotEqual(t, priv, replacement.PrivateKey)
		require.NotEqual(t, indexed.DID, replacement.SigningKey)
		require.NotEqual(t, indexed.RKey, replacement.RKey)
		require.False(t, replacement.Published)
		_, err = state.model.GetSigningKey(ctx, indexed.DID, did)
		require.ErrorContains(t, err, "signing key revoked")
	})
}
