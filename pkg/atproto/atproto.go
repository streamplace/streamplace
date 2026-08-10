package atproto

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

var SyncGetRepo = comatproto.SyncGetRepo

var handleCache = cache.New(1*time.Hour, 10*time.Minute)

func (atsync *ATProtoSynchronizer) SyncBlueskyRepoCached(ctx context.Context, handle string) (*model.Repo, error) {
	ctx, span := otel.Tracer("signer").Start(ctx, "SyncBlueskyRepoCached")
	defer span.End()
	repo, err := atsync.Model.GetRepoByHandleOrDID(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo for %s: %w", handle, err)
	}
	// A terminal account status -- deactivated, deleted, taken down -- means
	// syncing is pointless until the account comes back, which the firehose
	// tells us about by clearing the status. Hand back what we have rather than
	// asking a PDS a question we know the answer to.
	if repo != nil && repo.TerminalStatus() {
		log.Debug(ctx, "skipping sync of repo in terminal state", "did", repo.DID, "status", repo.Status)
		return repo, nil
	}
	// An empty Version means the row is a placeholder written at the start of a
	// backfill that never finished, so the repo is only partially indexed. Fall
	// through and sync it again -- unless the backfill that wrote it is still
	// running in this process, in which case the placeholder is exactly what
	// the caller should see (records being indexed right now call back in here).
	if repo != nil && (repo.Version != "" || syncInFlight(repo.DID)) {
		return repo, nil
	}

	return atsync.SyncBlueskyRepo(ctx, handle, atsync.Model)
}

