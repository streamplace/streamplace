package atproto

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/devenv"
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
	messages, err := mod.MostRecentChatMessages(user.DID)
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
	messages, err = mod.MostRecentChatMessages(user.DID)
	require.NoError(t, err)
	require.Len(t, messages, 2, "the record written during the gap is indexed")

	// And a sweep of a node that is genuinely current is one request per repo
	// and nothing else: no repair, no duplicates.
	require.NoError(t, atsync.Sweep(ctx))
	current, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, hostRev, current.Version)
	messages, err = mod.MostRecentChatMessages(user.DID)
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
