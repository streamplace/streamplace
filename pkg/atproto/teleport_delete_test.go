package atproto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
)

func TestDeleteTeleportWhenLookupFails(t *testing.T) {
	ctx := context.Background()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	dbModel := mod.(*model.DBModel)
	db, err := dbModel.DB.DB()
	require.NoError(t, err)
	defer db.Close()

	const uri = "at://did:plc:streamer/place.stream.live.teleport/1"
	require.NoError(t, mod.CreateTeleport(ctx, &model.Teleport{
		URI:       uri,
		CID:       "bafyreiteleport",
		RepoDID:   "did:plc:streamer",
		TargetDID: "did:plc:target",
	}))

	// The teleport lookup preloads its repo associations. Removing that table
	// makes the lookup fail while leaving the teleports table available to delete.
	require.NoError(t, dbModel.DB.Migrator().DropTable(&model.Repo{}))
	_, err = mod.GetTeleportByURI(uri)
	require.Error(t, err)

	atsync := &ATProtoSynchronizer{Model: mod}
	atsync.handleTeleportDelete(ctx, uri)

	got, err := mod.GetTeleportByURI(uri)
	require.NoError(t, err)
	require.Nil(t, got)
}
