package atproto

import (
	"context"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/reposync"
)

// revCASAttempts is how many times a commit tries to place itself on the repo
// row before concluding it found a gap.
//
// One attempt is the whole story when commits arrive in order. Within a single
// relay they now do -- the parallel scheduler chains same-DID events through a
// per-repo queue -- but this node consumes several relays at once, each with a
// scheduler of its own, and the deduper only guarantees that a given commit is
// handled once, not that two commits on one repo are handled in order. So they
// still race, and the older one can win the CAS after the newer one has already
// missed it. A second look then finds the row exactly where the newer commit
// expected it. Three is one more than that story needs; a lost race past it
// costs one unnecessary repair, never a missed record.
const revCASAttempts = 3

// repairSlack is how far before the last known rev a repair starts reading.
//
// The rev is a TID stamped by the repo's PDS, and the floor it becomes is
// compared against rkeys stamped by that same PDS, so this is not correcting
// for a clock difference between us and them. It covers the gap between when a
// record's rkey was minted and when the commit carrying it was stamped, plus
// any host whose clock has been stepped backwards since.
const repairSlack = time.Hour

// trackCommitRev keeps this node's idea of a repo's revision honest, and is
// what makes the firehose a checkable stream rather than a hope.
//
// A #commit says which rev it follows (Since) and which rev it creates (Rev).
// Called after the event's ops have been indexed, this either advances the
// stored rev -- proving that we have applied every commit for this repo in an
// unbroken chain -- or discovers that we cannot prove it, and marks the repo
// for repair.
//
// Repos we do not track, and repos in the middle of a backfill, are left
// entirely alone: an empty stored Version already means "sync me", and the
// backfill about to finish will write a rev of its own.
func (atsync *ATProtoSynchronizer) trackCommitRev(ctx context.Context, evt *indigoatproto.SyncSubscribeRepos_Commit) {
	if evt.Rev == "" {
		return
	}
	since := ""
	if evt.Since != nil {
		since = *evt.Since
	}

	for attempt := 0; attempt < revCASAttempts; attempt++ {
		// The happy path is one statement and no read: if the row still holds
		// the rev this commit follows, this commit is the next one.
		applied, err := atsync.Model.AdvanceRepoVersion(ctx, evt.Repo, since, evt.Rev)
		if err != nil {
			log.Error(ctx, "failed to advance repo rev", "did", evt.Repo, "err", err)
			return
		}
		if applied {
			return
		}

		// It did not apply. Find out which of the four reasons it was.
		row, err := atsync.Model.GetRepo(evt.Repo)
		if err != nil {
			log.Error(ctx, "failed to read repo rev", "did", evt.Repo, "err", err)
			return
		}
		switch {
		case row == nil || row.Version == "" || syncInFlight(evt.Repo):
			// A stranger, or a repo whose backfill is running or owed. Nothing
			// here is better than what that backfill will write.
			return
		case row.Version == since:
			// The rev this commit follows arrived while we were looking: an
			// out-of-order sibling won the CAS after ours missed it. Try again.
			continue
		case evt.Rev <= row.Version:
			// Old news -- a redelivery, or a commit we already have by way of
			// a backfill. The ops were indexed idempotently; the rev stays put
			// rather than regressing.
			return
		}

		// evt.Rev is ahead of us and does not follow what we have: commits for
		// this repo went missing. The ops from this one are indexed either way
		// -- fresh data now beats correct data later -- but the span between
		// our rev and this one has to be re-read.
		log.Log(ctx, "firehose gap detected", "did", evt.Repo,
			"ourRev", row.Version, "evtSince", since, "evtRev", evt.Rev)
		marked, err := atsync.Model.MarkRepoForRepair(ctx, evt.Repo, row.Version)
		if err != nil {
			log.Error(ctx, "failed to mark repo for repair", "did", evt.Repo, "err", err)
			return
		}
		if !marked {
			// Somebody moved the row between the read and the mark. Whatever
			// they wrote, this commit still has to place itself against it.
			continue
		}
		return
	}
}

// repairFloor is how far back a sync reads the windowed collections.
//
// First contact reads [InitialWindow] and nothing more: the account is servable
// in seconds and the deepening ladder fills in its history afterwards.
//
// A repair is different. lastRev is where our index was known good, so that is
// where the missed span starts, and everything written during the gap has an
// rkey from inside it. Reading from just before that rev covers the whole gap
// for a fraction of what re-reading the ladder would cost -- and the result is
// still never shallower than a first sync, so a repair of a repo we saw a
// minute ago still refreshes the last day.
//
// Known limitation, deliberately not solved here: this finds records created
// during the gap, not records DELETED during it, and not a record backdated
// into a window this walk does not cover. Both need a diff of what we hold
// against what the repo holds, which is future work.
func repairFloor(lastRev string, now time.Time) string {
	standard := now.Add(-InitialWindow)
	if lastRev == "" {
		return reposync.TIDForTime(standard)
	}
	revTime, err := reposync.TimeForTID(lastRev)
	if err != nil {
		// Not a TID we can place in time -- an old hand-written row, or a host
		// with its own idea of revs. The standard window is the safe answer.
		return reposync.TIDForTime(standard)
	}
	if from := revTime.Add(-repairSlack); from.Before(standard) {
		return reposync.TIDForTime(from)
	}
	return reposync.TIDForTime(standard)
}

// mergeBackfillState folds what a sync just learned into what the row already
// knew about this repo's history.
//
// It exists because a repair is a shallow sync of a repo that may have years of
// history indexed: the walk it just did says "the last day is indexed", which
// is true and is not the whole truth. Walking a recent window cannot un-complete
// history, and cannot raise a watermark that reaches further back than it does.
func mergeBackfillState(old *model.Repo, res backfillResult) (floor string, done bool) {
	floor, done = res.Floor, res.Done
	if old != nil && old.BackfillDone {
		done = true
	}
	if done {
		// Nothing left to fetch, so there is no watermark to keep: an empty
		// floor is what a completed history reads as everywhere else.
		return "", true
	}
	if old != nil && old.BackfillFloor != "" && (floor == "" || old.BackfillFloor < floor) {
		floor = old.BackfillFloor
	}
	return floor, false
}
