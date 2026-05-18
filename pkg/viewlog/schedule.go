package viewlog

import (
	"context"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
)

// TaskEnqueuer is the subset of *statedb.StatefulDB the periodic
// scheduler needs. Lets tests substitute a fake queue.
type TaskEnqueuer interface {
	EnqueueTask(ctx context.Context, taskType string, payload any, options ...statedb.TaskOption) (*statedb.AppTask, error)
}

// ScheduleConfig captures the cadence + dedup behavior of the
// periodic aggregator enqueuer. Buckets align on UTC multiples of
// Interval (e.g. 5-minute windows snap to :00, :05, :10, …) so every
// node computes the same bucket boundaries at the same wall-clock
// moment — the unique task-key dedup in statedb rejects duplicates,
// leaving exactly one task per bucket per cluster.
type ScheduleConfig struct {
	Interval time.Duration
	// Lag is how long after a bucket's WindowEnd we wait before
	// enqueueing it for aggregation. Must be at least one writer
	// flush interval so files have time to land in the store.
	Lag time.Duration
}

// ScheduleAggregations runs until ctx is cancelled. Every Interval,
// it computes the most recent fully-closed bucket (one Interval back
// after subtracting Lag) and enqueues a TaskViewCountAggregate task
// with a deterministic key. The DB unique constraint on TaskKey
// rejects re-enqueues, so every node trying the same thing is
// harmless.
func ScheduleAggregations(ctx context.Context, q TaskEnqueuer, cfg ScheduleConfig) error {
	if cfg.Interval <= 0 {
		return fmt.Errorf("viewlog: ScheduleAggregations needs a positive interval")
	}
	if cfg.Lag < 0 {
		cfg.Lag = 0
	}
	// Try once at startup so a fresh boot doesn't sit a full interval
	// before producing the first record. (Idempotent: if another node
	// already enqueued the bucket, statedb returns the existing row.)
	tryEnqueue(ctx, q, cfg, time.Now().UTC())

	tick := time.NewTicker(cfg.Interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-tick.C:
			tryEnqueue(ctx, q, cfg, now.UTC())
		}
	}
}

func tryEnqueue(ctx context.Context, q TaskEnqueuer, cfg ScheduleConfig, now time.Time) {
	end := bucketEnd(now.Add(-cfg.Lag), cfg.Interval)
	start := end.Add(-cfg.Interval)

	payload := statedb.ViewCountAggregateTask{
		WindowStart: start,
		WindowEnd:   end,
	}
	key := AggregateTaskKey(start, end)
	_, err := q.EnqueueTask(ctx, statedb.TaskViewCountAggregate, payload, statedb.WithTaskKey(key))
	if err != nil {
		// EnqueueTask already handles unique-constraint violation by
		// returning the existing row; a true error here is a bug.
		log.Error(ctx, "viewlog: enqueue aggregate task",
			"start", start, "end", end, "error", err)
		return
	}
	log.Debug(ctx, "viewlog: queued aggregate task", "start", start, "end", end, "task_key", key)
}

// bucketEnd snaps t down to the nearest interval boundary in UTC.
func bucketEnd(t time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		return t.UTC()
	}
	return t.UTC().Truncate(interval)
}

// AggregateTaskKey is the deterministic key used to dedup aggregator
// tasks across nodes. Same (start, end) → same key → at most one row
// in app_tasks. Exposed so tests + tooling can construct it.
func AggregateTaskKey(start, end time.Time) string {
	return fmt.Sprintf("view-count-aggregate::%s::%s",
		start.UTC().Format(time.RFC3339),
		end.UTC().Format(time.RFC3339))
}
