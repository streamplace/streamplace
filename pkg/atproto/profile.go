package atproto

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/appbsky"
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
