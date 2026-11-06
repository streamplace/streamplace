package atproto

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

var ErrUserNotFound = errors.New("user not found")

func (atsync *ATProtoSynchronizer) FetchUserProfile(ctx context.Context, username string) (*appbsky.ActorDefs_ProfileViewDetailed, error) {
	// Use ATSync to resolve username to DID, then fetch full profile from Bluesky
	var actor string

	// First try to resolve via internal DB
	repo, err := atsync.Model.GetRepoByHandleOrDID(username)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUserNotFound, err)
	} else if repo != nil {
		// Use the DID as it's the most reliable identifier
		actor = repo.DID
	} else {
		return nil, fmt.Errorf("no repo found for username: %s (%w)", username, ErrUserNotFound)
	}

	// Fetch full profile from Bluesky public API
	client := &xrpc.Client{
		Host: "https://public.api.bsky.app",
	}

	profile, err := appbsky.ActorGetProfile(ctx, client, actor)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile from Bluesky for '%s': %w", actor, err)
	}

	if profile == nil {
		return nil, fmt.Errorf("received nil profile from Bluesky API for '%s'", actor)
	}

	return profile, nil

}

// DecorateAuthorProfile adds indexed Bluesky metadata to a newly built view.
// Call before publishing: bus subscribers share the published view.
func (atsync *ATProtoSynchronizer) DecorateAuthorProfile(ctx context.Context, author *appbsky.ActorDefs_ProfileViewBasic) {
	if author == nil || author.Did == "" {
		return
	}
	profile, err := atsync.Model.GetBskyProfile(ctx, author.Did, false)
	if err != nil {
		log.Error(ctx, "could not hydrate author profile", "did", author.Did, "error", err)
		return
	}
	if profile == nil {
		return
	}
	author.DisplayName = profile.DisplayName
	author.Pronouns = profile.Pronouns
	author.Avatar = nil
	if profile.Avatar != nil {
		avatar := fmt.Sprintf("https://cdn.bsky.app/img/avatar/plain/%s/%s@jpeg", author.Did, profile.Avatar.Ref.String())
		author.Avatar = &avatar
	}
}

// DecorateChatProfiles also hydrates authors embedded in reply previews.
func (atsync *ATProtoSynchronizer) DecorateChatProfiles(ctx context.Context, message *placestream.ChatDefs_MessageView) {
	for message != nil {
		atsync.DecorateAuthorProfile(ctx, &message.Author)
		if message.ReplyTo == nil {
			break
		}
		message = message.ReplyTo.ChatDefs_MessageView
	}
}