func (atsync *ATProtoSynchronizer) SyncBlueskyRepo(ctx context.Context, handle string, mod model.Model) (*model.Repo, error) {
	ident, err := atsync.resolveIdent(ctx, handle, true)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Bluesky handle %s: %w", handle, err)
	}

	ctx = log.WithLogValues(ctx, "did", ident.DID.String(), "handle", ident.Handle.String())

	// The nil-row flavour of SyncBlueskyRepoCached's re-entrancy guard. That
	// guard can only consult the in-flight mark when it has a row to hand
	// back, and a row can vanish mid-walk — an operator deleting it to force
	// a resync is a long-standing habit. With no row, the walk's own record
	// visitor falls through to here and would lock the mutex its caller
	// already holds. The ctx marker identifies exactly that call tree and
	// nothing else: an unrelated caller racing this DID's first sync still
	// waits on the lock like it always has, instead of getting an error for
	// a microsecond coincidence. The record the refused call was serving is
	// re-indexed by whatever sync runs next.
	if walkingDID(ctx) == ident.DID.String() {
		return nil, fmt.Errorf("refusing re-entrant sync of %s inside its own walk", ident.DID.String())
	}

	handleLock := handleLocks.GetLock(ident.DID.String())
	handleLock.Lock()
	defer handleLock.Unlock()
	ctx = markWalking(ctx, ident.DID.String())

	// Tell re-entrant callers (handleCreateUpdate syncs the repos it sees
	// records from) that this DID's placeholder row is being filled in right
	// now, so they take the placeholder instead of recursing into the per-DID
	// lock we are holding.
	defer markSyncInFlight(ident.DID.String())()

	oldRepo, err := mod.GetRepo(ident.DID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get DID record for %s: %w", ident.DID.String(), err)
	}
	if oldRepo != nil && oldRepo.Version != "" {
		log.Debug(ctx, "found existing DID record", "did", oldRepo.DID, "handle", oldRepo.Handle, "version", oldRepo.Version)
		return oldRepo, nil
	}
	if oldRepo != nil {
		// A placeholder from a backfill that never finished: the repo is
		// half-indexed, so sync it again rather than leaving it that way
		// forever. The placeholder row is already there, don't rewrite it.
		log.Log(ctx, "found incomplete DID record, re-syncing", "did", oldRepo.DID, "handle", oldRepo.Handle)
	} else {
		// create an empty repo while we sync. this is useful because we'll start monitoring the firehose for
		// any new follows and such from this user while we're syncing, which can take a long time
		newRepo := model.Repo{
			DID:     ident.DID.String(),
			PDS:     ident.PDSEndpoint(),
			Version: "",
			Handle:  ident.Handle.String(),
		}
		err = mod.UpdateRepo(&newRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create empty DID record for %s: %w", ident.DID.String(), err)
		}
		err = atsync.StatefulDB.AddRepo(ident.DID.String())
		if err != nil {
			return nil, fmt.Errorf("failed to add repo to stateful DB for %s: %w", ident.DID.String(), err)
		}
		log.Log(ctx, "discovered new user", "did", ident.DID.String(), "handle", ident.Handle.String(), "pds", ident.PDSEndpoint())
	}

	// Debug: this fires on every sync operation, not just first contact --
	// "discovered new user" above is the first-contact line.
	log.Debug(ctx, "resolved atproto identity", "did", ident.DID, "handle", ident.Handle, "pds", ident.PDSEndpoint())
	xrpcc := xrpc.Client{
		Host:   ident.PDSEndpoint(),
		Client: SyncHTTPClient,
	}
	if xrpcc.Host == "" {
		return nil, fmt.Errorf("no PDS endpoint found for Bluesky identity %s", handle)
	}

	// First contact is shallow: everything this node indexes, but only the last
	// [InitialWindow] of the collections that can hold years of records. The
	// account is servable in seconds; the sweep deepens its history afterwards.
	// A repo marked for repair instead reads from where its index was last
	// known good, which is where the span it missed begins -- see [repairFloor].
	repairFrom := ""
	if oldRepo != nil {
		repairFrom = oldRepo.RepairFrom
	}
	floor := repairFloor(repairFrom, time.Now())
	result, err := atsync.backfillRepo(ctx, ident, &xrpcc, floor)
	if err != nil {
		if parked := parkTerminalRepo(ctx, mod, ident.DID.String(), err); parked != nil {
			return nil, parked
		}
		return nil, err
	}

	// A completed backfill proves the account is fine, so Status goes back to
	// empty -- UpdateRepo writes every column, so this happens by construction.
	// The same property is why the history state is merged rather than assigned:
	// a repair walks one recent window and would otherwise report a repo with
	// five years indexed as having a day. RepairFrom is left zero on purpose --
	// the repair it asked for is the one that just finished.
	floor, done := mergeBackfillState(oldRepo, result)
	newRepo := model.Repo{
		DID:           ident.DID.String(),
		PDS:           ident.PDSEndpoint(),
		Version:       result.Rev,
		RootCID:       result.RootCID,
		Handle:        ident.Handle.String(),
		Status:        model.RepoStatusOK,
		BackfillFloor: floor,
		BackfillDone:  done,
	}
	err = mod.UpdateRepo(&newRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to update DID record for %s: %w", ident.DID.String(), err)
	}
	err = atsync.StatefulDB.AddRepo(ident.DID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to add repo to stateful DB for %s: %w", ident.DID.String(), err)
	}

	// The one Info line a sync gets. The walks it took are Debug; this is the
	// state change that matters: the repo is servable from here.
	log.Log(ctx, "shallow sync complete", "rev", result.Rev, "deepSyncDone", done)

	return &newRepo, nil
}

