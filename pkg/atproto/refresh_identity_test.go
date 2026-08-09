package atproto

import (
	"context"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/devenv"
)

// TestRefreshIdentityPurgesCache proves that RefreshIdentity drops the cached
// identity entry, so the next cached resolve picks up a new PDS/handle instead
// of serving a stale entry for up to 24h. Without the purge, a PDS migration
// would leave every cached resolve pointing at the dead host.
func TestRefreshIdentityPurgesCache(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, _ := backfillTestSynchronizer(t, dev)
	user := dev.CreateAccount(t)

	// Warm the cache with a cached resolve.
	_, err := atsync.resolveIdent(ctx, user.DID, true)
	require.NoError(t, err)

	did, err := syntax.ParseDID(user.DID)
	require.NoError(t, err)

	cd, ok := atsync.directory(true).(*identity.CacheDirectory)
	require.True(t, ok)
	_, hit, err := cd.LookupDIDWithCacheState(ctx, did)
	require.NoError(t, err)
	require.True(t, hit, "cache should be warm before refresh")

	// RefreshIdentity should purge the cached entry.
	_, err = atsync.RefreshIdentity(ctx, user.DID)
	require.NoError(t, err)

	_, hit, err = cd.LookupDIDWithCacheState(ctx, did)
	require.NoError(t, err)
	require.False(t, hit, "RefreshIdentity should purge the cached identity")
}
