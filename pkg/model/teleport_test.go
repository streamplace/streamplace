package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetPendingTeleportForRepo(t *testing.T) {
	db := indexedTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	teleports := []Teleport{
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/expired",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(-time.Minute),
		},
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/3lteleport1",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(2 * time.Minute),
		},
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/3lteleport2",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(time.Minute),
		},
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/denied",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(2 * time.Minute),
			Denied:   true,
		},
	}
	for i := range teleports {
		require.NoError(t, db.CreateTeleport(ctx, &teleports[i]))
	}

	got, err := db.GetPendingTeleportForRepo("did:plc:streamer")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, teleports[2].URI, got.URI)
}

func TestGetPendingTeleportForRepoReturnsNilWhenNoneArePending(t *testing.T) {
	db := indexedTestDB(t)

	got, err := db.GetPendingTeleportForRepo("did:plc:streamer")
	require.NoError(t, err)
	require.Nil(t, got)
}
