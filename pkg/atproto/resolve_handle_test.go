package atproto

import (
	"context"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/devenv"
)

// TestResolveAuthorHandleReadsThroughCacheDirectory verifies that
// ResolveAuthorHandle is served from CacheDirectory, so purgeIdentCache
// invalidates it. (Dev accounts resolve as handle.invalid, so this tests
// the purge mechanism rather than a real rename.)
func TestResolveAuthorHandleReadsThroughCacheDirectory(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, _ := backfillTestSynchronizer(t, dev)
	user := dev.CreateAccount(t)

	// First call warms the CacheDirectory.
	handle := atsync.ResolveAuthorHandle(ctx, user.DID)
	require.NotEmpty(t, handle, "first resolve should return a handle")

	did, err := syntax.ParseDID(user.DID)
	require.NoError(t, err)

	cd, ok := atsync.directory(true).(*identity.CacheDirectory)
	require.True(t, ok)
	_, hit, err := cd.LookupDIDWithCacheState(ctx, did)
	require.NoError(t, err)
	require.True(t, hit, "ResolveAuthorHandle should warm the CacheDirectory")

	// Purge, then confirm the next lookup is a cache miss.
	atsync.purgeIdentCache(ctx, user.DID)
	_, hit, err = cd.LookupDIDWithCacheState(ctx, did)
	require.NoError(t, err)
	require.False(t, hit, "purgeIdentCache should evict the entry")

	// Second call still resolves successfully after purge.
	handle2 := atsync.ResolveAuthorHandle(ctx, user.DID)
	require.NotEmpty(t, handle2, "resolve after purge should still succeed")
	require.Equal(t, handle, handle2, "same identity, same handle")
}
