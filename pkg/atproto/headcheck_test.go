package atproto

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// TestHeadCheckHealsSilentGap is the test the whole reconciliation loop exists
// for.
//
// An account is indexed to completion. Then a record is written to its repo
// with nobody listening -- no firehose, no event, nothing that would ever tell
// this node the repo moved. That is not a contrived situation: it is a node
// that was down longer than a relay's replay window, and it is every account a
// freshly built index inherits.
//
// Before the head check, the record was invisible forever: the repo had a
// completed backfill, so no sweep would look at it again. After it, one
// getLatestCommit per sweep notices the disagreement, the repo repairs itself
// through the ordinary path, and the record lands -- with the history the repo
// already had still intact.
func TestHeadCheckHealsSilentGap(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	createBackfillRecord(t, user, "place.stream.chat.message", "", chatMessageRecord(user.DID, "before"))
	require.NoError(t, atsync.StatefulDB.AddRepo(user.DID))
	require.NoError(t, untilNoErrors(t, func() error {
		paths, err := walkAll(ctx, dev, user.DID, backfillRanges(""))
		if err != nil {
			return err
		}
		if len(paths) != 2 {
			return fmt.Errorf("PDS has %d records, want 2", len(paths))
		}
		return nil
	}), "waiting for the repo to settle")

	require.NoError(t, atsync.Sweep(ctx))
	indexed, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, indexed.Version)
	require.True(t, indexed.BackfillDone, "the sweep read the whole repo")
	messages, err := mod.MostRecentChatMessages(user.DID, time.Time{})
	require.NoError(t, err)
	require.Len(t, messages, 1)

	// Behind our back: no firehose is running in this test, so nothing at all
	// tells the index that this happened.
	createBackfillRecord(t, user, "place.stream.chat.message", "", chatMessageRecord(user.DID, "after the gap"))
	require.NoError(t, untilNoErrors(t, func() error {
		paths, err := walkAll(ctx, dev, user.DID, backfillRanges(""))
		if err != nil {
			return err
		}
		if len(paths) != 3 {
			return fmt.Errorf("PDS has %d records, want 3", len(paths))
		}
		return nil
	}), "waiting for the new record to commit")

	// Proof that the gap is real before we heal it.
	stale, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, indexed.Version, stale.Version, "nothing has told the index anything")
	hostRev, _, err := atsync.headRev(ctx, user.DID)
	require.NoError(t, err)
	require.NotEqual(t, stale.Version, hostRev, "the repo really did move")

	// The sweep's head-check pass finds the drift and repairs it.
	require.NoError(t, atsync.Sweep(ctx))

	healed, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, hostRev, healed.Version, "the repair caught the index up to the host")
	require.True(t, healed.BackfillDone, "repairing a day of history does not un-index the rest")
	require.Empty(t, healed.RepairFrom, "the repair it asked for is the one that ran")
	messages, err = mod.MostRecentChatMessages(user.DID, time.Time{})
	require.NoError(t, err)
	require.Len(t, messages, 2, "the record written during the gap is indexed")

	// And a sweep of a node that is genuinely current is one request per repo
	// and nothing else: no repair, no duplicates.
	require.NoError(t, atsync.Sweep(ctx))
	current, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, hostRev, current.Version)
	messages, err = mod.MostRecentChatMessages(user.DID, time.Time{})
	require.NoError(t, err)
	require.Len(t, messages, 2)
}

// TestSweepRefreshesDriftedIdentity is the identity flavour of the silent-gap
// test above: a handle or PDS change whose #identity event this node never saw
// (down past the replay window, or a deliberately skipped replay). Records get
// healed by the head check's rev comparison; the row's identity columns must be
// healed by the same pass, or they stay wrong until the account happens to emit
// another identity event.
func TestSweepRefreshesDriftedIdentity(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	require.NoError(t, atsync.StatefulDB.AddRepo(user.DID))
	require.NoError(t, atsync.Sweep(ctx))
	indexed, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, indexed.Version)
	require.NotEmpty(t, indexed.Handle)

	// Behind our back: from this node's point of view, an identity change it
	// never heard about is exactly a row that disagrees with the directory.
	require.NoError(t, mod.UpdateRepoIdentity(user.DID, "stale.handle.invalid", "https://old-pds.example.invalid"))

	require.NoError(t, atsync.Sweep(ctx))
	healed, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, indexed.Handle, healed.Handle, "the sweep healed the drifted handle")
	require.Equal(t, indexed.PDS, healed.PDS, "and the drifted PDS")
	require.Equal(t, indexed.Version, healed.Version, "identity refresh left the sync state alone")
	require.Equal(t, indexed.BackfillDone, healed.BackfillDone)
}

