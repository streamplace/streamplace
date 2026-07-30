package badges

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/placestream"
)

func TestGetValidBadges(t *testing.T) {
	ctx := context.Background()

	mod, err := indexdb.MakeDB(":memory:")
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
		perm := placestream.ModerationPermission{
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

func TestGetValidBadges_Issuance(t *testing.T) {
	ctx := context.Background()

	mod, err := indexdb.MakeDB(":memory:")
	require.NoError(t, err)

	issuerDID := "did:web:node.example.com"
	streamerDID := "did:plc:streamer2"
	vipUserDID := "did:plc:vipuser"
	otherStreamerDID := "did:plc:otherstreamer"

	defURI := "at://" + streamerDID + "/place.stream.badge.def/def001"
	issuanceURI := "at://" + streamerDID + "/place.stream.badge.issuance/iss001"

	setupVIPIssuance := func(t *testing.T, recipientDID string) {
		t.Helper()
		upsertBadgeDef(t, ctx, mod, defURI, "Streamer VIP", constants.BadgeTypeVIP)
		upsertBadgeIssuance(t, ctx, mod, issuanceURI, recipientDID, defURI)
	}

	t.Run("vip badge appears when issuance and selection match", func(t *testing.T) {
		setupVIPIssuance(t, vipUserDID)

		profile := buildProfileWithStreamerBadge(t, streamerDID, comatproto.RepoStrongRef{Uri: issuanceURI, Cid: "bafyiss"})
		upsertProfile(t, ctx, mod, vipUserDID, profile)

		badges, err := GetValidBadges(ctx, vipUserDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, badges, 1)
		require.Equal(t, constants.BadgeTypeVIP, badges[0].BadgeType)
		require.Equal(t, streamerDID, badges[0].Issuer)
		require.Equal(t, vipUserDID, badges[0].Recipient)
		require.NotNil(t, badges[0].Name)
		require.Equal(t, "Streamer VIP", *badges[0].Name)
	})

	t.Run("vip badge scoped to issuing streamer chat only", func(t *testing.T) {
		badges, err := GetValidBadges(ctx, vipUserDID, otherStreamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Empty(t, badges)
	})

	t.Run("vip badge absent without streamer context", func(t *testing.T) {
		badges, err := GetValidBadges(ctx, vipUserDID, "", issuerDID, mod)
		require.NoError(t, err)
		require.Empty(t, badges)
	})

	t.Run("badge rejected when issuance recipient does not match user", func(t *testing.T) {
		wrongIssuanceURI := "at://" + streamerDID + "/place.stream.badge.issuance/wrongiss"
		upsertBadgeIssuance(t, ctx, mod, wrongIssuanceURI, "did:plc:someoneelse", defURI)

		theftUserDID := "did:plc:theftuser"
		profile := buildProfileWithStreamerBadge(t, streamerDID, comatproto.RepoStrongRef{Uri: wrongIssuanceURI, Cid: "bafywrong"})
		upsertProfile(t, ctx, mod, theftUserDID, profile)

		badges, err := GetValidBadges(ctx, theftUserDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Empty(t, badges)
	})

	t.Run("badge disappears after issuance is deleted", func(t *testing.T) {
		err := mod.DeleteBadgeIssuance(ctx, issuanceURI)
		require.NoError(t, err)

		badges, err := GetValidBadges(ctx, vipUserDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Empty(t, badges)
	})

	t.Run("badge disappears after badge def is deleted", func(t *testing.T) {
		// Re-create issuance, then delete the def
		upsertBadgeIssuance(t, ctx, mod, issuanceURI, vipUserDID, defURI)

		err = mod.DeleteBadgeDef(ctx, defURI)
		require.NoError(t, err)

		badges, err := GetValidBadges(ctx, vipUserDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Empty(t, badges)
	})

	t.Run("event badge issued by node appears in any streamer context", func(t *testing.T) {
		// Register the node as an authorized global badge issuer for this test.
		constants.GlobalBadgeIssuers = append(constants.GlobalBadgeIssuers, issuerDID)
		t.Cleanup(func() {
			constants.GlobalBadgeIssuers = constants.GlobalBadgeIssuers[:len(constants.GlobalBadgeIssuers)-1]
		})

		eventUserDID := "did:plc:eventuser"
		eventDefURI := "at://" + issuerDID + "/place.stream.badge.def/eventdef"
		eventIssuanceURI := "at://" + issuerDID + "/place.stream.badge.issuance/eventiss"

		upsertBadgeDef(t, ctx, mod, eventDefURI, "Contest Winner", constants.BadgeTypeEvent)
		upsertBadgeIssuance(t, ctx, mod, eventIssuanceURI, eventUserDID, eventDefURI)

		profile := buildProfileWithGlobalBadge(t, comatproto.RepoStrongRef{Uri: eventIssuanceURI, Cid: "bafyeventiss"})
		upsertProfile(t, ctx, mod, eventUserDID, profile)

		// Appears in streamer's chat
		badges, err := GetValidBadges(ctx, eventUserDID, streamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, badges, 1)
		require.Equal(t, constants.BadgeTypeEvent, badges[0].BadgeType)

		// Also appears in a different streamer's chat (globally valid)
		badges, err = GetValidBadges(ctx, eventUserDID, otherStreamerDID, issuerDID, mod)
		require.NoError(t, err)
		require.Len(t, badges, 1)
		require.Equal(t, constants.BadgeTypeEvent, badges[0].BadgeType)
	})
}

func buildProfileWithStreamerBadge(t *testing.T, streamerDID string, ref comatproto.RepoStrongRef) *placestream.ChatProfile {
	t.Helper()
	return &placestream.ChatProfile{
		LexiconTypeID: "place.stream.chat.profile",
		Badges: &placestream.ChatProfile_BadgeSelections{
			Streamer: []placestream.ChatProfile_StreamerBadgeSelection{
				{Streamer: streamerDID, Badge: ref},
			},
		},
	}
}

func buildProfileWithGlobalBadge(t *testing.T, ref comatproto.RepoStrongRef) *placestream.ChatProfile {
	t.Helper()
	return &placestream.ChatProfile{
		LexiconTypeID: "place.stream.chat.profile",
		Badges: &placestream.ChatProfile_BadgeSelections{
			Global: &ref,
		},
	}
}

func upsertProfile(t *testing.T, ctx context.Context, mod indexdb.Model, ownerDID string, rec *placestream.ChatProfile) {
	t.Helper()
	aturi, err := syntax.ParseATURI("at://" + ownerDID + "/place.stream.chat.profile/self")
	require.NoError(t, err)
	require.NoError(t, mod.UpsertChatProfile(ctx, *rec, aturi))
}

func upsertBadgeDef(t *testing.T, ctx context.Context, mod indexdb.Model, uri, name, badgeType string) {
	t.Helper()
	aturi, err := syntax.ParseATURI(uri)
	require.NoError(t, err)
	require.NoError(t, mod.UpsertBadgeDef(ctx, placestream.BadgeDef{
		LexiconTypeID: "place.stream.badge.def",
		Name:          name,
		BadgeType:     badgeType,
	}, aturi))
}

func upsertBadgeIssuance(t *testing.T, ctx context.Context, mod indexdb.Model, uri, recipientDID, badgeURI string) {
	t.Helper()
	aturi, err := syntax.ParseATURI(uri)
	require.NoError(t, err)
	require.NoError(t, mod.UpsertBadgeIssuance(ctx, placestream.BadgeIssuance{
		LexiconTypeID: "place.stream.badge.issuance",
		Did:           recipientDID,
		Badge:         comatproto.RepoStrongRef{Uri: badgeURI},
	}, aturi))
}
