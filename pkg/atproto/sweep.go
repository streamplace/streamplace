package atproto

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/reposync"
)

const (
	// sweepStatusInterval is how often a running sweep says where it is. There
	// is exactly one such line per interval, and none at all when no sweep is
	// running.
	sweepStatusInterval = 10 * time.Second

	// sweepPhaseShallow syncs repos that have never been indexed: everything
	// this node cares about, plus the last [InitialWindow] of the windowed
	// collections. It is what makes an account servable.
	sweepPhaseShallow = "shallow"
	// sweepPhaseDeepen walks history backwards, one window at a time, for every
	// repo that is not complete yet.
	sweepPhaseDeepen = "deepen"
)

// maxDeepenRounds stops the deepening loop from spinning if a repo somehow
// never reports itself finished. Every successful round moves a repo one rung
// down [backfillSpans], so the ladder is walked in len+1 rounds; the slack is
// pure belt and braces.
var maxDeepenRounds = len(backfillSpans) + 3

// sweepItem is one repo for a sweep to work on, tagged with the lane it belongs
// to.
type sweepItem struct {
	DID string
	// Lane is what work is grouped by: the repo's PDS host. Everything a sweep
	// spends its time on is a remote server, so the host is the only shape of
	// the work that matters.
	Lane string
}

// sweepLane is the lane a repo row belongs in: its PDS host, or -- for a repo
// whose host is not known even after [ATProtoSynchronizer.resolveLanes] tried to
// find out -- a lane of its own.
//
// A lane to itself, rather than a shared catch-all: an unplaceable repo is
// normally a resolution failure, so its sync is about to fail too, and queueing
// those behind one another would make an outage at one identity service look
// like a stalled sweep. Two of them on one host is no worse than the flat worker
// pool this replaced -- the per-host pdsLock still keeps their fetches from
// interleaving -- and once either finishes, its row names a PDS, so the next
// sweep lanes it properly.
func sweepLane(did, pds string) string {
	if host := reposync.HostKey(pds); host != "" {
		return host
	}
	return "did:" + did
}

// identityResolveConcurrency is how many identities [ATProtoSynchronizer.resolveLanes]
// looks up at once.
//
// Deliberately smaller than the sweep's own concurrency: these are lookups
// against a handful of shared identity services (plc.directory, DNS) rather than
// against thousands of PDSes, and the whole point is to move work that the
// backfill would have done anyway, not to arrive at plc.directory with a
// thundering herd.
const identityResolveConcurrency = 8

// resolveLanes gives a lane to every item that has not got one, by resolving the
// repo's identity to find its PDS.
//
// This exists because the sweep's DID list and the PDS column come from
// different databases. The DIDs are the state database's set of "repos this node
// indexes"; the host is a column in the index. A node with both has a host for
// every repo (both the placeholder written when a backfill starts and the row
// that replaces it record the PDS). A node with a fresh index and an inherited
// state database -- which is exactly what `streamplace sync` is for, warming a
// new index revision before it takes traffic -- has no rows at all, and would
// put every repo in a lane of its own: sharding by host would do nothing on
// precisely the sweep it was built for.
//
// The lookup is moved rather than added. It goes through the same cached
// directory [ATProtoSynchronizer.SyncBlueskyRepo] resolves with, so the backfill
// a few seconds later reads this answer out of the cache instead of asking
// again.
//
// Failures are not fatal and are not even logged loudly: the repo keeps a lane
// of its own and its sync fails on its own terms, one repo at a time, the way it
// did before.
func (atsync *ATProtoSynchronizer) resolveLanes(ctx context.Context, items []sweepItem) {
	var todo []int
	for i := range items {
		if items[i].Lane == "" {
			todo = append(todo, i)
		}
	}
	if len(todo) == 0 {
		return
	}
	log.Log(ctx, "resolving PDS hosts to shard the sweep", "repos", len(todo))

	start := time.Now()
	var resolved atomic.Int64
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(identityResolveConcurrency)
	for _, i := range todo {
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			ident, err := atsync.resolveIdent(gctx, items[i].DID, true)
			if err != nil {
				log.Debug(gctx, "could not resolve a repo's PDS for sharding", "did", items[i].DID, "err", err)
				return nil
			}
			items[i].Lane = reposync.HostKey(ident.PDSEndpoint())
			resolved.Add(1)
			return nil
		})
	}
	// A cancelled context is the only error this can produce, and the caller is
	// about to notice it for itself.
	_ = g.Wait()

	for _, i := range todo {
		if items[i].Lane == "" {
			items[i].Lane = sweepLane(items[i].DID, "")
		}
	}
	log.Log(ctx, "resolved PDS hosts to shard the sweep", "repos", len(todo),
		"resolved", resolved.Load(), "took", time.Since(start))
}