// DeepenRepo walks one more window of history for a repo whose recent records
// are already indexed. It reports whether that repo is now complete, and the
// watermark it left behind: the TID from which the repo's windowed collections
// are now fully indexed, which is what the sweep's horizon is made of.
//
// Each call reaches one rung further back down [backfillSpans] and advances the
// row's watermark; the sweep calls it repeatedly, round-robin across the repos
// on one host, so that every account there reaches a week of history before any
// of them reaches a month. The records it re-emits from the window boundary are
// absorbed by the idempotent indexer.
//
// It never writes a placeholder row and never blanks Version, so a repo stays
// served -- and stays out of the wedge path -- for the entire time its history
// is being filled in.
func (atsync *ATProtoSynchronizer) DeepenRepo(ctx context.Context, did string) (bool, string, error) {
	repo, err := atsync.Model.GetRepo(did)
	if err != nil {
		return false, "", fmt.Errorf("failed to get repo for %s: %w", did, err)
	}
	switch {
	case repo == nil:
		return false, "", fmt.Errorf("no repo row for %s", did)
	case repo.TerminalStatus():
		// The account is gone; whatever we indexed is all there will be.
		return true, repo.BackfillFloor, nil
	case repo.BackfillDone:
		return true, repo.BackfillFloor, nil
	case repo.Version == "":
		// Never synced (or wedged): that is the shallow sync's job, and doing
		// it here would skip the full collections entirely.
		return false, "", fmt.Errorf("repo %s has no completed sync to deepen", did)
	}

	// The same lock a full sync takes, so the two cannot walk one repo at once.
	// Indexing a record re-enters SyncBlueskyRepoCached for this same DID, and
	// the Version check above is not enough to keep that from walking in here:
	// a firehose gap detected mid-walk marks the repo for repair, which blanks
	// Version. The in-flight mark is what holds regardless — without it, the
	// re-entry takes this very lock and deadlocks the lane forever.
	handleLock := handleLocks.GetLock(did)
	handleLock.Lock()
	defer handleLock.Unlock()
	defer markSyncInFlight(did)()
	ctx = markWalking(ctx, did)

	ident, err := atsync.resolveIdent(ctx, did, true)
	if err != nil {
		return false, "", fmt.Errorf("failed to resolve %s: %w", did, err)
	}
	xrpcc := xrpc.Client{Host: ident.PDSEndpoint(), Client: SyncHTTPClient}
	if xrpcc.Host == "" {
		return false, "", fmt.Errorf("no PDS endpoint found for %s", did)
	}

	window := nextBackfillWindow(repo.BackfillFloor, time.Now())
	ctx = log.WithLogValues(ctx, "did", did)
	log.Debug(ctx, "walking a history window", "floor", repo.BackfillFloor, "to", window.Lo, "genesis", window.Genesis)

	rev, root, err := atsync.walkBackfill(ctx, ident, &xrpcc, windowRanges(window.Lo, window.Hi))
	if err != nil && isMethodNotSupported(err) {
		// No windowed walk to be had from this host. The full-CAR fallback reads
		// the entire repo, so one of those finishes the job for good.
		log.Warn(ctx, "host does not support sync.getBlocks, deepening with a full getRepo",
			"pds", xrpcc.Host, "err", err)
		// The legacy path has no verified MST root to record.
		root = ""
		rev, err = atsync.legacyBackfill(ctx, ident, &xrpcc)
		window = backfillWindow{Genesis: true}
	}
	if err != nil {
		if parked := parkTerminalRepo(ctx, atsync.Model, did, err); parked != nil {
			return false, "", parked
		}
		return false, "", err
	}

	applied, err := atsync.Model.AdvanceRepoBackfill(ctx, did, rev, root, window.Lo, window.Genesis)
	if err != nil {
		return false, "", fmt.Errorf("failed to record backfill watermark for %s: %w", did, err)
	}
	if !applied {
		// A repair wedged this repo while its window was being walked. The
		// wedge wins: the window goes unrecorded, the repair re-syncs the repo,
		// and the sweep ladders it again from the floor it already had.
		log.Log(ctx, "repo was marked for repair mid-deepen; leaving it wedged")
		return false, repo.BackfillFloor, nil
	}
	// Debug: at one line per repo per window this is thousands of lines per
	// sweep. The sweep logs one Info summary per repo when its ladder finishes.
	log.Debug(ctx, "deepened repo history", "rev", rev, "floor", window.Lo, "done", window.Genesis)
	return window.Genesis, window.Lo, nil
}

