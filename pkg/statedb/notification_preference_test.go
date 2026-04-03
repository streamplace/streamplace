package statedb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/streamplace"
)

func TestNotificationPreference(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		userDID    := "did:plc:follower111"
		repoDID    := "did:plc:streamer000"

		// default: no record stored, should return nil
		pref, err := state.GetNotificationPreference(ctx, userDID, repoDID)
		require.NoError(t, err)
		require.Nil(t, pref)

		// opt out
		err = state.SetNotificationPreference(ctx, userDID, &streamplace.GraphNotificationPreference{
			RepoDID: repoDID,
			Enabled: false,
		})
		require.NoError(t, err)

		pref, err = state.GetNotificationPreference(ctx, userDID, repoDID)
		require.NoError(t, err)
		require.NotNil(t, pref)
		require.False(t, pref.Enabled)

		// re-enable
		err = state.SetNotificationPreference(ctx, userDID, &streamplace.GraphNotificationPreference{
			RepoDID: repoDID,
			Enabled: true,
		})
		require.NoError(t, err)

		pref, err = state.GetNotificationPreference(ctx, userDID, repoDID)
		require.NoError(t, err)
		require.NotNil(t, pref)
		require.True(t, pref.Enabled)
	})
}

func TestFilterNotificationRecipients(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		repoDID      := "did:plc:streamer000"
		optedIn      := "did:plc:follower111"
		optedOut     := "did:plc:follower222"
		noPreference := "did:plc:follower333"

		err := state.SetNotificationPreference(ctx, optedOut, &streamplace.GraphNotificationPreference{
			RepoDID: repoDID,
			Enabled: false,
		})
		require.NoError(t, err)

		result, err := state.filterNotificationRecipients(ctx, repoDID, []string{optedIn, optedOut, noPreference})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{optedIn, noPreference}, result)
	})
}
