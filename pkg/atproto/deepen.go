package atproto

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

// deepenIdleInterval is how long the deepener waits before scanning again when
// a pass achieved nothing -- either because every repo already has its whole
// history, or because the ones that do not are failing. Long enough that a node
// with nothing to fetch costs one query every ten minutes, short enough that a
// repo discovered by the firehose starts getting its history within one.
const deepenIdleInterval = 10 * time.Minute

// DeepenForever fetches this node's history, continuously and slowly, for as
// long as ctx lives.
//
// History is the one part of the sync engine nothing waits for. A repo is
// servable the moment its shallow sync lands; everything older is a nicety that
// arrives when it arrives. Boot used to be when all of it arrived at once --
// the sweep drains every repo's ladder in one pass, and restarts come round
// more often than the six-hour sweep interval does, so in practice a serving
// node spent its every boot replaying years of the network's chat at full lane
// speed. So this is deliberately a trickle: one global rate limiter over the
// whole node (see [config.DefaultDeepenRate]), and no relationship at all to
// the pace of [ATProtoSynchronizer.SweepForever]'s reconciliation passes.
//
// It scans, ladders what it finds, and scans again. The scan is the entire
// state machine: BackfillFloor and BackfillDone on the repo rows say where each
// repo got to, so a pass that is cancelled, a process that is killed and a node
// that is redeployed all resume from the same place, and repos discovered while
// a pass was running are picked up by the next one.
//
// A pass and a sweep may run at once and may even touch the same repo. Nothing
// guards against it because nothing needs to: the per-DID handleLock serializes
// the walks, the per-host pdsLock serializes the requests, and the row updates
// are compare-and-swap -- AdvanceRepoBackfill refuses a row somebody has
// wedged for repair -- so the loser of any race leaves the row alone and
// re-walks the window later.
func (atsync *ATProtoSynchronizer) DeepenForever(ctx context.Context) {
	// The same hold the boot sweep takes, for the same reason: a warm index's
	// busiest minutes are the ones just after a restart, and history is in no
	// hurry. A fresh index waits for nothing, which is the initial build --
	// the case where this loop IS the boot work.
	if !deepenSleep(ctx, atsync.sweepBootDelay(ctx)) {
		return
	}
	limiter := deepenLimiter(atsync.deepenRate())
	for ctx.Err() == nil {
		windows, err := atsync.deepenPass(ctx, limiter)
		if err != nil && ctx.Err() == nil {
			log.Error(ctx, "history deepening pass failed", "err", err)
		}
		if ctx.Err() != nil {
			return
		}
		if windows == 0 {
			// Nothing to deepen, or nothing that can be: either way, asking
			// again immediately would be a hot loop over the same answer.
			if !deepenSleep(ctx, deepenIdleInterval) {
				return
			}
		}
	}
}

// deepenSleep waits out d, reporting whether the context outlived it.
func deepenSleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// deepenPass walks the history of every repo that owes any, and reports how
// many windows that came to.
//
// It is the sweep's lane machinery with nothing but ladders in it: work is
// sharded by host, one worker per host and at most --sweep-concurrency of them,
// and within a host it is breadth-first, so every account a PDS serves reaches
// a week of history before any of them reaches a month. What is different is
// the pace -- every window waits on the node-wide limiter first -- and that
// zero repos is an ordinary outcome rather than the end of anything.
func (atsync *ATProtoSynchronizer) deepenPass(ctx context.Context, limiter *rate.Limiter) (int, error) {
	items, floors, err := atsync.deepenCandidates(ctx)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	progress := &sweepProgress{mode: sweepModeDeepen, rate: deepenRateLabel(limiter)}
	progress.begin(0, 0, floors)
	stop := progress.start(ctx)
	defer stop()

	log.Log(ctx, "deepening repo history", "repos", len(items),
		"hosts", laneCount(items), "rate", progress.rate)

	// The lane cap here is the deepener's own --sweep-concurrency budget, not
	// one shared with a sweep that may be running: what keeps a host from being
	// talked to twice at once is its pdsLock, which both passes take.
	sched := newLaneScheduler(ctx, atsync.sweepConcurrency(), func(ctx context.Context, step sweepStep) bool {
		// The cap is on window walks rather than on requests: a window is one
		// range walk against one host and one burst of writes into the index,
		// which is the unit both of them feel.
		//
		// A lane waiting here is holding its slot, so a paced node runs fewer
		// hosts at once than its cap allows. That is the rate doing its job:
		// with a node-wide budget the slots stop being the scarce resource, and
		// a pass takes windows/rate however they are spread.
		if err := limiter.Wait(ctx); err != nil {
			return false
		}
		return atsync.sweepWindow(ctx, progress, step)
	})
	for _, item := range items {
		sched.add(item)
	}
	lanes, err := sched.wait()
	if err != nil {
		return progress.windowsWalked(), err
	}
	windows := progress.windowsWalked()
	log.Log(ctx, "deepening pass complete", append([]any{"repos", len(items), "hosts", lanes}, progress.status()...)...)
	return windows, nil
}

