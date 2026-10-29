package atproto

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
)

// mockIdentity is an identity as a mock directory reports it: a DID, the
// handle that verifies for it, and the PDS it lives on.
func mockIdentity(did, handle, pds string) identity.Identity {
	return identity.Identity{
		DID:         syntax.DID(did),
		Handle:      syntax.Handle(handle),
		AlsoKnownAs: []string{"at://" + handle},
		Services:    map[string]identity.ServiceEndpoint{"atproto_pds": {Type: "AtprotoPersonalDataServer", URL: pds}},
	}
}

// gatedDirectory parks every lookup until the test answers it, so a test can
// hold one refresh mid-resolve while another one runs.
type gatedDirectory struct {
	identity.Directory
	lookups chan chan *identity.Identity
}

func (d *gatedDirectory) Lookup(ctx context.Context, _ syntax.AtIdentifier) (*identity.Identity, error) {
	reply := make(chan *identity.Identity)
	d.lookups <- reply
	return <-reply, nil
}

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

// TestRefreshIdentitySerializesWriters: the sweep's head check and a firehose
// #identity event can both refresh one account at the same time. The one that
// resolved first used to be able to write last -- a resolution held across a
// slow directory answer, or the head check's wait on its host -- leaving the
// older identity on the row. A refresh must not resolve until the one before
// it has written, so the last write is also the latest answer.
func TestRefreshIdentitySerializesWriters(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)
	const did = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID: did, Handle: "old.example", PDS: "https://old-pds.example", Version: "3lrev00000000",
	}))
	inner := identity.NewMockDirectory()
	gate := &gatedDirectory{Directory: &inner, lookups: make(chan chan *identity.Identity)}
	atsync.PLCDirectory = gate
	stale := mockIdentity(did, "old.example", "https://old-pds.example")
	fresh := mockIdentity(did, "new.example", "https://new-pds.example")

	errs := make(chan error, 2)
	refresh := func(identifier string) {
		_, err := atsync.RefreshIdentity(ctx, identifier)
		errs <- err
	}
	// The first refresh is mid-resolve: the directory has been asked and has
	// not answered.
	go refresh(did)
	first := <-gate.lookups
	// A handle request first discovers the same DID, then must wait its turn
	// instead of resolving the DID while the first refresh is still writing.
	go refresh("new.example")
	(<-gate.lookups) <- &fresh
	select {
	case <-gate.lookups:
		t.Fatal("a second refresh resolved while the first was still mid-write")
	case <-time.After(100 * time.Millisecond):
	}
	first <- &stale
	(<-gate.lookups) <- &stale // ownership is checked under the handle's lock
	require.NoError(t, <-errs)
	(<-gate.lookups) <- &fresh
	(<-gate.lookups) <- &fresh
	require.NoError(t, <-errs)

	got, err := mod.GetRepo(did)
	require.NoError(t, err)
	require.Equal(t, "new.example", got.Handle, "the later resolution is the last one written")
	require.Equal(t, "https://new-pds.example", got.PDS)
	require.Equal(t, "3lrev00000000", got.Version, "identity writes leave the sync state alone")
}

// TestRefreshIdentityPurgesEvictedOwner: when a handle changes hands and the
// new owner's refresh releases the old owner's claim, the old owner's cached
// identity still names it by that handle, and the cached handle lookup still
// points at it. Both are wrong the moment the row is, and every chat author
// line goes through that cache.
func TestRefreshIdentityPurgesEvictedOwner(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)
	const old, next = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa", "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID: old, Handle: "alice.example", PDS: "https://pds.example", Version: "3lrev00000000",
	}))
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID: next, PDS: "https://pds.example", Version: "3lrev00000001",
	}))
	dir := identity.NewMockDirectory()
	dir.Insert(mockIdentity(old, "alice.example", "https://pds.example"))
	atsync.PLCDirectory = &dir
	// Warm the cache with the old owner, by DID and by handle.
	require.Equal(t, "alice.example", atsync.ResolveAuthorHandle(ctx, old))
	// Behind the cache, the handle moves.
	dir.Insert(mockIdentity(old, "bob.example", "https://pds.example"))
	dir.Insert(mockIdentity(next, "alice.example", "https://pds.example"))

	_, err := atsync.RefreshIdentity(ctx, next)
	require.NoError(t, err)

	evicted, err := mod.GetRepo(old)
	require.NoError(t, err)
	require.Empty(t, evicted.Handle, "the old owner's row no longer claims the handle")
	require.Equal(t, "3lrev00000000", evicted.Version)
	require.Equal(t, "bob.example", atsync.ResolveAuthorHandle(ctx, old),
		"the evicted owner's cached handle went with its claim")
	byHandle, err := atsync.resolveIdent(ctx, "alice.example", true)
	require.NoError(t, err)
	require.Equal(t, next, byHandle.DID.String(), "a cached lookup by name no longer stops at the old owner")
}

