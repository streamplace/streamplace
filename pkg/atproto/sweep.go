package atproto

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/reposync"
)

// sweepStatusInterval is how often a running sweep says where it is. There is
// exactly one such line per interval, and none at all when no sweep is running.
const sweepStatusInterval = 10 * time.Second

// maxDeepenRounds stops a repo's ladder from spinning if it somehow never
// reports itself finished. Every successful window moves a repo one rung down
// [backfillSpans], so the ladder is walked in len+1 windows; the slack is pure
// belt and braces. It is a per-repo budget for one sweep, not a global round
// count -- there are no global rounds.
var maxDeepenRounds = len(backfillSpans) + 3

// sweepItem is one repo for a sweep to work on, tagged with the lane it belongs
// to and with which half of its lane's program it starts in.
type sweepItem struct {
	DID string
	// Handle is the repo's handle as the index last knew it, carried purely so
	// the sweep's log lines name an account a human recognizes. It comes off the
	// same row the plan already read, so it costs no query; it is empty for a
	// repo we have no row for yet, and may be stale, which for a log line is
	// fine.
	Handle string
	// Lane is what work is grouped by: the repo's PDS host. Everything a sweep
	// spends its time on is a remote server, so the host is the only shape of
	// the work that matters.
	Lane string
	// Deepen is set for a repo that already has a completed sync and needs
	// nothing but history: it starts in its lane's ladder rather than in its
	// lane's shallow queue.
	Deepen bool
	// Check is set for a repo that is servable and believed current, and so
	// starts with one request that asks its host whether that belief is true.
	Check bool
}

// sweepStep is one unit of work a lane does: either the shallow sync a repo
// needs before anything else can happen to it, or one window of its history.
type sweepStep struct {
	sweepItem
	// Windows is how many history windows this repo has already had this
	// sweep, which for a deepening step is also the ladder rung it came off.
	Windows int
}

// sweepLane is the lane a repo row belongs in: its PDS host, or -- for a repo
// whose host is not known even after [ATProtoSynchronizer.feedUnresolved] tried
// to find out -- a lane of its own.
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

// laneCount is how many distinct lanes -- hosts, in practice -- a set of items
// covers.
func laneCount(items []sweepItem) int {
	lanes := make(map[string]struct{}, len(items))
	for _, item := range items {
		lanes[item.Lane] = struct{}{}
	}
	return len(lanes)
}

// identityResolveConcurrency is how many identities [ATProtoSynchronizer.feedUnresolved]
// looks up at once.
//
// Deliberately smaller than the sweep's own concurrency: these are lookups
// against a handful of shared identity services (plc.directory, DNS) rather than
// against thousands of PDSes, and the whole point is to move work that the
// backfill would have done anyway, not to arrive at plc.directory with a
// thundering herd. On a fresh index every repo needs one of these, so this is
// also the ceiling on how fast a fresh node discovers work -- which is why it
// feeds a running [laneScheduler] instead of gating the sweep behind a barrier.
const identityResolveConcurrency = 16