// The sweep must heal broken handles, missed renames, and a stale cache even
// when the indexed row is already current. Still-broken accounts remain
// reachable by DID, never by the shared sentinel.
func TestSweepHealsIdentityBehindCache(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	fixed, renamed, broken, current := dev.CreateAccount(t), dev.CreateAccount(t), dev.CreateAccount(t), dev.CreateAccount(t)
	users := []*devenv.DevEnvAccount{fixed, renamed, broken, current}
	for _, user := range users {
		require.NoError(t, atsync.StatefulDB.AddRepo(user.DID))
	}
	require.NoError(t, atsync.Sweep(ctx))
	indexed := make(map[string]*model.Repo, len(users))
	for _, user := range users {
		row, err := mod.GetRepo(user.DID)
		require.NoError(t, err)
		require.NotEmpty(t, row.Version)
		indexed[user.DID] = row
	}

	realDirectory := atsync.directory(false)
	authoritative := identity.NewMockDirectory()
	atsync.PLCDirectory = &authoritative
	atsync.CachedPLCDirectory = nil
	for _, tc := range []struct {
		user                      *devenv.DevEnvAccount
		recorded, cached, current syntax.Handle
	}{
		{fixed, syntax.HandleInvalid, syntax.HandleInvalid, "fixed.example.test"},
		{renamed, "old.example.test", "old.example.test", "renamed.example.test"},
		{broken, syntax.HandleInvalid, syntax.HandleInvalid, syntax.HandleInvalid},
		{current, "current.example.test", "cached.example.test", "current.example.test"},
	} {
		ident, err := realDirectory.LookupDID(ctx, syntax.DID(tc.user.DID))
		require.NoError(t, err)
		was, now := *ident, *ident
		was.Handle, now.Handle = tc.cached, tc.current
		authoritative.Insert(was)
		_, err = atsync.resolveIdent(ctx, tc.user.DID, true)
		require.NoError(t, err)
		authoritative.Insert(now)
		require.NoError(t, mod.UpdateRepoIdentity(tc.user.DID, tc.recorded.String(), ident.PDSEndpoint()))
	}
	// A parked previous owner is never swept, so the current owner's refresh
	// must release its stale claim even when the current row already agrees.
	const parkedDID = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID: parkedDID, Handle: "current.example.test", Status: model.RepoStatusDeactivated,
	}))
	require.NoError(t, atsync.StatefulDB.AddRepo(parkedDID))

	byHandle, err := mod.GetRepoByHandleOrDID("handle.invalid")
	require.NoError(t, err)
	require.Nil(t, byHandle, "a broken handle names nobody")

	require.NoError(t, atsync.Sweep(ctx))

	for did, want := range map[string]string{
		fixed.DID:   "fixed.example.test",
		renamed.DID: "renamed.example.test",
		broken.DID:  "handle.invalid",
		current.DID: "current.example.test",
	} {
		got, err := mod.GetRepo(did)
		require.NoError(t, err)
		require.Equal(t, want, got.Handle)
		require.Equal(t, indexed[did].Version, got.Version, "identity refresh left the sync state alone")
		require.Equal(t, indexed[did].BackfillDone, got.BackfillDone)
		byDID, err := mod.GetRepoByHandleOrDID(did)
		require.NoError(t, err)
		require.NotNil(t, byDID, "reachable by DID")
		resolved, err := atsync.resolveIdent(ctx, did, true)
		require.NoError(t, err)
		require.Equal(t, want, resolved.Handle.String(), "cached consumers see the repaired identity")
	}
	byHandle, err = mod.GetRepoByHandleOrDID("renamed.example.test")
	require.NoError(t, err)
	require.NotNil(t, byHandle)
	require.Equal(t, renamed.DID, byHandle.DID)
	byHandle, err = mod.GetRepoByHandleOrDID("current.example.test")
	require.NoError(t, err)
	require.NotNil(t, byHandle)
	require.Equal(t, current.DID, byHandle.DID)
	parked, err := mod.GetRepo(parkedDID)
	require.NoError(t, err)
	require.Empty(t, parked.Handle)
	require.Equal(t, model.RepoStatusDeactivated, parked.Status)
	byHandle, err = mod.GetRepoByHandleOrDID("old.example.test")
	require.NoError(t, err)
	require.Nil(t, byHandle, "the old handle no longer routes anywhere")
	byHandle, err = mod.GetRepoByHandleOrDID("handle.invalid")
	require.NoError(t, err)
	require.Nil(t, byHandle)
}

// TestResetIdentCacheDropsCachedResolutions proves the reset is real: a warm
// cache serves hits until resetIdentCache (what dropping a stale cursor
// calls), after which the next resolve goes back to the directory. Combined
// with TestSweepRefreshesDriftedIdentity above — a resolution that disagrees
// with the row heals it — this is the full path by which an identity change
// hidden inside a skipped replay reaches the row: the reset makes the healing
// sweep's resolutions fresh, and a fresh resolution that disagrees with the
// row rewrites it.
//
// (A true end-to-end rename is not observable in the dev environment: handle
// verification cannot complete there, so every account resolves as
// handle.invalid and a rename changes nothing the directory reports.)
func TestResetIdentCacheDropsCachedResolutions(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, _ := backfillTestSynchronizer(t, dev)
	user := dev.CreateAccount(t)

	_, err := atsync.resolveIdent(ctx, user.DID, true)
	require.NoError(t, err)
	did, err := syntax.ParseDID(user.DID)
	require.NoError(t, err)
	cd, ok := atsync.directory(true).(*identity.CacheDirectory)
	require.True(t, ok)
	_, hit, err := cd.LookupDIDWithCacheState(ctx, did)
	require.NoError(t, err)
	require.True(t, hit, "the resolve above warmed the cache")

	atsync.resetIdentCache()

	cd, ok = atsync.directory(true).(*identity.CacheDirectory)
	require.True(t, ok)
	_, hit, err = cd.LookupDIDWithCacheState(ctx, did)
	require.NoError(t, err)
	require.False(t, hit, "the reset cache resolves from the directory again")
}