// TestSyncBlueskyRepoCachedSettlesAmbiguousHandle: two indexed rows claim one
// handle (a new owner indexed while the old owner's row is stale). The lookup
// by handle used to come back empty, and the sync that followed asked the
// cached directory -- which still named the old owner -- and served its repo.
// The directory's authoritative answer settles ownership instead, and nothing
// about either account's sync state moves.
func TestSyncBlueskyRepoCachedSettlesAmbiguousHandle(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)
	const old, next = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa", "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	// Both rows are complete, so a sync of either would be a no-op; a sync of
	// anyone else would reach for a PDS that is not there and fail.
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID: old, Handle: "alice.example", PDS: "https://pds.example", Version: "3lrev00000000", BackfillDone: true,
	}))
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID: next, Handle: "alice.example", PDS: "https://pds.example", Version: "3lrev00000001",
	}))
	dir := identity.NewMockDirectory()
	dir.Insert(mockIdentity(old, "alice.example", "https://pds.example"))
	atsync.PLCDirectory = &dir
	warm, err := atsync.resolveIdent(ctx, "alice.example", true)
	require.NoError(t, err)
	require.Equal(t, old, warm.DID.String(), "the cache names the old owner")
	dir.Insert(mockIdentity(old, syntax.HandleInvalid.String(), "https://pds.example"))
	dir.Insert(mockIdentity(next, "alice.example", "https://pds.example"))

	got, err := atsync.SyncBlueskyRepoCached(ctx, "alice.example")
	require.NoError(t, err)
	require.Equal(t, next, got.DID, "the handle routes to the account it verifies for")
	require.Equal(t, "3lrev00000001", got.Version, "settling ownership is not a sync")

	settled, err := mod.GetRepoByHandleOrDID("alice.example")
	require.NoError(t, err)
	require.Equal(t, next, settled.DID, "ownership is settled for every later lookup")
	loser, err := mod.GetRepo(old)
	require.NoError(t, err)
	require.Empty(t, loser.Handle)
	require.Equal(t, "3lrev00000000", loser.Version)
	require.True(t, loser.BackfillDone, "the loser keeps its sync state")

	// The sentinel never names anyone, however many rows carry it.
	_, err = atsync.SyncBlueskyRepoCached(ctx, syntax.HandleInvalid.String())
	require.Error(t, err)
}

type delayedIdentityDirectory struct {
	identity.Directory
	stale   identity.Identity
	entered chan struct{}
	release chan struct{}
}

func (d *delayedIdentityDirectory) Lookup(ctx context.Context, id syntax.AtIdentifier) (*identity.Identity, error) {
	if id.String() != d.stale.DID.String() {
		return d.Directory.Lookup(ctx, id)
	}
	close(d.entered)
	select {
	case <-d.release:
		return &d.stale, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestRefreshIdentityRejectsDelayedOldOwner(t *testing.T) {
	atsync, mod, _ := offlineSynchronizer(t)
	const old, next = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa", "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: old, Handle: "alice.example", Version: "3lrev00000000"}))
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: next, Version: "3lrev00000001"}))
	dir := identity.NewMockDirectory()
	dir.Insert(mockIdentity(old, "alice.example", "https://pds.example"))
	delayed := &delayedIdentityDirectory{
		Directory: &dir, stale: mockIdentity(old, "alice.example", "https://pds.example"),
		entered: make(chan struct{}), release: make(chan struct{}),
	}
	atsync.PLCDirectory = delayed
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := atsync.RefreshIdentity(ctx, old)
		done <- err
	}()
	<-delayed.entered
	dir.Insert(mockIdentity(old, "handle.invalid", "https://pds.example"))
	dir.Insert(mockIdentity(next, "alice.example", "https://pds.example"))
	_, err := atsync.RefreshIdentity(ctx, next)
	require.NoError(t, err)
	close(delayed.release)
	require.Error(t, <-done, "the old resolution cannot reclaim a handle that changed owners")
	owner, err := mod.GetRepoByHandle("alice.example")
	require.NoError(t, err)
	require.Equal(t, next, owner.DID)
	loser, err := mod.GetRepo(old)
	require.NoError(t, err)
	require.Empty(t, loser.Handle)
}

func TestSyncBlueskyRepoCachedRejectsChangedHandle(t *testing.T) {
	atsync, mod, _ := offlineSynchronizer(t)
	const old, next = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa", "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	for _, did := range []string{old, next} {
		require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, Handle: "alice.example", Version: "3lrev00000000"}))
	}
	inner := identity.NewMockDirectory()
	gate := &gatedDirectory{Directory: &inner, lookups: make(chan chan *identity.Identity)}
	atsync.PLCDirectory = gate
	done := make(chan error, 1)
	go func() {
		_, err := atsync.SyncBlueskyRepoCached(t.Context(), "alice.example")
		done <- err
	}()
	first := mockIdentity(old, "alice.example", "https://pds.example")
	(<-gate.lookups) <- &first
	last := mockIdentity(old, "handle.invalid", "https://pds.example")
	(<-gate.lookups) <- &last
	require.Error(t, <-done, "a handle request must not return a DID that no longer verifies for it")
}