// feedUnresolved lanes every item that has not got one, by resolving the repo's
// identity to find its PDS, handing each item to add as its answer lands. It
// returns when every item has been handed over.
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
// Streaming, not a barrier: a fresh node has tens of thousands of these lookups
// to do, and doing them all before the first walk turned the start of a sweep
// into minutes of dead air. Feeding a running [laneScheduler] means the first
// repos are being walked while the last are still being resolved.
//
// The lookup is moved rather than added. It goes through the same cached
// directory [ATProtoSynchronizer.SyncBlueskyRepo] resolves with, so the backfill
// moments later reads this answer out of the cache instead of asking again.
//
// Failures are not fatal and are not even logged loudly: the repo gets a lane
// of its own and its sync fails on its own terms, one repo at a time, the way it
// did before.
func (atsync *ATProtoSynchronizer) feedUnresolved(ctx context.Context, items []sweepItem, add func(sweepItem)) {
	if len(items) == 0 {
		return
	}
	log.Log(ctx, "resolving PDS hosts to shard the sweep", "repos", len(items))

	start := time.Now()
	var resolved atomic.Int64
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(identityResolveConcurrency)
	for _, item := range items {
		g.Go(func() error {
			// On cancellation, still hand the item over (with a lane of its
			// own): the scheduler's workers notice the dead context themselves,
			// and every item accounted for exactly once is the simpler
			// invariant to keep.
			if gctx.Err() == nil {
				if ident, err := atsync.resolveIdent(gctx, item.DID, true); err == nil {
					item.Lane = reposync.HostKey(ident.PDSEndpoint())
					resolved.Add(1)
				} else {
					log.Debug(gctx, "could not resolve a repo's PDS for sharding", "did", item.DID, "handle", item.Handle, "err", err)
				}
			}
			if item.Lane == "" {
				item.Lane = sweepLane(item.DID, "")
			}
			add(item)
			return nil
		})
	}
	_ = g.Wait()
	log.Log(ctx, "resolved PDS hosts to shard the sweep", "repos", len(items),
		"resolved", resolved.Load(), "took", time.Since(start))
}

// laneProgram is one host's entire share of a sweep, and the reason a sweep has
// no phases: rather than every lane doing shallow work until the slowest host
// has finished its shallow work, a lane runs this program to completion by
// itself and then gives its slot to the next host.
//
// Head checks come first, because each is one request that turns a repo we
// believe is current into one we know is current -- or into shallow work this
// lane did not know it had. Shallow work is next, because a repo with no
// completed sync cannot be deepened at all, and because a repo that has just
// been discovered is not servable until it has one. Deepening is round-robin
// within the host, which is what the ladder buckets are for.
type laneProgram struct {
	// check is the repos on this host whose head has not been verified against
	// ours this sweep.
	check []sweepItem
	// shallow is the repos on this host with no completed sync, oldest first.
	shallow []sweepItem
	// ladder holds the repos with history left to fetch, bucketed by how many
	// windows they have had this sweep: ladder[n] is the repos on their (n+1)th
	// window. Always taking from the lowest non-empty bucket makes the
	// breadth-first guarantee structural -- no repo on this host gets its
	// (n+1)th window while another still wants its nth -- and it holds for
	// repos that join late, which a plain round-robin queue would leave a full
	// lap behind. The bucket index is also the per-repo spin guard: nothing is
	// ever pushed past [maxDeepenRounds].
	ladder [][]sweepItem
	// live reports that a worker is draining this lane. Guarded by the
	// scheduler's lock, like everything else here.
	live bool
}

// next takes the lane's next step: an unchecked head if there is one, then the
// oldest waiting shallow sync, then the least-deepened repo's next window.
func (p *laneProgram) next() (sweepStep, bool) {
	if len(p.check) > 0 {
		item := p.check[0]
		p.check = p.check[1:]
		return sweepStep{sweepItem: item}, true
	}
	if len(p.shallow) > 0 {
		item := p.shallow[0]
		p.shallow = p.shallow[1:]
		return sweepStep{sweepItem: item}, true
	}
	for n, bucket := range p.ladder {
		if len(bucket) == 0 {
			continue
		}
		item := bucket[0]
		p.ladder[n] = bucket[1:]
		return sweepStep{sweepItem: item, Windows: n}, true
	}
	return sweepStep{}, false
}

// add puts a repo into the part of the program it belongs in.
func (p *laneProgram) add(item sweepItem) {
	switch {
	case item.Check:
		p.check = append(p.check, item)
	case item.Deepen:
		p.push(item, 0)
	default:
		p.shallow = append(p.shallow, item)
	}
}

