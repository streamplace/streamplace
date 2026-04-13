package badges

import (
	"context"
	"fmt"

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
	}

	// Check if user has moderation permissions for this streamer
	if userDID != streamerDID {
		delegations, err := m.GetModerationDelegations(ctx, streamerDID, userDID)
		if err != nil {
			log.Error(ctx, "failed to get moderation delegations", "err", err, "userDID", userDID, "streamerDID", streamerDID)
			return nil, err
		}

		if len(delegations) > 0 {
			badges = append(badges, &streamplace.BadgeDefs_BadgeView{
				BadgeType: constants.BadgeTypeMod,
				Issuer:    issuerDID,
				Recipient: userDID,
			})
		}
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

	// Resolve issuance-based badges from the user's badge selection.
	for _, ref := range spChatProfile.Selection {
		if ref == nil {
			continue
		}
		issuance, err := m.GetBadgeIssuanceByURI(ctx, ref.Uri)
		if err != nil {
			log.Error(ctx, "failed to get badge issuance", "err", err, "uri", ref.Uri)
			continue
		}
		if issuance == nil {
			continue // revoked or not yet indexed
		}
		if issuance.RecipientDID != userDID {
			log.Warn(ctx, "badge issuance recipient mismatch", "issuanceRecipient", issuance.RecipientDID, "userDID", userDID)
			continue
		}

		def, err := m.GetBadgeDefByURI(ctx, issuance.BadgeURI)
		if err != nil {
			log.Error(ctx, "failed to get badge def", "err", err, "uri", issuance.BadgeURI)
			continue
		}
		if def == nil {
			continue // def was deleted
		}

		switch def.BadgeType {
		case constants.BadgeTypeVIP:
			// VIP badges are streamer-scoped: only shown in the granting streamer's chat.
			if streamerDID == "" || issuance.RepoDID != streamerDID {
				continue
			}
		default:
			// All other badge types (event, unknown) are globally valid but must be
			// issued by an authorized global badge issuer.
			if !IsGlobalIssuer(issuance.RepoDID) {
				continue
			}
		}

		view := &streamplace.BadgeDefs_BadgeView{
			BadgeType: def.BadgeType,
			Issuer:    issuance.RepoDID,
			Recipient: userDID,
		}
		if def.Name != "" {
			view.Name = &def.Name
		}
		if def.Description != "" {
			view.Description = &def.Description
		}
		if def.ImageCID != "" {
			imageUrl := fmt.Sprintf("https://cdn.bsky.app/img/feed_fullsize/plain/%s/%s@png", def.RepoDID, def.ImageCID)
			view.ImageUrl = &imageUrl
		}
		badges = append(badges, view)
	}

	return badges, nil
}

func IsGlobalIssuer(did string) bool {
	for _, authorized := range constants.GlobalBadgeIssuers {
		if did == authorized {
			return true
		}
	}
	return false
}
