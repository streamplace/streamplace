package atproto

import (
	"context"
	"fmt"

	"stream.place/streamplace/pkg/badges"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// AddModBadgeIfApplicable checks if a message author has mod permissions for the streamer
// and adds a mod or streamer badge as the first badge (server-controlled).
// - If the author is the streamer, adds a "streamer" badge
// - If the author has moderation permissions, adds a "mod" badge
// - If the author self-labels as a bot, adds a "bot" badge
func AddModBadgeIfApplicable(ctx context.Context, message *streamplace.ChatDefs_MessageView, streamerDID string, issuerDID string, m model.Model) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	authorDID := message.Author.Did

	// Get valid badges for this user
	validBadges, err := badges.GetValidBadges(ctx, authorDID, streamerDID, issuerDID, m)
	if err != nil {
		return err
	}

	// Prepend server-controlled badges (first badge slot is reserved for server)
	if len(validBadges) > 0 {
		if message.Badges == nil {
			message.Badges = validBadges
		} else {
			message.Badges = append(validBadges, message.Badges...)
		}
	}

	return nil
}
