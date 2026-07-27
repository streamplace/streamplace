package atproto

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/reposync"
)

const (
	// sweepConcurrency bounds how many repos a sweep works on at once. A boot
	// used to start one goroutine per known repo, which meant a fresh node
	// opened with a thundering herd at every PDS it had ever heard of.
	sweepConcurrency = 6

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
	log.Log(ctx, "starting backfill sweep", "totalRepos", len(dids))

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
	var todo []string
	for _, did := range dids {
		repo, err := atsync.Model.GetRepo(did)
		if err != nil {
			return fmt.Errorf("failed to get repo for %s: %w", did, err)
		}
		if repo != nil && repo.Version != "" {
			continue
		}
		todo = append(todo, did)
	}
	progress.begin(sweepPhaseShallow, len(todo), time.Now().Add(-InitialWindow))
	if len(todo) == 0 {
		return nil
	}
	log.Log(ctx, "syncing repos", "phase", sweepPhaseShallow, "repos", len(todo))

	var mu sync.Mutex
	failed := 0
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(sweepConcurrency)
	for _, did := range todo {
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			if _, err := atsync.SyncBlueskyRepoCached(gctx, did); err != nil {
				log.Error(gctx, "failed to sync repo", "did", did, "err", err)
				mu.Lock()
				failed++
				mu.Unlock()
				return nil
			}
			progress.finished()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	if failed == len(todo) {
		return fmt.Errorf("all %d repos failed to sync", failed)
	}
	return nil
}

// sweepDeepen fills in history for every repo that has some but not all of it,
// one window per repo per round. Round-robin rather than draining each repo is
// the point: it is what puts the same horizon behind every account.
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
	log.Log(ctx, "deepening repo history", "phase", sweepPhaseDeepen, "repos", len(pending))

	for round := 0; len(pending) > 0 && round < maxDeepenRounds; round++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var mu sync.Mutex
		var next []string
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(sweepConcurrency)
		for _, did := range pending {
			g.Go(func() error {
				if err := gctx.Err(); err != nil {
					return err
				}
				done, err := atsync.DeepenRepo(gctx, did)
				if err != nil {
					log.Error(gctx, "failed to deepen repo history", "did", did, "err", err)
					return nil
				}
				if done {
					progress.finished()
					return nil
				}
				mu.Lock()
				next = append(next, did)
				mu.Unlock()
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
		// Restore the priority order the round scrambled.
		sort.Slice(next, func(i, j int) bool { return rank[next[i]] < rank[next[j]] })
		pending = next
		if _, horizon, err := atsync.deepenPending(ctx, pending); err == nil {
			progress.setHorizon(horizon)
		}
	}
	return nil
}

// deepenPending is the subset of dids whose history is incomplete, plus the
// sweep's horizon: the most recent floor among them, which is the instant after
// which every one of these repos is fully indexed.
func (atsync *ATProtoSynchronizer) deepenPending(ctx context.Context, dids []string) ([]string, time.Time, error) {
	var pending []string
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
		pending = append(pending, did)
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