// walkingDIDKey carries, on a ctx, the DID whose repo walk this call tree is
// performing. It is how a re-entrant sync attempt for that DID — a record
// visitor calling back into SyncBlueskyRepo after the row vanished out from
// under its walk — can be refused instead of deadlocking on the per-DID lock
// its own caller holds. A ctx value rather than a registry on purpose: it
// names one call tree, so unrelated concurrent callers are never mistaken for
// re-entry.
type walkingDIDKey struct{}

func markWalking(ctx context.Context, did string) context.Context {
	return context.WithValue(ctx, walkingDIDKey{}, did)
}

func walkingDID(ctx context.Context) string {
	s, _ := ctx.Value(walkingDIDKey{}).(string)
	return s
}

// syncsInFlight holds the DIDs whose backfill is running in this process right
// now. A placeholder repo row (empty Version) otherwise means "incomplete,
// re-sync me", which would be wrong -- and, since indexing a record can call
// back into SyncBlueskyRepoCached for the same DID, deadlock on the per-DID
// lock -- while the backfill that wrote it is still going.
var syncsInFlight = struct {
	sync.Mutex
	dids map[string]int
}{dids: map[string]int{}}

func markSyncInFlight(did string) func() {
	syncsInFlight.Lock()
	syncsInFlight.dids[did]++
	syncsInFlight.Unlock()
	return func() {
		syncsInFlight.Lock()
		defer syncsInFlight.Unlock()
		if syncsInFlight.dids[did] <= 1 {
			delete(syncsInFlight.dids, did)
			return
		}
		syncsInFlight.dids[did]--
	}
}

func syncInFlight(did string) bool {
	syncsInFlight.Lock()
	defer syncsInFlight.Unlock()
	return syncsInFlight.dids[did] > 0
}

func (atsync *ATProtoSynchronizer) RefreshIdentity(ctx context.Context, did string) (*identity.Identity, error) {
	id, err := atsync.resolveIdent(ctx, did, false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ident: %w", err)
	}
	newRepo := model.Repo{
		DID:    id.DID.String(),
		PDS:    id.PDSEndpoint(),
		Handle: id.Handle.String(),
	}
	// UpdateRepo writes every column, so carry the sync state over: blanking
	// Version here would mark the repo as never-backfilled and (now that an
	// empty Version means "re-sync me") make every identity event trigger a
	// pointless full re-index.
	oldRepo, err := atsync.Model.GetRepo(id.DID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}
	if oldRepo != nil {
		newRepo.Version = oldRepo.Version
		newRepo.RootCID = oldRepo.RootCID
		// Same reasoning for Status: an identity event is not evidence the
		// account came back, and blanking it here would put every deactivated
		// repo back in the boot-time sync sweep.
		newRepo.Status = oldRepo.Status
		// And for the backfill watermark: losing it would make the sweep walk
		// this repo's whole history again from the top of the ladder.
		newRepo.BackfillFloor = oldRepo.BackfillFloor
		newRepo.BackfillDone = oldRepo.BackfillDone
		// And for the repair watermark, which is how a pending repair knows
		// which span of history it is there to re-read.
		newRepo.RepairFrom = oldRepo.RepairFrom
	}
	err = atsync.Model.UpdateRepo(&newRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to update repo: %w", err)
	}
	return id, nil
}

func (atsync *ATProtoSynchronizer) ResolveAuthorHandle(ctx context.Context, did string) string {
	if cached, ok := handleCache.Get(did); ok {
		return cached.(string)
	}
	ident, err := atsync.resolveIdent(ctx, did, true)
	if err != nil {
		log.Warn(ctx, "failed to resolve author handle", "did", did, "err", err)
		return ""
	}
	handle := ident.Handle.String()
	if handle != "" {
		handleCache.SetDefault(did, handle)
	}
	return handle
}

