package atproto

import (
	"context"
	"testing"
	"time"

	bsky "stream.place/streamplace/pkg/appbsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

func TestAddModBadge(t *testing.T) {
	ctx := context.Background()

	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)

	streamerDID := "did:plc:streamer"
	moderatorDID := "did:plc:moderator"
	issuerDID := "did:web:example.com"

	// Create a chat message
	message := placestream.ChatDefs_MessageView{
		LexiconTypeID: "place.stream.chat.defs#messageView",
		Uri:           "at://test/place.stream.chat.message/123",
		Cid:           "test-cid",
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Did:    moderatorDID,
			Handle: "moderator.test",
		},
		IndexedAt: "2024-01-01T00:00:00Z",
	}

	t.Run("no badge when user is not a moderator", func(t *testing.T) {
		msg := *message // copy
		err := AddModBadgeIfApplicable(ctx, &msg, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Nil(t, msg.Badges, "should not have badges when user is not a moderator")
	})

	t.Run("adds streamer badge when user is the streamer", func(t *testing.T) {
		msg := *message // copy
		msg.Author = appbsky.ActorDefs_ProfileViewBasic{
			Did:    streamerDID,
			Handle: "streamer.test",
		}
		err := AddModBadgeIfApplicable(ctx, &msg, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, msg.Badges, 1, "should have 1 badge when user is the streamer")
		require.Equal(t, constants.BadgeTypeStreamer, msg.Badges[0].BadgeType)
		require.Equal(t, issuerDID, msg.Badges[0].Issuer)
		require.Equal(t, streamerDID, msg.Badges[0].Recipient)
	})

	t.Run("adds mod badge when user has moderation permissions", func(t *testing.T) {
		// Grant moderation permissions to the moderator
		perm := placestream.ModerationPermission{
			LexiconTypeID: "place.stream.moderation.permission",
			Moderator:     moderatorDID,
			Permissions:   []string{"ban", "hide"},
			CreatedAt:     time.Now().Format(util.ISO8601),
		}
		aturi, err := syntax.ParseATURI("at://" + streamerDID + "/place.stream.moderation.permission/test123")
		require.NoError(t, err)

		// Sync the permission to the model
		err = mod.CreateModerationDelegation(ctx, perm, aturi)
		require.NoError(t, err)

		msg := *message // copy
		err = AddModBadgeIfApplicable(ctx, &msg, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, msg.Badges, 1, "should have 1 badge when user is a moderator")
		require.Equal(t, constants.BadgeTypeMod, msg.Badges[0].BadgeType)
		require.Equal(t, issuerDID, msg.Badges[0].Issuer)
		require.Equal(t, moderatorDID, msg.Badges[0].Recipient)
	})

	t.Run("prepends mod badge to existing badges", func(t *testing.T) {
		// Create message with existing user-settable badge
		msg := *message // copy
		msg.Badges = []placestream.BadgeDefs_BadgeView{
			{
				BadgeType: constants.BadgeTypeVIP,
				Issuer:    "did:web:other.com",
				Recipient: moderatorDID,
			},
		}

		err = AddModBadgeIfApplicable(ctx, &msg, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, msg.Badges, 2, "should have 2 badges")
		require.Equal(t, constants.BadgeTypeMod, msg.Badges[0].BadgeType, "mod badge should be first")
		require.Equal(t, constants.BadgeTypeVIP, msg.Badges[1].BadgeType, "vip badge should be second")
	})
}
