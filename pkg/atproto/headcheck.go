package atproto

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/reposync"
)

// headRev asks a repo's host which revision it is on.
//
// One request, nothing verified: see [reposync.LatestCommit] for why that is
// the right trade for a drift check. It goes through the same per-host lock and
// the same backoff memory as every other sync request, so a pass over thousands
// of repos is as polite to a host as a backfill is.
func (atsync *ATProtoSynchronizer) headRev(ctx context.Context, did string) (string, error) {
	ident, err := atsync.resolveIdent(ctx, did, true)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s: %w", did, err)
	}
	host := ident.PDSEndpoint()
	if host == "" {
		return "", fmt.Errorf("no PDS endpoint found for %s", did)
	}
	xrpcc := &xrpc.Client{Host: host, Client: SyncHTTPClient}

	lock := pdsLocks.GetLock(host)
	lock.Lock()
	defer lock.Unlock()
	latest, err := reposync.LatestCommit(ctx, xrpcc, did, reposync.RetryPolicy{Hints: pdsBackoffHints})
	if err != nil {
		return "", err
	}
	return latest.Rev, nil
}

// sweepCheck is the step that closes the reconciliation loop: it asks one
// repo's host whether the rev we hold is still its rev.
//
// Without it, a repo that finished its backfill is never looked at again, and a
// span of commits missed while this node was down -- or written before a fresh
// index started listening -- is indistinguishable from an account that has been
// quiet. With it, silence is checked once per sweep for the price of one
// request, and drift is turned into the ordinary repair the rest of the engine
// already knows how to do.
//
// It reports whether the repo should go on to its lane's ladder, which for a
// repo that is current means "if it still owes history". A repo that has
// drifted goes back to the lane's shallow queue instead, via enqueue: the
// repair has to happen before deepening means anything.
func (atsync *ATProtoSynchronizer) sweepCheck(ctx context.Context, progress *sweepProgress, enqueue func(sweepItem), step sweepStep) bool {
	defer progress.checked()

	repo, err := atsync.Model.GetRepo(step.DID)
	if err != nil {
		log.Error(ctx, "failed to get repo", "did", step.DID, "err", err)
		return false
	}
	if repo == nil || repo.Version == "" || repo.TerminalStatus() {
		// The row moved since the plan was made -- the firehose marked it for
		// repair, or it got parked. Either way the row is now right and this
		// check would only ask a question somebody already answered.
		return false
	}

	rev, err := atsync.headRev(ctx, step.DID)
	if err != nil {
		if parked := parkTerminalRepo(ctx, atsync.Model, step.DID, err); parked == nil {
			log.Warn(ctx, "failed to check repo head", "did", step.DID, "err", err)
		}
		return false
	}
	if rev == repo.Version {
		return !repo.BackfillDone
	}

	log.Log(ctx, "repo has drifted from its host", "did", step.DID,
		"ourRev", repo.Version, "hostRev", rev)
	marked, err := atsync.Model.MarkRepoForRepair(ctx, step.DID, repo.Version)
	if err != nil {
		log.Error(ctx, "failed to mark repo for repair", "did", step.DID, "err", err)
		return !repo.BackfillDone
	}
	if !marked {
		// Somebody else wedged it first; it is already on its way to a repair.
		return false
	}
	progress.repairing()
	enqueue(sweepItem{DID: step.DID, Lane: step.Lane})
	return false
}