// hostLanes groups items into one lane per [sweepItem.Lane], keeping each lane's
// items in input order and the lanes in order of first appearance.
//
// Both orders matter. Input order is priority order (own DIDs first, see
// [prioritizeDIDs]), so the lane holding this node's own repos is the first lane
// [runLanes] starts.
func hostLanes(items []sweepItem) [][]sweepItem {
	lanes := make([][]sweepItem, 0, len(items))
	index := make(map[string]int, len(items))
	for _, item := range items {
		i, ok := index[item.Lane]
		if !ok {
			index[item.Lane] = len(lanes)
			lanes = append(lanes, []sweepItem{item})
			continue
		}
		lanes[i] = append(lanes[i], item)
	}
	return lanes
}

// runLanes works every lane, each in its own goroutine and each one item at a
// time, with at most limit lanes in flight. Lanes are started in order, so when
// there are more lanes than slots the earliest lanes go first.
//
// One worker per host is the whole point. A PDS gives a client something like
// ten requests a second and a single range walk already uses five to seven, so
// pointing several walks at one host wins nothing: they interleave their chunk
// fetches through the per-host pdsLock and every one of them crawls. Measured on
// a production sweep, repos on a contended host walked at 12-30 records/s
// against 120-144 uncontended. Lanes make that contention structurally
// impossible within a sweep, while the limit keeps the total request rate across
// the network bounded.
//
// work never fails the sweep -- a repo that errors is logged by the worker and
// its lane moves on to the next repo. Only a cancelled context stops the run,
// and it stops it between items.
func runLanes(ctx context.Context, limit int, lanes [][]sweepItem, work func(context.Context, sweepItem)) error {
	if limit <= 0 {
		limit = config.DefaultSweepConcurrency
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	for _, lane := range lanes {
		g.Go(func() error {
			for _, item := range lane {
				if err := gctx.Err(); err != nil {
					return err
				}
				work(gctx, item)
			}
			return nil
		})
	}
	return g.Wait()
}

// sweepConcurrency is how many host lanes this node runs at once.
func (atsync *ATProtoSynchronizer) sweepConcurrency() int {
	if atsync.CLI != nil && atsync.CLI.SweepConcurrency > 0 {
		return atsync.CLI.SweepConcurrency
	}
	return config.DefaultSweepConcurrency
}

// sweepDIDs is the DIDs of items, for the row lookups that work in DIDs.
func sweepDIDs(items []sweepItem) []string {
	dids := make([]string, len(items))
	for i, item := range items {
		dids[i] = item.DID
	}
	return dids
}

// Sweep brings every repo this node knows about up to date, in two phases:
// first a shallow sync of anything never indexed, then history deepening for
// everything that is not complete.
//
// It is breadth-first on purpose. The shallow phase makes accounts servable as
// fast as it can, and the deepening phase gives every repo one window before it
// gives any repo two, so a node coming up with ten thousand accounts reaches a
// week of history everywhere rather than five years of history for the first
// hundred DIDs in the table.
//
// Nothing on the node waits for this. Repos that fail are logged and left for
// the next sweep -- their rows keep whatever they had -- except that a sweep
// where every shallow sync failed returns an error, because that is a broken
// node rather than a few broken accounts.
func (atsync *ATProtoSynchronizer) Sweep(ctx context.Context) error {
	dids, err := atsync.sweepCandidates(ctx)
	if err != nil {
		return err
	}
	log.Log(ctx, "starting backfill sweep", "totalRepos", len(dids), "concurrency", atsync.sweepConcurrency())

	progress := &sweepProgress{}
	stop := progress.start(ctx)
	defer stop()

	if err := atsync.sweepShallow(ctx, progress, dids); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := atsync.sweepDeepen(ctx, progress, dids); err != nil {
		return err
	}
	log.Log(ctx, "backfill sweep complete", "totalRepos", len(dids))
	return nil
}

// sweepCandidates is every repo worth syncing, own DIDs first.
func (atsync *ATProtoSynchronizer) sweepCandidates(ctx context.Context) ([]string, error) {
	// Accounts that are deactivated, deleted, or taken down fail their backfill
	// the same way on every boot forever. One query up front keeps them out of
	// the sweep entirely, instead of one logged failure each.
	terminalDIDs, err := atsync.Model.TerminalRepoDIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list repos in terminal states: %w", err)
	}
	terminal := make(map[string]struct{}, len(terminalDIDs))
	for _, did := range terminalDIDs {
		terminal[did] = struct{}{}
	}

	var allDIDs []string
	skipped := 0
	offset := 0
	for {
		repos, err := atsync.StatefulDB.ListRepos(100, offset)
		if err != nil {
			return nil, err
		}
		if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			if _, ok := terminal[repo.DID]; ok {
				skipped++
				continue
			}
			allDIDs = append(allDIDs, repo.DID)
		}
		offset += len(repos)
	}

	if skipped > 0 {
		log.Log(ctx, "skipping repos with terminal status", "skipped", skipped)
	}
	return prioritizeDIDs(allDIDs, atsync.CLI.ServerDID(), atsync.CLI.BroadcasterDID()), nil
}

