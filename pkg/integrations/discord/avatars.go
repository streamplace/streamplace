package discord

import (
	"context"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"golang.org/x/sync/singleflight"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/aqhttp"
)

// avatarCacheEntry is a cached avatar lookup result. Negative results (no
// avatar, or a fetch error) are cached too, so a chatter without an avatar
// doesn't trigger a network call on every message they send.
type avatarCacheEntry struct {
	url       string
	hasAvatar bool
	err       error
	fetchedAt time.Time
}

// avatarNegativeTTL is how long "no avatar" and fetch-error results stay
// cached before being re-validated. Avatar hits are cached for the process
// lifetime. Variable so tests can shrink it.
var avatarNegativeTTL = time.Minute

var avatarCache = struct {
	sync.RWMutex
	m map[string]avatarCacheEntry
}{m: make(map[string]avatarCacheEntry)}

// avatarFetchGroup collapses concurrent cache misses for the same DID into a
// single fetch.
var avatarFetchGroup singleflight.Group

// fetchAvatarURL fetches the avatar URL for a Bluesky DID from the public
// appview. Variable so tests can stub it.
var fetchAvatarURL = func(ctx context.Context, did string) (string, error) {
	// pretty ugly. we're going to replace this with indexing bluesky profiles
	// at some point.
	xrpc := &xrpc.Client{
		Host:   "https://public.api.bsky.app",
		Client: &aqhttp.Client,
	}

	profile, err := appbsky.ActorGetProfile(ctx, xrpc, did)
	if err != nil {
		return "", err
	}

	if profile.Avatar != nil {
		return *profile.Avatar, nil
	}

	return "", nil
}

// GetAvatarURL gets the avatar URL for a Bluesky DID from the public appview.
//
// Successful avatar URLs are cached for the process lifetime; "no avatar" and
// error results are cached for avatarNegativeTTL so busy chat doesn't hammer
// the appview on every message. Concurrent misses for the same DID collapse
// into a single fetch. The cache lock is only held for map access, never
// across the network call.
func GetAvatarURL(ctx context.Context, did string) (string, error) {
	avatarCache.RLock()
	entry, ok := avatarCache.m[did]
	avatarCache.RUnlock()
	if ok {
		if entry.err != nil {
			if time.Since(entry.fetchedAt) < avatarNegativeTTL {
				return "", entry.err
			}
		} else if entry.hasAvatar || time.Since(entry.fetchedAt) < avatarNegativeTTL {
			return entry.url, nil
		}
		// stale negative entry: fall through to refetch
	}

	v, err, _ := avatarFetchGroup.Do(did, func() (any, error) {
		url, fetchErr := fetchAvatarURL(ctx, did)
		entry := avatarCacheEntry{fetchedAt: time.Now(), err: fetchErr}
		if fetchErr == nil {
			entry.url = url
			entry.hasAvatar = url != ""
		}
		avatarCache.Lock()
		avatarCache.m[did] = entry
		avatarCache.Unlock()
		return url, fetchErr
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}
