package atproto

import (
	"context"
	"sync"
	"testing"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/reposync"
)

// contiguityTestSync is a synchronizer with nothing but an index: the rev
// tracking runs off the repo row and touches no network.
func contiguityTestSync(t *testing.T) (*ATProtoSynchronizer, model.Model) {
	t.Helper()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	return &ATProtoSynchronizer{Model: mod}, mod
}

func commitEvent(did, since, rev string) *indigoatproto.SyncSubscribeRepos_Commit {
	evt := &indigoatproto.SyncSubscribeRepos_Commit{Repo: did, Rev: rev}
	if since != "" {
		evt.Since = &since
	}
	return evt
}

// syncedRepo is a repo with a completed sync and a history behind it: exactly
// the row a firehose gap must not damage.
func syncedRepo(did, version string) *model.Repo {
	return &model.Repo{
		DID:           did,
		Handle:        "someone.example",
		PDS:           "https://pds.example",
		Version:       version,
		RootCID:       "bafyreiabc",
		BackfillFloor: "3lpfloor00000",
		BackfillDone:  true,
	}
}

// TestTrackCommitRev is the contiguity check itself: the three things a commit
// can be relative to what we hold, and the two kinds of repo it must not touch.
//
// The ops of an event are indexed before this runs in every case -- gap
// included, since fresh data now beats correct data later -- so what is under
// test here is only what the event does to the row.
func TestTrackCommitRev(t *testing.T) {
	ctx := context.Background()
	atsync, mod := contiguityTestSync(t)

	// Contiguous: the commit follows the rev we hold, so we hold its rev now
	// and the chain from our backfill to here is unbroken.
	require.NoError(t, mod.UpdateRepo(syncedRepo("did:plc:chain", "3lprev0000000")))
	atsync.trackCommitRev(ctx, commitEvent("did:plc:chain", "3lprev0000000", "3lprev0000001"))
	got, err := mod.GetRepo("did:plc:chain")
	require.NoError(t, err)
	require.Equal(t, "3lprev0000001", got.Version)
	require.Equal(t, "bafyreiabc", got.RootCID, "only the rev moves")

	// Stale: a redelivery from a second relay, or a commit a backfill already
	// read. The rev must not regress.
	atsync.trackCommitRev(ctx, commitEvent("did:plc:chain", "3lprev0000000", "3lprev0000001"))
	atsync.trackCommitRev(ctx, commitEvent("did:plc:chain", "3lpolder00000", "3lpold0000000"))
	got, err = mod.GetRepo("did:plc:chain")
	require.NoError(t, err)
	require.Equal(t, "3lprev0000001", got.Version, "old news does not move the rev backwards")
	require.Empty(t, got.RepairFrom, "and is not a gap")

	// Gap: a commit from ahead of us that does not follow what we hold. The
	// repo is wedged for repair, with its history intact and the rev the
	// repair has to start from written down.
	require.NoError(t, mod.UpdateRepo(syncedRepo("did:plc:gap", "3lprev0000000")))
	atsync.trackCommitRev(ctx, commitEvent("did:plc:gap", "3lpmissed00000", "3lprev0000009"))
	got, err = mod.GetRepo("did:plc:gap")
	require.NoError(t, err)
	require.Empty(t, got.Version, "a gap wedges the repo so the repair path finds it")
	require.Equal(t, "3lprev0000000", got.RepairFrom)
	require.Equal(t, "bafyreiabc", got.RootCID)
	require.Equal(t, "3lpfloor00000", got.BackfillFloor)
	require.True(t, got.BackfillDone, "an hour of missed commits does not un-index a history")

	// A commit with no Since at all -- the first commit of a repo, or a relay
	// that does not send one -- cannot be proven contiguous, so it is a gap.
	require.NoError(t, mod.UpdateRepo(syncedRepo("did:plc:nosince", "3lprev0000000")))
	atsync.trackCommitRev(ctx, commitEvent("did:plc:nosince", "", "3lprev0000009"))
	got, err = mod.GetRepo("did:plc:nosince")
	require.NoError(t, err)
	require.Empty(t, got.Version)

	// A stranger stays a stranger: the firehose does not create rows.
	atsync.trackCommitRev(ctx, commitEvent("did:plc:stranger", "3lprev0000000", "3lprev0000001"))
	got, err = mod.GetRepo("did:plc:stranger")
	require.NoError(t, err)
	require.Nil(t, got)

	// A repo whose backfill is owed or running is left alone: an empty Version
	// already means "sync me", and the sync will write a rev of its own.
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: "did:plc:wedged", PDS: "https://pds.example"}))
	atsync.trackCommitRev(ctx, commitEvent("did:plc:wedged", "3lprev0000000", "3lprev0000001"))
	got, err = mod.GetRepo("did:plc:wedged")
	require.NoError(t, err)
	require.Empty(t, got.Version)
	require.Empty(t, got.RepairFrom, "an unsynced repo has no gap to repair")

	// An event with no rev is not evidence of anything.
	require.NoError(t, mod.UpdateRepo(syncedRepo("did:plc:norev", "3lprev0000000")))
	atsync.trackCommitRev(ctx, commitEvent("did:plc:norev", "3lpsomething0", ""))
	got, err = mod.GetRepo("did:plc:norev")
	require.NoError(t, err)
	require.Equal(t, "3lprev0000000", got.Version)
}