// push queues a repo for its next window, having had windows of them already.
// A repo that has used its whole budget is dropped: it keeps its watermark, so
// the next sweep carries on where this one stopped.
func (p *laneProgram) push(item sweepItem, windows int) {
	if windows >= maxDeepenRounds {
		return
	}
	item.Deepen = true
	// Whatever this repo was doing, it is deepening now: a checked or freshly
	// synced repo must not be handed back its old step class.
	item.Check = false
	for len(p.ladder) <= windows {
		p.ladder = append(p.ladder, nil)
	}
	p.ladder[windows] = append(p.ladder[windows], item)
}

// laneScheduler runs one [laneProgram] per host, at most [laneScheduler.limit]
// of them at a time, one worker per host ever.
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
// It accepts items while it is running, so repos whose lane is already known are
// walked while a resolver is still finding hosts for the rest, and a repo
// resolved late lands in a lane that is already deep in its ladder -- where its
// shallow sync preempts the rest of that ladder.
//
// work never fails the sweep: a repo that errors is logged by the worker and its
// lane moves on. Only a cancelled context stops the run, and it stops it between
// steps.
type laneScheduler struct {
	ctx context.Context
	// work does one step and reports whether the repo wants another: a shallow
	// step returning true puts the repo at the bottom of the ladder, a
	// deepening step returning true asks for one more window.
	work func(context.Context, sweepStep) bool

	mu    sync.Mutex
	lanes map[string]*laneProgram
	seen  int
	// free and waiting are the slot budget. A worker holds a slot for the life
	// of its lane, and slots are handed out strictly in the order lanes were
	// first added -- own DIDs first, see [prioritizeDIDs]. A queue rather than a
	// buffered channel because a channel would hand slots out in the order
	// worker goroutines happened to get scheduled, which is no order at all.
	free    int
	waiting []chan struct{}
	wg      sync.WaitGroup
}

func newLaneScheduler(ctx context.Context, limit int, work func(context.Context, sweepStep) bool) *laneScheduler {
	if limit <= 0 {
		limit = config.DefaultSweepConcurrency
	}
	return &laneScheduler{
		ctx:   ctx,
		work:  work,
		lanes: map[string]*laneProgram{},
		free:  limit,
	}
}

// add enqueues an item on its lane, starting a worker for the lane if none is
// running. Safe from any goroutine, and never blocks; must not be called after
// [laneScheduler.wait] returns.
func (s *laneScheduler) add(item sweepItem) {
	s.mu.Lock()
	prog, ok := s.lanes[item.Lane]
	if !ok {
		prog = &laneProgram{}
		s.lanes[item.Lane] = prog
		s.seen++
	}
	prog.add(item)
	var slot chan struct{}
	if !prog.live {
		prog.live = true
		slot = s.acquire()
		s.wg.Add(1)
	}
	s.mu.Unlock()
	if slot != nil {
		go s.run(prog, slot)
	}
}

// acquire takes a slot, or a promise of one: the returned channel is closed
// when the caller may run. Called with the lock held, so that slots are queued
// in the order [laneScheduler.add] is called rather than the order goroutines
// start.
func (s *laneScheduler) acquire() chan struct{} {
	slot := make(chan struct{})
	if s.free > 0 {
		s.free--
		close(slot)
		return slot
	}
	s.waiting = append(s.waiting, slot)
	return slot
}

// release hands a finished lane's slot to the longest-waiting lane, or back to
// the budget if nobody is waiting.
func (s *laneScheduler) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.waiting) > 0 {
		slot := s.waiting[0]
		s.waiting = s.waiting[1:]
		close(slot)
		return
	}
	s.free++
}

// giveUp drops a slot the caller was waiting for. If the slot was granted in
// the meantime it is passed on rather than lost.
func (s *laneScheduler) giveUp(slot chan struct{}) {
	s.mu.Lock()
	for i, w := range s.waiting {
		if w == slot {
			s.waiting = append(s.waiting[:i], s.waiting[i+1:]...)
			s.mu.Unlock()
			return
		}
	}
	s.mu.Unlock()
	s.release()
}

