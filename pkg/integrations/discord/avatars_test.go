package discord

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// resetAvatarCache clears the global avatar cache between tests.
func resetAvatarCache() {
	avatarCache.Lock()
	avatarCache.m = make(map[string]avatarCacheEntry)
	avatarCache.Unlock()
}

func TestGetAvatarURLCachesHits(t *testing.T) {
	resetAvatarCache()
	defer resetAvatarCache()

	var calls int32
	original := fetchAvatarURL
	fetchAvatarURL = func(ctx context.Context, did string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "https://example.com/" + did + ".png", nil
	}
	defer func() { fetchAvatarURL = original }()

	ctx := context.Background()
	url, err := GetAvatarURL(ctx, "did:example:alice")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/did:example:alice.png", url)

	// Second call is served from cache, no refetch.
	url, err = GetAvatarURL(ctx, "did:example:alice")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/did:example:alice.png", url)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestGetAvatarURLCachesMisses(t *testing.T) {
	resetAvatarCache()
	defer resetAvatarCache()

	var calls int32
	original := fetchAvatarURL
	fetchAvatarURL = func(ctx context.Context, did string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", nil
	}
	defer func() { fetchAvatarURL = original }()

	ctx := context.Background()
	url, err := GetAvatarURL(ctx, "did:example:noavatar")
	require.NoError(t, err)
	require.Empty(t, url)

	// The "no avatar" result is cached, so a second message doesn't refetch.
	url, err = GetAvatarURL(ctx, "did:example:noavatar")
	require.NoError(t, err)
	require.Empty(t, url)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestGetAvatarURLCachesErrors(t *testing.T) {
	resetAvatarCache()
	defer resetAvatarCache()

	oldTTL := avatarNegativeTTL
	avatarNegativeTTL = time.Hour
	defer func() { avatarNegativeTTL = oldTTL }()

	var calls int32
	original := fetchAvatarURL
	fetchAvatarURL = func(ctx context.Context, did string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", errors.New("appview down")
	}
	defer func() { fetchAvatarURL = original }()

	ctx := context.Background()
	_, err := GetAvatarURL(ctx, "did:example:errored")
	require.Error(t, err)

	// The error is cached within the TTL, so a second message doesn't refetch.
	_, err = GetAvatarURL(ctx, "did:example:errored")
	require.Error(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestGetAvatarURLRefetchesAfterNegativeTTL(t *testing.T) {
	resetAvatarCache()
	defer resetAvatarCache()

	oldTTL := avatarNegativeTTL
	avatarNegativeTTL = 10 * time.Millisecond
	defer func() { avatarNegativeTTL = oldTTL }()

	var calls int32
	original := fetchAvatarURL
	fetchAvatarURL = func(ctx context.Context, did string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", nil
	}
	defer func() { fetchAvatarURL = original }()

	ctx := context.Background()
	_, _ = GetAvatarURL(ctx, "did:example:stale")
	_, _ = GetAvatarURL(ctx, "did:example:stale")
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))

	time.Sleep(30 * time.Millisecond)
	_, _ = GetAvatarURL(ctx, "did:example:stale")
	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestGetAvatarURLAvatarHitsIgnoreTTL(t *testing.T) {
	resetAvatarCache()
	defer resetAvatarCache()

	oldTTL := avatarNegativeTTL
	avatarNegativeTTL = 10 * time.Millisecond
	defer func() { avatarNegativeTTL = oldTTL }()

	var calls int32
	original := fetchAvatarURL
	fetchAvatarURL = func(ctx context.Context, did string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "https://example.com/avatar.png", nil
	}
	defer func() { fetchAvatarURL = original }()

	ctx := context.Background()
	_, _ = GetAvatarURL(ctx, "did:example:hasavatar")

	time.Sleep(30 * time.Millisecond)
	url, err := GetAvatarURL(ctx, "did:example:hasavatar")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/avatar.png", url)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestGetAvatarURLSingleflight(t *testing.T) {
	resetAvatarCache()
	defer resetAvatarCache()

	var calls int32
	release := make(chan struct{})
	original := fetchAvatarURL
	fetchAvatarURL = func(ctx context.Context, did string) (string, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return "https://example.com/avatar.png", nil
	}
	defer func() { fetchAvatarURL = original }()

	ctx := context.Background()
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			url, err := GetAvatarURL(ctx, "did:example:concurrent")
			if err != nil {
				results <- err
				return
			}
			if url != "https://example.com/avatar.png" {
				results <- errors.New("unexpected url: " + url)
				return
			}
			results <- nil
		}()
	}

	// Wait until the single flight is in flight, then release it.
	require.Eventually(t, func() bool { return atomic.LoadInt32(&calls) == 1 }, time.Second, time.Millisecond*5)
	close(release)
	for i := 0; i < 10; i++ {
		require.NoError(t, <-results)
	}

	// All 10 callers shared one fetch.
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
