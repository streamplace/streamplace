package atproto

import (
	"context"
	"fmt"

	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

// AddModBadgeIfApplicable checks if a message author has mod permissions for the streamer
// and adds a mod or streamer badge as the first badge (server-controlled).
// - If the author is the streamer, adds a "streamer" badge
// - If the author has moderation permissions, adds a "mod" badge
func AddModBadgeIfApplicable(ctx context.Context, message *streamplace.ChatDefs_MessageView, streamerDID string, issuerDID string, m model.Model) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	authorDID := message.Author.Did

	var badge *streamplace.BadgeDefs_BadgeView

	// Check if author is the streamer
	if authorDID == streamerDID {
		badge = &streamplace.BadgeDefs_BadgeView{
			BadgeType: constants.BadgeTypeStreamer,
			Issuer:    issuerDID,
			Recipient: authorDID,
		}
	} else {
		// Check if author has any moderation permissions for the streamer
		delegations, err := m.GetModerationDelegations(ctx, streamerDID, authorDID)
		if err != nil {
			log.Error(ctx, "failed to get moderation delegations", "err", err, "authorDID", authorDID, "streamerDID", streamerDID)
			return err
		}

		// If the author has any delegations (meaning they're a moderator), add a mod badge
		if len(delegations) > 0 {
			badge = &streamplace.BadgeDefs_BadgeView{
				BadgeType: constants.BadgeTypeMod,
				Issuer:    issuerDID,
				Recipient: authorDID,
			}
		}
	}

	// Prepend the badge if one was created (server-controlled badge is first)
	if badge != nil {
		if message.Badges == nil {
			message.Badges = []*streamplace.BadgeDefs_BadgeView{badge}
		} else {
			message.Badges = append([]*streamplace.BadgeDefs_BadgeView{badge}, message.Badges...)
		}
	}

	return nil
}
