package badges

import (
	"context"
	"fmt"

	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// GetValidBadges returns valid badges for a user in the context of a streamer's chat.
// Returns server-controlled badges (streamer, mod) based on permissions.
func GetValidBadges(ctx context.Context, userDID, streamerDID, issuerDID string, m indexdb.Model) ([]placestream.BadgeDefs_BadgeView, error) {
	badges := []placestream.BadgeDefs_BadgeView{}

	// If no streamer context, return empty badges
	if streamerDID == "" {
		return badges, nil
	}

	// Check if user is the streamer
	if userDID == streamerDID {
		badges = append(badges, placestream.BadgeDefs_BadgeView{
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
			badges = append(badges, placestream.BadgeDefs_BadgeView{
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
	spChatProfile, err := chatProfile.ToRecord()

	if err != nil {
		return badges, nil
	}

	for _, label := range spChatProfile.SelfLabels {
		if label == constants.SelfLabelBot {
			log.Warn(ctx, "user self-labels as bot", "userDID", userDID)
			badges = append(badges, placestream.BadgeDefs_BadgeView{
				BadgeType: constants.BadgeTypeBot,
				Issuer:    issuerDID,
				Recipient: userDID,
			})
		}
	}

	// Resolve issuance-based badges from the user's badge selections.
	if spChatProfile.Badges != nil {
		// VIP badges: one per streamer channel, only shown in that streamer's chat.
		if streamerDID != "" {
			for _, sel := range spChatProfile.Badges.Streamer {
				if sel.Badge.Uri == "" || sel.Streamer != streamerDID {
					continue
				}
				view, err := resolveIssuanceBadgeView(ctx, sel.Badge.Uri, userDID, m)
				if err != nil || view == nil {
					continue
				}
				if view.BadgeType != constants.BadgeTypeVIP {
					log.Warn(ctx, "streamer slot contains non-VIP badge", "badgeType", view.BadgeType)
					continue
				}
				badges = append(badges, *view)
			}
		}

		// Global badge: shown in any chat, must be issued by an authorized global issuer.
		if spChatProfile.Badges.Global != nil {
			view, err := resolveIssuanceBadgeView(ctx, spChatProfile.Badges.Global.Uri, userDID, m)
			if err != nil || view == nil {
				// logged inside resolveIssuanceBadgeView
			} else if !IsGlobalIssuer(view.Issuer) {
				log.Warn(ctx, "global slot badge not from authorized issuer", "issuer", view.Issuer)
			} else {
				badges = append(badges, *view)
			}
		}
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

// resolveIssuanceBadgeView fetches an issuance by URI, verifies the recipient, and returns a BadgeView.
// Returns nil (with no error) if the issuance is revoked, not indexed, or the recipient doesn't match.
func resolveIssuanceBadgeView(ctx context.Context, uri string, userDID string, m indexdb.Model) (*placestream.BadgeDefs_BadgeView, error) {
	issuance, err := m.GetBadgeIssuanceByURI(ctx, uri)
	if err != nil {
		log.Error(ctx, "failed to get badge issuance", "err", err, "uri", uri)
		return nil, err
	}
	if issuance == nil {
		return nil, nil // revoked or not yet indexed
	}
	if issuance.RecipientDID != userDID {
		log.Warn(ctx, "badge issuance recipient mismatch", "issuanceRecipient", issuance.RecipientDID, "userDID", userDID)
		return nil, nil
	}
	def, err := m.GetBadgeDefByURI(ctx, issuance.BadgeURI)
	if err != nil {
		log.Error(ctx, "failed to get badge def", "err", err, "uri", issuance.BadgeURI)
		return nil, err
	}
	if def == nil {
		return nil, nil // def was deleted
	}
	view := placestream.BadgeDefs_BadgeView{
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
	return &view, nil
}
