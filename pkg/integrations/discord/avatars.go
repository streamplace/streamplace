package discord

import (
	"context"
	"sync"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/aqhttp"
)

var avatarCache = make(map[string]string)
var avatarCacheMutex = sync.Mutex{}

// getAvatarURL gets the avatar URL for a Bluesky from the public appview
// pretty ugly. we're going to replace this with indexing bluesky profiles
// at some point.
func GetAvatarURL(ctx context.Context, did string) (string, error) {
	avatarCacheMutex.Lock()
	defer avatarCacheMutex.Unlock()

	if avatar, ok := avatarCache[did]; ok {
		return avatar, nil
	}

	xrpc := &xrpc.Client{
		Host:   "https://public.api.bsky.app",
		Client: &aqhttp.Client,
	}

	profile, err := bsky.ActorGetProfile(ctx, xrpc, did)
	if err != nil {
		return "", err
	}

	if profile.Avatar != nil {
		avatarCache[did] = *profile.Avatar
		return *profile.Avatar, nil
	}

	return "", nil
}