// run works one lane's program to the end, one step at a time, holding a slot
// throughout. It marks the lane not-live under the lock in the same instant it
// observes the program empty, so a concurrent add either lands before that (and
// this worker picks it up) or after (and spawns a fresh worker).
func (s *laneScheduler) run(prog *laneProgram, slot chan struct{}) {
	defer s.wg.Done()
	select {
	case <-slot:
	case <-s.ctx.Done():
		s.giveUp(slot)
		s.abandon(prog)
		return
	}
	defer s.release()
	for {
		if s.ctx.Err() != nil {
			s.abandon(prog)
			return
		}
		s.mu.Lock()
		step, ok := prog.next()
		if !ok {
			prog.live = false
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		if !s.work(s.ctx, step) {
			continue
		}
		// A shallow sync that worked drops the repo at the bottom of the
		// ladder; a window that worked asks for the next rung.
		windows := 0
		if step.Deepen {
			windows = step.Windows + 1
		}
		s.mu.Lock()
		prog.push(step.sweepItem, windows)
		s.mu.Unlock()
	}
}

// abandon drops a lane's remaining work on cancellation. The repos keep their
// rows untouched, so the next sweep picks them up.
func (s *laneScheduler) abandon(prog *laneProgram) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prog.live = false
	prog.check = nil
	prog.shallow = nil
	prog.ladder = nil
}

// wait blocks until every lane has run its program out or been abandoned, and
// reports how many distinct lanes the run touched. Callers must have finished
// adding.
func (s *laneScheduler) wait() (lanes int, err error) {
	s.wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen, s.ctx.Err()
}

// SweepForever runs a sweep at boot and another every [config.CLI.SweepInterval]
// after that, for as long as ctx lives.
//
// Repeating is what makes the head check worth having: a repo that is current
// costs one request per interval, and one that has drifted -- because this node
// was down longer than the relay's replay window, or because a fresh index
// started listening after the accounts it inherited had already moved -- is
// found and repaired within an interval instead of never.
func (atsync *ATProtoSynchronizer) SweepForever(ctx context.Context) {
	atsync.sweepLoop(ctx, atsync.sweepInterval(), func(ctx context.Context) {
		atsync.sweepOnce(ctx, atsync.Sweep)
	})
}

// sweepLoop runs run now and every interval after, until ctx ends. A
// non-positive interval runs it exactly once: the boot sweep is not optional,
// only repeating it is.
func (atsync *ATProtoSynchronizer) sweepLoop(ctx context.Context, interval time.Duration, run func(context.Context)) {
	run(ctx)
	if interval <= 0 || ctx.Err() != nil {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run(ctx)
		}
	}
}

// sweepOnce runs a sweep unless one is already running.
//
// A sweep of a large index can take longer than the interval -- a fresh node's
// first one takes hours -- and two of them at once would double every host's
// request rate while doing the same work twice. The tick is dropped rather than
// queued: the next one is another interval away, which is exactly when a sweep
// that just finished should run again.
func (atsync *ATProtoSynchronizer) sweepOnce(ctx context.Context, sweep func(context.Context) error) {
	if !atsync.sweeping.CompareAndSwap(false, true) {
		log.Log(ctx, "skipping scheduled sweep; the previous one is still running")
		return
	}
	defer atsync.sweeping.Store(false)
	if err := sweep(ctx); err != nil && ctx.Err() == nil {
		log.Error(ctx, "backfill sweep failed", "err", err)
	}
}

// sweepInterval is how often this node re-sweeps. Zero (or negative) disables
// the ticker, leaving the boot sweep on its own.
func (atsync *ATProtoSynchronizer) sweepInterval() time.Duration {
	if atsync.CLI == nil {
		return config.DefaultSweepInterval
	}
	return atsync.CLI.SweepInterval
}