// directory hands back the identity directory to resolve with, building the
// pair on first use.
//
// Under a lock because a sweep resolves identities from dozens of goroutines at
// once -- lane workers and the sharding resolver, at the same instant -- and
// two of them racing to install the lazily built directory would each end up
// using a different cache, if the race detector let them get that far.
func (atsync *ATProtoSynchronizer) directory(cached bool) identity.Directory {
	atsync.dirMu.Lock()
	defer atsync.dirMu.Unlock()
	if atsync.PLCDirectory == nil {
		atsync.PLCDirectory = CustomDirectory(atsync.CLI.PLCURL)
	}
	if atsync.CachedPLCDirectory == nil {
		cachedDir := identity.NewCacheDirectory(atsync.PLCDirectory, 250_000, time.Hour*24, time.Minute*2, time.Minute*5)
		atsync.CachedPLCDirectory = &cachedDir
	}
	if cached {
		return atsync.CachedPLCDirectory
	}
	return atsync.PLCDirectory
}

// resetIdentCache throws away every cached identity resolution. For when the
// node has knowingly skipped firehose it can never replay: any number of
// #identity events may sit in that gap, and a cache entry from before it
// agrees with the equally-stale repo row — hiding exactly the drift the next
// sweep's head check is being asked to find. Starting the cache over makes
// that sweep's resolutions authoritative. (A restart gets this for free; the
// cache lives in process memory.)
func (atsync *ATProtoSynchronizer) resetIdentCache() {
	atsync.dirMu.Lock()
	defer atsync.dirMu.Unlock()
	atsync.CachedPLCDirectory = nil
}

// purgeIdentCache drops a DID's cached identity, so the next cached resolve
// re-reads the directory instead of serving an entry we just proved stale.
func (atsync *ATProtoSynchronizer) purgeIdentCache(ctx context.Context, did string) {
	atid, err := syntax.ParseAtIdentifier(did)
	if err != nil {
		return
	}
	if cd, ok := atsync.directory(true).(*identity.CacheDirectory); ok {
		if err := cd.Purge(ctx, *atid); err != nil {
			log.Debug(ctx, "failed to purge cached identity", "did", did, "err", err)
		}
	}
}

func (atsync *ATProtoSynchronizer) resolveIdent(ctx context.Context, arg string, cached bool) (*identity.Identity, error) {
	dir := atsync.directory(cached)
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}

	resolvedID, err := dir.Lookup(ctx, *id)
	if err != nil {
		return nil, err
	}
	log.Debug(ctx, "resolved ident", "id", resolvedID.DID.String(), "handle", resolvedID.Handle.String())

	return resolvedID, nil
}
func CustomDirectory(plcURL string) identity.Directory {
	base := identity.BaseDirectory{
		PLCURL:     plcURL,
		HTTPClient: aqhttp.Client,
		Resolver: net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: time.Second * 3}
				return d.DialContext(ctx, network, address)
			},
		},
		TryAuthoritativeDNS: true,
		// primary Bluesky PDS instance only supports HTTP resolution method
		SkipDNSDomainSuffixes: []string{".bsky.social"},
	}
	return &base
}

func DIDDoc(host string, pubMultibase string) map[string]any {
	return map[string]any{
		"@context": []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
			"https://w3id.org/security/suites/secp256k1-2019/v1",
		},
		"id":          fmt.Sprintf("did:web:%s", host),
		"alsoKnownAs": []string{},
		"service": []map[string]any{
			{
				"id":              "#bsky_fg",
				"type":            "BskyFeedGenerator",
				"serviceEndpoint": fmt.Sprintf("https://%s", host),
			},
			{
				"id":              "#atproto_pds",
				"type":            "AtprotoPersonalDataServer",
				"serviceEndpoint": fmt.Sprintf("https://%s", host),
			},
		},
		"verificationMethod": []map[string]any{
			{
				"id":                 fmt.Sprintf("did:web:%s#atproto", host),
				"type":               "Multikey",
				"controller":         fmt.Sprintf("did:web:%s", host),
				"publicKeyMultibase": pubMultibase,
			},
		},
	}
}