// prioritizeDIDs moves the given DIDs to the front of the list, in the order
// given, keeping everything else where it was.
//
// This node's own repos go first: they hold the streams, videos and settings
// the node itself serves, so a boot that is going to spend an hour on the
// network should spend its first second on them.
func prioritizeDIDs(dids []string, first ...string) []string {
	if len(dids) == 0 || len(first) == 0 {
		return dids
	}
	present := make(map[string]struct{}, len(dids))
	for _, did := range dids {
		present[did] = struct{}{}
	}
	head := make([]string, 0, len(first))
	inHead := make(map[string]struct{}, len(first))
	for _, did := range first {
		if did == "" {
			continue
		}
		if _, ok := present[did]; !ok {
			continue
		}
		if _, ok := inHead[did]; ok {
			continue
		}
		head = append(head, did)
		inHead[did] = struct{}{}
	}
	if len(head) == 0 {
		return dids
	}
	out := make([]string, 0, len(dids))
	out = append(out, head...)
	for _, did := range dids {
		if _, ok := inHead[did]; ok {
			continue
		}
		out = append(out, did)
	}
	return out
}

// sweepShallow syncs every repo that has never completed one. A repo row with
// an empty Version is exactly that: either brand new, or left half-indexed by a
// run that died, which is the same thing as far as anyone reading the index is
// concerned.
func (atsync *ATProtoSynchronizer) sweepShallow(ctx context.Context, progress *sweepProgress, dids []string) error {
	var todo []sweepItem
	for _, did := range dids {
		repo, err := atsync.Model.GetRepo(did)
		if err != nil {
			return fmt.Errorf("failed to get repo for %s: %w", did, err)
		}
		if repo != nil && repo.Version != "" {
			continue
		}
		pds := ""
		if repo != nil {
			pds = repo.PDS
		}
		// Left empty when the row does not name a host: resolveLanes below fills
		// those in rather than letting them each become a lane.
		todo = append(todo, sweepItem{DID: did, Lane: reposync.HostKey(pds)})
	}
	progress.begin(sweepPhaseShallow, len(todo), time.Now().Add(-InitialWindow))
	if len(todo) == 0 {
		return nil
	}
	atsync.resolveLanes(ctx, todo)
	if err := ctx.Err(); err != nil {
		return err
	}
	lanes := hostLanes(todo)
	log.Log(ctx, "syncing repos", "phase", sweepPhaseShallow, "repos", len(todo), "hosts", len(lanes))

	var failed atomic.Int64
	err := runLanes(ctx, atsync.sweepConcurrency(), lanes, func(ctx context.Context, item sweepItem) {
		if _, err := atsync.SyncBlueskyRepoCached(ctx, item.DID); err != nil {
			log.Error(ctx, "failed to sync repo", "did", item.DID, "err", err)
			failed.Add(1)
			return
		}
		progress.finished()
	})
	if err != nil {
		return err
	}
	if int(failed.Load()) == len(todo) {
		return fmt.Errorf("all %d repos failed to sync", len(todo))
	}
	return nil
}

