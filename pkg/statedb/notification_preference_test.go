package statedb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterNotificationRecipients(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		streamerDID  := "did:plc:streamer000"
		optedIn      := "did:plc:follower111"
		optedOut     := "did:plc:follower222"
		noPreference := "did:plc:follower333"

		err := state.model.SetNotificationPreference(ctx, optedOut, streamerDID, false)
		require.NoError(t, err)

		result, err := state.filterNotificationRecipients(ctx, streamerDID, []string{optedIn, optedOut, noPreference})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{optedIn, noPreference}, result)
	})
}