// deepenCandidates is every repo that is servable and still owes history,
// laned by host, together with the watermarks they start from.
//
// A repo with a completed sync always has a host recorded on its row -- the
// sync that completed is what wrote it -- so unlike a sweep this needs no
// identity resolution to shard by. A row that somehow names no host still gets
// a lane of its own rather than sharing one, exactly as [sweepLane] says.
func (atsync *ATProtoSynchronizer) deepenCandidates(ctx context.Context) ([]sweepItem, map[string]time.Time, error) {
	dids, err := atsync.sweepCandidates(ctx)
	if err != nil {
		return nil, nil, err
	}
	var items []sweepItem
	floors := map[string]time.Time{}
	for _, did := range dids {
		repo, err := atsync.Model.GetRepo(did)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get repo for %s: %w", did, err)
		}
		switch {
		case repo == nil || repo.Version == "":
			// Never synced, or wedged for repair: the sweep's shallow queue
			// owns those, and deepening one would read the wrong ranges.
			continue
		case repo.TerminalStatus() || repo.BackfillDone:
			continue
		}
		items = append(items, sweepItem{
			DID:    did,
			Handle: repo.Handle,
			Lane:   sweepLane(did, repo.PDS),
			Deepen: true,
		})
		floors[did] = backfillFloorTime(repo.BackfillFloor)
	}
	return items, floors, nil
}

// deepenRate is how many history windows a minute this node walks. 0 is
// unlimited; anything negative is the default.
func (atsync *ATProtoSynchronizer) deepenRate() int {
	if atsync.CLI == nil || atsync.CLI.DeepenRate < 0 {
		return config.DefaultDeepenRate
	}
	return atsync.CLI.DeepenRate
}

// deepenBurstWindow is how much of the budget the limiter will hand out at
// once: a few seconds' worth, so that a lane which has been waiting on a slow
// host is not then made to wait on the clock, while a node still cannot empty a
// minute of budget into one host in one go.
const deepenBurstWindow = 3 * time.Second

// deepenLimiter turns a windows-per-minute cap into the limiter that paces
// them. Zero means no cap, which is [rate.Inf]: a limiter that exists and never
// waits, so there is nothing to nil-check at the call site.
func deepenLimiter(windowsPerMinute int) *rate.Limiter {
	if windowsPerMinute <= 0 {
		return rate.NewLimiter(rate.Inf, 1)
	}
	perSecond := float64(windowsPerMinute) / 60
	burst := int(perSecond * deepenBurstWindow.Seconds())
	if burst < 1 {
		burst = 1
	}
	return rate.NewLimiter(rate.Limit(perSecond), burst)
}

// deepenRateLabel renders a limiter's pace for the status line.
func deepenRateLabel(limiter *rate.Limiter) string {
	if limiter == nil || limiter.Limit() == rate.Inf {
		return "unlimited"
	}
	return fmt.Sprintf("%d/min", int(float64(limiter.Limit())*60))
}

// windowsWalked is how many history windows this pass has landed.
func (p *sweepProgress) windowsWalked() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.windows
}