// sweepDeepen fills in history for every repo that has some but not all of it,
// one window per repo per round. Round-robin rather than draining each repo is
// the point: it is what puts the same horizon behind every account.
//
// Each round is a barrier: every repo gets its window, then the next round
// starts. Within a round the work is sharded by host the same way the shallow
// phase shards it, so the breadth-first guarantee ("everyone reaches 7d before
// anyone starts 30d") survives lanes untouched -- a fast host simply waits at
// the end of the round instead of racing ahead through the ladder.
//
// A repo that fails a round drops out of this sweep and keeps its watermark, so
// the next sweep picks it up exactly where it stopped.
func (atsync *ATProtoSynchronizer) sweepDeepen(ctx context.Context, progress *sweepProgress, dids []string) error {
	rank := make(map[string]int, len(dids))
	for i, did := range dids {
		rank[did] = i
	}

	pending, horizon, err := atsync.deepenPending(ctx, dids)
	if err != nil {
		return err
	}
	progress.begin(sweepPhaseDeepen, len(pending), horizon)
	if len(pending) == 0 {
		return nil
	}
	log.Log(ctx, "deepening repo history", "phase", sweepPhaseDeepen, "repos", len(pending),
		"hosts", len(hostLanes(pending)))

	for round := 0; len(pending) > 0 && round < maxDeepenRounds; round++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var mu sync.Mutex
		var next []sweepItem
		err := runLanes(ctx, atsync.sweepConcurrency(), hostLanes(pending), func(ctx context.Context, item sweepItem) {
			done, err := atsync.DeepenRepo(ctx, item.DID)
			if err != nil {
				log.Error(ctx, "failed to deepen repo history", "did", item.DID, "err", err)
				return
			}
			if done {
				progress.finished()
				return
			}
			mu.Lock()
			next = append(next, item)
			mu.Unlock()
		})
		if err != nil {
			return err
		}
		// Restore the priority order the round scrambled.
		sort.Slice(next, func(i, j int) bool { return rank[next[i].DID] < rank[next[j].DID] })
		pending = next
		if _, horizon, err := atsync.deepenPending(ctx, sweepDIDs(pending)); err == nil {
			progress.setHorizon(horizon)
		}
	}
	return nil
}

// deepenPending is the subset of dids whose history is incomplete, laned by
// host, plus the sweep's horizon: the most recent floor among them, which is the
// instant after which every one of these repos is fully indexed.
func (atsync *ATProtoSynchronizer) deepenPending(ctx context.Context, dids []string) ([]sweepItem, time.Time, error) {
	var pending []sweepItem
	var horizon time.Time
	for _, did := range dids {
		repo, err := atsync.Model.GetRepo(did)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("failed to get repo for %s: %w", did, err)
		}
		// No row, no completed sync, parked, or already complete: nothing to
		// deepen. A repo the shallow phase failed on has no Version and is left
		// alone here rather than fetched with the wrong ranges.
		if repo == nil || repo.Version == "" || repo.TerminalStatus() || repo.BackfillDone {
			continue
		}
		pending = append(pending, sweepItem{DID: did, Lane: sweepLane(did, repo.PDS)})
		floor := time.Now()
		if repo.BackfillFloor != "" {
			if t, err := reposync.TimeForTID(repo.BackfillFloor); err == nil {
				floor = t
			}
		}
		if floor.After(horizon) {
			horizon = floor
		}
	}
	return pending, horizon, nil
}

// sweepProgress is the state behind the sweep's status line. It is written by
// every worker and read by the ticker, so everything goes through the mutex.
type sweepProgress struct {
	mu      sync.Mutex
	phase   string
	done    int
	total   int
	horizon time.Time
	started bool
}

// begin starts a phase, resetting the completion count.
func (p *sweepProgress) begin(phase string, total int, horizon time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.phase = phase
	p.total = total
	p.done = 0
	p.horizon = horizon
	p.started = true
}

// finished records one repo completing the current phase.
func (p *sweepProgress) finished() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done++
}

// setHorizon updates how far back the sweep has taken every repo it is working
// on.
func (p *sweepProgress) setHorizon(horizon time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.horizon = horizon
}

// status is the status line's key/value pairs. horizon is unix seconds: the
// instant after which every repo in this phase is fully indexed, so a number
// that climbs backwards through history as the sweep works.
func (p *sweepProgress) status() []any {
	p.mu.Lock()
	defer p.mu.Unlock()
	horizon := int64(0)
	if !p.horizon.IsZero() {
		horizon = p.horizon.Unix()
	}
	return []any{"phase", p.phase, "users", p.done, "total", p.total, "horizon", horizon}
}

// start runs the status ticker until the returned function is called, which
// also waits for it to stop. Nothing is logged before the first tick, so a
// sweep with nothing to do is silent.
func (p *sweepProgress) start(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(sweepStatusInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.mu.Lock()
				started := p.started
				p.mu.Unlock()
				if !started {
					continue
				}
				log.Log(ctx, "backfill sweep", p.status()...)
			}
		}
	}()
	return func() {
		cancel()
		<-stopped
	}
}