// sweepConcurrency is how many host lanes this node runs at once.
func (atsync *ATProtoSynchronizer) sweepConcurrency() int {
	if atsync.CLI != nil && atsync.CLI.SweepConcurrency > 0 {
		return atsync.CLI.SweepConcurrency
	}
	return config.DefaultSweepConcurrency
}

// Sweep brings every repo this node knows about up to date: a shallow sync for
// anything never indexed, then history deepening until it is complete.
//
// Both happen at once, because a sweep is thousands of independent
// conversations with hundreds of servers and the slowest of them must not hold
// up the rest. Each host gets a lane, each lane runs its own program -- see
// [laneProgram] -- and a host that finishes early hands its slot to a host that
// has not started. The sweep is over when the last lane is.
//
// Within a host it is still breadth-first: shallow syncs first, so accounts
// become servable as fast as they can, and then one window per repo before any
// repo gets two, so a node coming up reaches a week of history everywhere on
// that host rather than five years for the first few DIDs. Across hosts there is
// no such guarantee, and buying it was what cost half the wall clock: it made
// every lane wait for the slowest host, once per rung of the ladder.
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

	plan, err := atsync.sweepPlan(dids)
	if err != nil {
		return err
	}
	progress := &sweepProgress{}
	progress.begin(plan.shallow, plan.checks, plan.floors)
	stop := progress.start(ctx)
	defer stop()

	log.Log(ctx, "sweeping repos", "shallow", plan.shallow, "check", plan.checks,
		"deepen", len(plan.floors), "knownHosts", laneCount(plan.ready),
		"unresolved", len(plan.unresolved))

	var attempted, failed atomic.Int64
	// The scheduler is captured by the work it runs: a head check that finds
	// drift has repair work to hand back, and hands it to the lane it is
	// already running on.
	var sched *laneScheduler
	sched = newLaneScheduler(ctx, atsync.sweepConcurrency(), func(ctx context.Context, step sweepStep) bool {
		switch {
		case step.Check:
			return atsync.sweepCheck(ctx, progress, sched.add, step)
		case !step.Deepen:
			return atsync.sweepSync(ctx, progress, &attempted, &failed, step)
		default:
			return atsync.sweepWindow(ctx, progress, step)
		}
	})
	// Lanes whose host is already known start working immediately, in priority
	// order (own DIDs first); the rest stream in as the resolver finds them.
	for _, item := range plan.ready {
		sched.add(item)
	}
	atsync.feedUnresolved(ctx, plan.unresolved, sched.add)
	lanes, err := sched.wait()
	if err != nil {
		return err
	}
	// Counted rather than compared against the plan: head checks add shallow
	// work as they find it, so the number of syncs a sweep tries is not known
	// when it starts.
	if tried := attempted.Load(); tried > 0 && failed.Load() == tried {
		return fmt.Errorf("all %d repos failed to sync", tried)
	}
	log.Log(ctx, "backfill sweep complete",
		append([]any{"totalRepos", len(dids), "hosts", lanes}, progress.status()...)...)
	return nil
}

// sweepSync gives a repo the shallow sync it has never had, and reports whether
// it now has history to deepen.
func (atsync *ATProtoSynchronizer) sweepSync(ctx context.Context, progress *sweepProgress, attempted, failed *atomic.Int64, step sweepStep) bool {
	attempted.Add(1)
	repo, err := atsync.SyncBlueskyRepoCached(ctx, step.DID)
	if err != nil {
		log.Error(ctx, "failed to sync repo", "did", step.DID, "handle", step.Handle, "err", err)
		failed.Add(1)
		// A repo whose shallow sync failed is left alone for the rest of the
		// sweep: it has no Version, so deepening it would fetch the wrong
		// ranges. The next sweep retries it from the top.
		return false
	}
	progress.synced()
	if repo == nil || repo.Version == "" || repo.TerminalStatus() || repo.BackfillDone {
		return false
	}
	progress.laddered(step.DID, backfillFloorTime(repo.BackfillFloor))
	return true
}