// TestTrackCommitRevOutOfOrder is the race the CAS exists for: events are
// handled a goroutine each, so a repo's commits arrive in whatever order the
// scheduler feels like. However they interleave, the chain must end at the
// newest rev, and commits that really are contiguous must not be mistaken for a
// gap.
func TestTrackCommitRevOutOfOrder(t *testing.T) {
	ctx := context.Background()
	atsync, mod := contiguityTestSync(t)
	require.NoError(t, mod.UpdateRepo(syncedRepo("did:plc:race", "3lprev0000000")))

	// Three consecutive commits, delivered at once and in no order.
	events := []*indigoatproto.SyncSubscribeRepos_Commit{
		commitEvent("did:plc:race", "3lprev0000000", "3lprev0000001"),
		commitEvent("did:plc:race", "3lprev0000001", "3lprev0000002"),
		commitEvent("did:plc:race", "3lprev0000002", "3lprev0000003"),
	}
	var wg sync.WaitGroup
	for _, evt := range events {
		wg.Add(1)
		go func() {
			defer wg.Done()
			atsync.trackCommitRev(ctx, evt)
		}()
	}
	wg.Wait()

	got, err := mod.GetRepo("did:plc:race")
	require.NoError(t, err)
	if got.Version == "" {
		// A losing interleaving costs a repair, never a record: the repair
		// starts from the rev we did have.
		require.NotEmpty(t, got.RepairFrom)
		return
	}
	require.Equal(t, "3lprev0000003", got.Version)
	require.True(t, got.BackfillDone)
}

// TestRepairFloor: how far back a repair reads. The missed span starts at the
// rev we were last good at, so that -- not "one day ago" -- is where the walk
// has to start, and a repair is never shallower than a first sync.
func TestRepairFloor(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return reposync.TIDForTime(now.Add(d)) }
	floorTime := func(t *testing.T, tid string) time.Time {
		t.Helper()
		parsed, err := reposync.TimeForTID(tid)
		require.NoError(t, err)
		return parsed
	}

	// First contact: one day, exactly as before.
	require.Equal(t, at(-InitialWindow), repairFloor("", now))
	// A rev that is not a TID tells us nothing about when it was.
	require.Equal(t, at(-InitialWindow), repairFloor("not-a-tid", now))
	// A rev from inside the last day: still one day, because a repair must not
	// read less than a first sync would.
	require.Equal(t, at(-InitialWindow), repairFloor(at(-time.Hour), now))
	require.Equal(t, at(-InitialWindow), repairFloor(at(-23*time.Hour), now))

	// A node that was down for a week reads from a week ago, plus the slack,
	// rather than from yesterday -- everything written during those six days
	// carries an rkey from those six days.
	week := repairFloor(at(-7*24*time.Hour), now)
	require.Equal(t, now.Add(-7*24*time.Hour-repairSlack), floorTime(t, week))
	require.True(t, floorTime(t, week).Before(now.Add(-InitialWindow)))
}

// TestMergeBackfillState: a repair is a shallow sync of a repo that may have
// years indexed. Walking a recent window says something true about the last
// day and nothing at all about the years, so the row's history has to survive
// it -- otherwise every gap would send a completed repo back to the top of the
// deepening ladder.
func TestMergeBackfillState(t *testing.T) {
	// Real TIDs, because "deeper" means "sorts earlier" and a made-up string
	// would let the test agree with itself about the wrong order.
	now := time.Now()
	day := reposync.TIDForTime(now.Add(-InitialWindow))
	month := reposync.TIDForTime(now.Add(-30 * 24 * time.Hour))
	hour := reposync.TIDForTime(now.Add(-time.Hour))
	shallow := backfillResult{Floor: day, Done: false}

	// First contact: whatever the walk found.
	floor, done := mergeBackfillState(nil, shallow)
	require.Equal(t, day, floor)
	require.False(t, done)

	// A repo with its whole history, repaired: still complete.
	floor, done = mergeBackfillState(&model.Repo{BackfillDone: true}, shallow)
	require.True(t, done)
	require.Empty(t, floor, "a complete history has no watermark left to hold")

	// A repo mid-ladder keeps the deeper of the two watermarks: it really is
	// indexed from the older one forward.
	floor, done = mergeBackfillState(&model.Repo{BackfillFloor: month}, shallow)
	require.Equal(t, month, floor)
	require.False(t, done)

	// A row whose watermark is shallower than what we just walked keeps the
	// fresh one; a row with no watermark at all contributes nothing.
	floor, _ = mergeBackfillState(&model.Repo{BackfillFloor: hour}, shallow)
	require.Equal(t, day, floor)
	floor, _ = mergeBackfillState(&model.Repo{}, shallow)
	require.Equal(t, day, floor)

	// The full-CAR fallback reads everything, so it completes a repo outright.
	floor, done = mergeBackfillState(&model.Repo{BackfillFloor: month},
		backfillResult{Done: true})
	require.True(t, done)
	require.Empty(t, floor)
}
