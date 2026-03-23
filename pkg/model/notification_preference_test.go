package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotificationPreference(t *testing.T) {
	mod, err := MakeDB(":memory:")
	require.NoError(t, err)

	ctx := context.Background()
	userDID     := "did:plc:user111"
	streamerDID := "did:plc:streamer222"

	// no row = nil (caller treats as enabled)
	pref, err := mod.GetNotificationPreference(ctx, userDID, streamerDID)
	require.NoError(t, err)
	require.Nil(t, pref)

	// opt out
	err = mod.SetNotificationPreference(ctx, userDID, streamerDID, false)
	require.NoError(t, err)

	pref, err = mod.GetNotificationPreference(ctx, userDID, streamerDID)
	require.NoError(t, err)
	require.NotNil(t, pref)
	require.False(t, pref.Enabled)

	// re-enable
	err = mod.SetNotificationPreference(ctx, userDID, streamerDID, true)
	require.NoError(t, err)

	pref, err = mod.GetNotificationPreference(ctx, userDID, streamerDID)
	require.NoError(t, err)
	require.NotNil(t, pref)
	require.True(t, pref.Enabled)
}
