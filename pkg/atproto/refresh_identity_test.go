package atproto

import (
	"context"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
)

// RefreshIdentity should drop the cached identity entry,
// so the next cached resolve picks up a new PDS/handle
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

// RefreshIdentity is an identity write, not a sync: it must carry nothing but
// handle and PDS, leave an indexed repo's sync state untouched, and not index
// an account just because somebody asked about it (the XRPC endpoint takes any
// identifier).
func TestRefreshIdentityWritesOnlyIdentity(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)
	user := dev.CreateAccount(t)
	stranger := dev.CreateAccount(t)

	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:           user.DID,
		Handle:        "stale.example",
		PDS:           "https://old-pds.example.invalid",
		Version:       "3lrev00000000",
		RootCID:       "bafyroot",
		Status:        model.RepoStatusDeactivated,
		BackfillFloor: "3lfloor0000000",
		BackfillDone:  true,
		RepairFrom:    "3lrepair000000",
	}))

	id, err := atsync.RefreshIdentity(ctx, user.DID)
	require.NoError(t, err)
	got, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, id.Handle.String(), got.Handle)
	require.Equal(t, id.PDSEndpoint(), got.PDS)
	require.Equal(t, "3lrev00000000", got.Version)
	require.Equal(t, "bafyroot", got.RootCID)
	require.Equal(t, model.RepoStatusDeactivated, got.Status)
	require.Equal(t, "3lfloor0000000", got.BackfillFloor)
	require.True(t, got.BackfillDone)
	require.Equal(t, "3lrepair000000", got.RepairFrom)

	_, err = atsync.RefreshIdentity(ctx, stranger.DID)
	require.NoError(t, err)
	row, err := mod.GetRepo(stranger.DID)
	require.NoError(t, err)
	require.Nil(t, row, "refreshing an identity does not index the account")
}

// A refresh whose authoritative lookup fails must change nothing: the row keeps
// the identity it has, and the cached entry is not purged, so the failure
// neither blanks a good handle nor forces every reader back to the directory.
func TestRefreshIdentityFailureLeavesRowAndCache(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)
	user := dev.CreateAccount(t)

	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     user.DID,
		Handle:  "kept.example",
		PDS:     "https://kept-pds.example",
		Version: "3lrev00000000",
	}))
	_, err := atsync.resolveIdent(ctx, user.DID, true)
	require.NoError(t, err)
	did, err := syntax.ParseDID(user.DID)
	require.NoError(t, err)
	cd, ok := atsync.directory(true).(*identity.CacheDirectory)
	require.True(t, ok)

	// The directory behind the authoritative path stops knowing the account;
	// the cache built on the original one still does.
	empty := identity.NewMockDirectory()
	atsync.PLCDirectory = &empty

	_, err = atsync.RefreshIdentity(ctx, user.DID)
	require.Error(t, err)

	got, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, "kept.example", got.Handle)
	require.Equal(t, "https://kept-pds.example", got.PDS)
	_, hit, err := cd.LookupDIDWithCacheState(ctx, did)
	require.NoError(t, err)
	require.True(t, hit, "a failed refresh leaves the cached identity alone")
}
