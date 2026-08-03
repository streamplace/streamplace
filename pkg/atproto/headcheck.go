package atproto

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/reposync"
)

// headRev asks a repo's host which revision it is on. It also returns the
// identity it resolved along the way (whenever resolution succeeded, even if
// the host then failed to answer), because the caller has a second use for it:
// see [ATProtoSynchronizer.refreshDriftedIdentity].
//
// One request, nothing verified: see [reposync.LatestCommit] for why that is
// the right trade for a drift check. It goes through the same per-host lock and
// the same backoff memory as every other sync request, so a pass over thousands
// of repos is as polite to a host as a backfill is.
func (atsync *ATProtoSynchronizer) headRev(ctx context.Context, did string) (string, *identity.Identity, error) {
	ident, err := atsync.resolveIdent(ctx, did, true)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve %s: %w", did, err)
	}
	host := ident.PDSEndpoint()
	if host == "" {
		return "", ident, fmt.Errorf("no PDS endpoint found for %s", did)
	}
	xrpcc := &xrpc.Client{Host: host, Client: SyncHTTPClient}

	lock := pdsLocks.GetLock(host)
	lock.Lock()
	defer lock.Unlock()
	latest, err := reposync.LatestCommit(ctx, xrpcc, did, reposync.RetryPolicy{Hints: pdsBackoffHints})
	if err != nil {
		return "", ident, err
	}
	return latest.Rev, ident, nil
}

// refreshDriftedIdentity keeps the row's identity columns as fresh as the head
// check keeps its rev.
//
// An #identity event is the only firehose signal that a handle or PDS changed,
// and this node is allowed to miss firehose: it may be down past the replay
// window, or deliberately skip a replay too old to be worth it. Missed commits
// are healed by the head check, but a missed identity event used to stay wrong
// until the account happened to emit another one. So: the head check resolves
// every servable repo's identity anyway, to find its host — if what it resolved
// disagrees with the row, something is stale. Confirm against an authoritative
// (uncached) resolve before writing, because the disagreement could equally be
// the identity cache lagging a row a live event already updated. Either way the
// cache entry is purged, so the next cached resolve serves what we just
// learned.
func (atsync *ATProtoSynchronizer) refreshDriftedIdentity(ctx context.Context, repo *model.Repo, ident *identity.Identity) {
	if ident == nil || (ident.Handle.String() == repo.Handle && ident.PDSEndpoint() == repo.PDS) {
		return
	}
	fresh, err := atsync.resolveIdent(ctx, repo.DID, false)
	if err != nil {
		log.Warn(ctx, "failed to confirm drifted identity", "did", repo.DID, "handle", repo.Handle, "err", err)
		return
	}
	handle, pds := fresh.Handle.String(), fresh.PDSEndpoint()
	if handle != repo.Handle || pds != repo.PDS {
		log.Log(ctx, "repo identity has drifted; refreshing", "did", repo.DID,
			"oldHandle", repo.Handle, "handle", handle, "oldPDS", repo.PDS, "pds", pds)
		if err := atsync.Model.UpdateRepoIdentity(repo.DID, handle, pds); err != nil {
			log.Error(ctx, "failed to refresh repo identity", "did", repo.DID, "handle", handle, "err", err)
			return
		}
	}
	atsync.purgeIdentCache(ctx, repo.DID)
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
		log.Error(ctx, "failed to get repo", "did", step.DID, "handle", step.Handle, "err", err)
		return false
	}
	if repo == nil || repo.Version == "" || repo.TerminalStatus() {
		// The row moved since the plan was made -- the firehose marked it for
		// repair, or it got parked. Either way the row is now right and this
		// check would only ask a question somebody already answered.
		return false
	}

	rev, ident, err := atsync.headRev(ctx, step.DID)
	// Identity drift is checked even when the host's answer never came: the
	// resolve is the part that matters here, and a moved repo's old host
	// erroring is one of the ways this situation looks.
	atsync.refreshDriftedIdentity(ctx, repo, ident)
	if err != nil {
		if parked := parkTerminalRepo(ctx, atsync.Model, step.DID, err); parked == nil {
			log.Warn(ctx, "failed to check repo head", "did", step.DID, "handle", repo.Handle, "err", err)
		}
		return false
	}
	if rev == repo.Version {
		return !repo.BackfillDone
	}

	log.Log(ctx, "repo has drifted from its host", "did", step.DID, "handle", repo.Handle,
		"ourRev", repo.Version, "hostRev", rev)
	marked, err := atsync.Model.MarkRepoForRepair(ctx, step.DID, repo.Version)
	if err != nil {
		log.Error(ctx, "failed to mark repo for repair", "did", step.DID, "handle", repo.Handle, "err", err)
		return !repo.BackfillDone
	}
	if !marked {
		// Somebody else wedged it first; it is already on its way to a repair.
		return false
	}
	progress.repairing()
	enqueue(sweepItem{DID: step.DID, Handle: step.Handle, Lane: step.Lane})
	return false
}
