package badges

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

func TestGetValidBadges(t *testing.T) {
	ctx := context.Background()

	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)

	issuerDID := "did:web:node.example.com"
	streamerDID := "did:plc:streamer"
	moderatorDID := "did:plc:moderator"
	regularUserDID := "did:plc:regular"

	t.Run("returns empty when no streamer context", func(t *testing.T) {
		badges, err := GetValidBadges(ctx, regularUserDID, "", issuerDID, mod)
		require.NoError(t, err)
		require.Empty(t, badges)
	})

	t.Run("returns streamer badge for streamer", func(t *testing.T) {
		badges, err := GetValidBadges(ctx, streamerDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, badges, 1)
		require.Equal(t, constants.BadgeTypeStreamer, badges[0].BadgeType)
		require.Equal(t, issuerDID, badges[0].Issuer)
		require.Equal(t, streamerDID, badges[0].Recipient)
	})

	t.Run("returns no badges for regular user", func(t *testing.T) {
		badges, err := GetValidBadges(ctx, regularUserDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Empty(t, badges)
	})

	t.Run("returns mod badge for moderator", func(t *testing.T) {
		// Grant moderation permissions
		perm := &streamplace.ModerationPermission{
			LexiconTypeID: "place.stream.moderation.permission",
			Moderator:     moderatorDID,
			Permissions:   []string{"ban", "hide"},
			CreatedAt:     time.Now().Format(util.ISO8601),
		}
		aturi, err := syntax.ParseATURI("at://" + streamerDID + "/place.stream.moderation.permission/test123")
		require.NoError(t, err)

		err = mod.CreateModerationDelegation(ctx, perm, aturi)
		require.NoError(t, err)

		badges, err := GetValidBadges(ctx, moderatorDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, badges, 1)
		require.Equal(t, constants.BadgeTypeMod, badges[0].BadgeType)
		require.Equal(t, issuerDID, badges[0].Issuer)
		require.Equal(t, moderatorDID, badges[0].Recipient)
	})

	t.Run("streamer badge takes priority over mod", func(t *testing.T) {
		// Even if streamer has mod permissions for themselves, they get streamer badge
		badges, err := GetValidBadges(ctx, streamerDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, badges, 1)
		require.Equal(t, constants.BadgeTypeStreamer, badges[0].BadgeType)
	})
}
