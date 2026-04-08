package badges

import (
	"context"

	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

// GetValidBadges returns valid badges for a user in the context of a streamer's chat.
// Returns server-controlled badges (streamer, mod) based on permissions.
func GetValidBadges(ctx context.Context, userDID, streamerDID, issuerDID string, m model.Model) ([]*streamplace.BadgeDefs_BadgeView, error) {
	badges := []*streamplace.BadgeDefs_BadgeView{}

	// If no streamer context, return empty badges
	if streamerDID == "" {
		return badges, nil
	}

	// Check if user is the streamer
	if userDID == streamerDID {
		badges = append(badges, &streamplace.BadgeDefs_BadgeView{
			BadgeType: constants.BadgeTypeStreamer,
			Issuer:    issuerDID,
			Recipient: userDID,
		})
		return badges, nil
	}

	// Check if user has moderation permissions for this streamer
	delegations, err := m.GetModerationDelegations(ctx, streamerDID, userDID)
	if err != nil {
		log.Error(ctx, "failed to get moderation delegations", "err", err, "userDID", userDID, "streamerDID", streamerDID)
		return nil, err
	}

	// If user has any delegations, they're a moderator
	if len(delegations) > 0 {
		badges = append(badges, &streamplace.BadgeDefs_BadgeView{
			BadgeType: constants.BadgeTypeMod,
			Issuer:    issuerDID,
			Recipient: userDID,
		})
	}

	// if user "self-labels" as a bot (in chat profile), add bot badge
	chatProfile, err := m.GetChatProfile(ctx, userDID)
	if err != nil || chatProfile == nil {
		return badges, nil
	}
	spChatProfile, err := chatProfile.ToStreamplaceChatProfile()

	if err != nil || spChatProfile == nil {
		return badges, nil
	}

	for _, label := range spChatProfile.SelfLabels {
		if *label == constants.SelfLabelBot {
			log.Warn(ctx, "user self-labels as bot", "userDID", userDID)
			badges = append(badges, &streamplace.BadgeDefs_BadgeView{
				BadgeType: constants.BadgeTypeBot,
				Issuer:    issuerDID,
				Recipient: userDID,
			})
		}
	}

	// TODO: Add badge issuance records when implemented
	// - Query place.stream.badge.issuance records for this user
	// - Verify signatures if issuer is not the current node
	// - Add VIP badges, subscriber badges, etc.

	return badges, nil
}
