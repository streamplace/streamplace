package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNotificationRepoDIDPreserved guards against the regression where
// re-registering a push token without a repoDID wiped out the existing
// association. CreateNotification used to DB.Save() the row, which issues a
// full-row UPDATE and blanked repo_did whenever the client re-registered
// before its OAuth session had restored, silently dropping the device from
// follower livestream blasts.
func TestNotificationRepoDIDPreserved(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		const token = "device-token-1"
		const didA = "did:plc:aaaa"
		const didB = "did:plc:bbbb"

		// Initial registration while logged in associates the DID.
		require.NoError(t, state.CreateNotification(token, didA))
		tokens, err := state.GetManyNotificationTokens([]string{didA})
		require.NoError(t, err)
		require.Equal(t, []string{token}, tokens)

		// Re-registration without a DID (e.g. before the OAuth session has
		// restored) must NOT clobber the existing association.
		require.NoError(t, state.CreateNotification(token, ""))
		tokens, err = state.GetManyNotificationTokens([]string{didA})
		require.NoError(t, err)
		require.Equal(t, []string{token}, tokens, "repo_did was wiped by a DID-less re-registration")

		// ...and it must not have duplicated the row (token is the primary key).
		nots, err := state.ListNotifications()
		require.NoError(t, err)
		require.Len(t, nots, 1)

		// Re-registering with a different DID replaces the association.
		require.NoError(t, state.CreateNotification(token, didB))
		tokens, err = state.GetManyNotificationTokens([]string{didB})
		require.NoError(t, err)
		require.Equal(t, []string{token}, tokens)
		tokens, err = state.GetManyNotificationTokens([]string{didA})
		require.NoError(t, err)
		require.Empty(t, tokens, "old DID association should be replaced")
	})
}

// TestNotificationAnonymousThenAssociated covers a token that first registers
// with no DID (anonymous) and later gets associated once the user logs in,
// without creating a duplicate row.
func TestNotificationAnonymousThenAssociated(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		const token = "device-token-2"
		const did = "did:plc:cccc"

		// Anonymous registration: the row exists but has no association yet.
		require.NoError(t, state.CreateNotification(token, ""))
		tokens, err := state.GetManyNotificationTokens([]string{did})
		require.NoError(t, err)
		require.Empty(t, tokens)
		nots, err := state.ListNotifications()
		require.NoError(t, err)
		require.Len(t, nots, 1)

		// Once logged in, the association is set without adding a new row.
		require.NoError(t, state.CreateNotification(token, did))
		tokens, err = state.GetManyNotificationTokens([]string{did})
		require.NoError(t, err)
		require.Equal(t, []string{token}, tokens)
		nots, err = state.ListNotifications()
		require.NoError(t, err)
		require.Len(t, nots, 1)
	})
}