// sweepWindow walks one window of a repo's history and reports whether it wants
// another.
func (atsync *ATProtoSynchronizer) sweepWindow(ctx context.Context, progress *sweepProgress, step sweepStep) bool {
	done, floor, err := atsync.DeepenRepo(ctx, step.DID)
	if err != nil {
		log.Error(ctx, "failed to deepen repo history", "did", step.DID, "handle", step.Handle, "err", err)
		return false
	}
	progress.window(step.DID, backfillFloorTime(floor))
	if done {
		progress.deepened(step.DID)
		log.Log(ctx, "finished deepening repo history", "did", step.DID, "handle", step.Handle, "windows", step.Windows+1)
		return false
	}
	return true
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

// sweepPlan is what a sweep has to do, read off the index once at the start.
type sweepPlan struct {
	// ready is every repo whose lane is already known, in priority order --
	// which is the order lanes are created in, and so the order they get slots
	// in. Shallow and deepening work is interleaved here rather than split,
	// because splitting it is what would put this node's own repos behind a
	// thousand strangers' lanes.
	ready []sweepItem
	// unresolved is the repos needing a shallow sync whose host the index does
	// not know; [ATProtoSynchronizer.feedUnresolved] streams them in.
	unresolved []sweepItem
	// shallow is how many repos in total need a shallow sync, ready and
	// unresolved together.
	shallow int
	// checks is how many repos are servable and believed current, and so get a
	// head check before anything else happens to them.
	checks int
	// floors is the backfill watermark of every repo that starts in a ladder,
	// for the status line's horizon.
	floors map[string]time.Time
}

// sweepPlan sorts every candidate into the work it needs.
//
// A repo row with an empty Version has never completed a sync: either brand new,
// or left half-indexed by a run that died, which is the same thing as far as
// anyone reading the index is concerned. Anything parked is left alone. Every
// other repo -- servable, and as far as this node knows current -- starts with
// a head check, including the ones with history left to fetch: a repo whose
// recent records are wrong is not made righter by deepening it, and the check
// costs one request against the several its first window will.
func (atsync *ATProtoSynchronizer) sweepPlan(dids []string) (*sweepPlan, error) {
	plan := &sweepPlan{floors: map[string]time.Time{}}
	for _, did := range dids {
		repo, err := atsync.Model.GetRepo(did)
		if err != nil {
			return nil, fmt.Errorf("failed to get repo for %s: %w", did, err)
		}
		switch {
		case repo == nil || repo.Version == "":
			plan.shallow++
			pds, handle := "", ""
			if repo != nil {
				pds, handle = repo.PDS, repo.Handle
			}
			// A row that does not name a host does not get a lane of its own
			// here: feedUnresolved finds those hosts in the background rather
			// than letting each become a lane.
			if host := reposync.HostKey(pds); host != "" {
				plan.ready = append(plan.ready, sweepItem{DID: did, Handle: handle, Lane: host})
			} else {
				plan.unresolved = append(plan.unresolved, sweepItem{DID: did, Handle: handle})
			}
		case repo.TerminalStatus():
		default:
			plan.checks++
			plan.ready = append(plan.ready, sweepItem{DID: did, Handle: repo.Handle, Lane: sweepLane(did, repo.PDS), Check: true})
			if !repo.BackfillDone {
				// It joins its lane's ladder once its head checks out, but the
				// horizon it holds is true from the moment the sweep starts.
				plan.floors[did] = backfillFloorTime(repo.BackfillFloor)
			}
		}
	}
	return plan, nil
}

// backfillFloorTime reads a backfill watermark as the instant it encodes. A repo
// with no watermark -- a row from a build that had never heard of one -- has had
// no history walked at all, so its floor is now: it holds the horizon at the
// present moment until its first window lands.
func backfillFloorTime(tid string) time.Time {
	if tid != "" {
		if t, err := reposync.TimeForTID(tid); err == nil {
			return t
		}
	}
	return time.Now()
}

// sweepProgress is the state behind the sweep's status line. It is written by
// every worker and read by the ticker, so everything goes through the mutex.
type sweepProgress struct {
	mu           sync.Mutex
	started      bool
	checkTotal   int
	checkDone    int
	shallowTotal int
	shallowDone  int
	deepenTotal  int
	deepenDone   int
	windows      int
	// floors is the watermark of every repo that is servable but not fully
	// indexed, which is the set the horizon is the maximum over. Repos leave it
	// when they finish; ones that failed stay, holding the horizon where they
	// left it, because that is the truth about what this node can serve.
	floors map[string]time.Time
}

// begin starts a sweep with the work its plan found.
func (p *sweepProgress) begin(shallow, checks int, floors map[string]time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = true
	p.checkTotal = checks
	p.checkDone = 0
	p.shallowTotal = shallow
	p.shallowDone = 0
	p.deepenTotal = len(floors)
	p.deepenDone = 0
	p.windows = 0
	p.floors = make(map[string]time.Time, len(floors))
	for did, floor := range floors {
		p.floors[did] = floor
	}
}

// checked records one repo's head having been compared with ours, however that
// went: the fraction is of checks made, so that it finishes.
func (p *sweepProgress) checked() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkDone++
}

// repairing records a head check finding drift, which is a shallow sync this
// sweep did not know it had.
func (p *sweepProgress) repairing() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shallowTotal++
}

// synced records one repo's shallow sync completing.
func (p *sweepProgress) synced() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shallowDone++
}

// laddered records a freshly synced repo joining its lane's ladder. The number
// of repos being deepened is not known when a sweep starts -- every shallow sync
// can add one -- so the denominator grows as the sweep discovers it.
func (p *sweepProgress) laddered(did string, floor time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, counted := p.floors[did]; !counted {
		// A repo the plan already expected to deepen -- one that drifted, got
		// repaired, and is on its way back to the ladder it never left -- is
		// not a second repo.
		p.deepenTotal++
	}
	p.floors[did] = floor
}

// window records one history window completing. A repo only counts as deepened
// at the bottom of its ladder, so without this the status line would read
// deepened=0 while tens of thousands of windows finished underneath it.
func (p *sweepProgress) window(did string, floor time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.windows++
	if p.floors != nil {
		p.floors[did] = floor
	}
}

// deepened records a repo reaching the start of its history.
func (p *sweepProgress) deepened(did string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deepenDone++
	delete(p.floors, did)
}

// horizon is the most recent watermark among repos that are servable but not
// fully indexed: the instant after which everything this node serves is indexed.
// It climbs backwards through history as the sweep works. Zero when there is
// nothing left to deepen.
func (p *sweepProgress) horizon() int64 {
	var newest time.Time
	for _, floor := range p.floors {
		if floor.After(newest) {
			newest = floor
		}
	}
	if newest.IsZero() {
		return 0
	}
	return newest.Unix()
}

// status is the status line's key/value pairs: how many repos have been made
// servable, how many have their whole history, how many windows that took, and
// the horizon as unix seconds.
func (p *sweepProgress) status() []any {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []any
	// Only a sweep with repos to check has anything to say about checking
	// them, which is every sweep but a fresh node's first.
	if p.checkTotal > 0 {
		out = append(out, "checked", fmt.Sprintf("%d/%d", p.checkDone, p.checkTotal))
	}
	return append(out,
		"shallow", fmt.Sprintf("%d/%d", p.shallowDone, p.shallowTotal),
		"deepened", fmt.Sprintf("%d/%d", p.deepenDone, p.deepenTotal),
		"windows", p.windows,
		"horizon", p.horizon(),
	)
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
