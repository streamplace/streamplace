package director

import (
	"context"
	"errors"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/stationshard"
)

// Recording and transcoding are origin-only; multistream can run on any
// carrier. Standalone sessions use SQL leases; sharded stations use fenced leases.
const (
	roleRecord      = "record"
	roleMultistream = "multistream"
	roleTranscode   = "transcode"
)

var (
	// roleLeaseTTL is how long a dead holder keeps a job from the others.
	roleLeaseTTL = 6 * time.Second
	// roleLeaseRenew is how often a holder renews, and a contender asks.
	roleLeaseRenew = 2 * time.Second
	// roleRetryAfter is how long a node that won a job but could not take
	// it up stays out of contention, leaving the job to a peer that can.
	roleRetryAfter = 10 * time.Second
)

// errRoleDeclined is a role's start saying this node will not do the job
// for this stream at all (the streamer turned recording off, say): the
// lease goes back and the session stops contending for it.
var errRoleDeclined = errors.New("role declined")

// role is one job: what taking it up and putting it down means. start gets
// a context that ends when the role is lost or the session ends; an error
// means the job was not taken up, and its lease is handed back.
type role struct {
	name             string
	start            func(ctx context.Context, lease stationshard.Lease) error
	stop             func(drainCtx context.Context)
	firstAttemptDone func()
	failed           func() <-chan struct{}
}

func (r *role) finishFirstAttempt() {
	if r.firstAttemptDone != nil {
		r.firstAttemptDone()
		r.firstAttemptDone = nil
	}
}

// startRoles enters the session into contention for driving the stream's
// multistream targets, which any node carrying it can do. Once per session,
// on the first segment of either provenance. For the session's first
// segment the director runs Start in the background and NewSegment right
// after it, so the two calls can race; sync.Once lets one of them do the
// work and the other return. The loops run on the session's context, so
// ending the session puts every job down.
func (ss *StreamSession) startRoles(repoDID string) {
	ss.roles.Do(func() {
		ctx := ss.ctx
		var done chan struct{}
		ss.contend(ctx, repoDID, role{
			name: roleMultistream,
			start: func(rctx context.Context, _ stationshard.Lease) error {
				done = make(chan struct{})
				ss.Go(ctx, func() error {
					defer close(done)
					return ss.HandleMultistreamTargets(rctx)
				})
				return nil
			},
			stop: func(context.Context) { <-done },
		})
	})
}

// startOriginRoles enters the session into contention for recording and
// transcoding, once, and only on a node that is an origin for the stream:
// one that ingests it without pushing it on (see ingestOrigin), or that a
// peer pushes it to. Both jobs want the stream at its live edge. Peers take
// a stream's renditions from its origin, so renditions minted anywhere else
// would reach nobody but that node's own viewers; and a recorder lagging
// the one it takes over from would record again what that one had saved.
func (ss *StreamSession) startOriginRoles(repoDID string) {
	ss.originRoles.Do(func() {
		ctx := ss.ctx
		if ss.cli.S3Configured() {
			recording := role{
				name: roleRecord,
				start: func(rctx context.Context, lease stationshard.Lease) error {
					return ss.startS3Upload(rctx, repoDID, lease)
				},
				stop: ss.s3Close,
			}
			if shards := ss.statefulDB.StationShards; shards != nil {
				ss.beginS3Initialization(ctx, shards.TTL())
				recording.firstAttemptDone = func() { ss.installS3Uploader(nil) }
				recording.failed = ss.s3PersistenceFailed
			}
			ss.contend(ctx, repoDID, recording)
		}
		if ss.cli.LivepeerGatewayURL != "" {
			ss.contend(ctx, repoDID, role{
				name: roleTranscode,
				start: func(rctx context.Context, _ stationshard.Lease) error {
					ss.transcodeMu.Lock()
					ss.transcoding = rctx
					ss.transcodeMu.Unlock()
					return nil
				},
				stop: func(context.Context) {
					ss.transcodeMu.Lock()
					ss.transcoding = nil
					ss.transcodeMu.Unlock()
					ss.transcodePending.Wait()
				},
			})
		}
	})
}

// contend runs one role's lease loop for the life of the session: take the
// lease when it is free, renew it while held, give the job up when it is
// lost, and release it on the way out so a peer takes over at once. A job
// won but not taken up is released at once too, and asked for again after
// roleRetryAfter, or never, when declined. Standalone starts synchronously
// to retain first-segment behavior; shard I/O stays off the director loop.
func (ss *StreamSession) contend(ctx context.Context, repoDID string, r role) {
	if shards := ss.statefulDB.StationShards; shards != nil {
		ss.Go(ctx, func() error {
			contendShard(ctx, shards, repoDID, r)
			return nil
		})
		return
	}
	name := "stream:" + r.name + ":" + repoDID
	holder := ss.cli.ServerDID()
	ctx = log.WithLogValues(ctx, "role", r.name)
	var held bool
	var renewed, retryAt time.Time
	var cancel context.CancelFunc
	release := func() {
		rctx, done := context.WithTimeout(context.WithoutCancel(ctx), roleLeaseRenew)
		defer done()
		if err := ss.statefulDB.ReleaseLease(rctx, name, holder); err != nil {
			log.Warn(ctx, "could not release stream role lease", "error", err)
		}
	}
	// check reports whether to go on contending.
	check := func() bool {
		if !held && time.Now().Before(retryAt) {
			return true
		}
		// The job is kept only while its last confirmed lease lasts, so a
		// renewal the database never answers gives up when that runs out.
		asked := time.Now()
		deadline := asked.Add(roleLeaseRenew)
		if held {
			deadline = renewed.Add(roleLeaseTTL)
		}
		dctx, done := context.WithDeadline(ctx, deadline)
		ok, err := ss.statefulDB.AcquireLease(dctx, name, holder, roleLeaseTTL)
		done()
		if err != nil {
			// A database we cannot reach cannot have handed the lease to
			// anyone either, unless this node alone is cut off: keep the
			// job until the lease would have lapsed.
			log.Warn(ctx, "stream role lease check failed", "error", err)
			ok = held && time.Since(renewed) < roleLeaseTTL
		} else if ok {
			renewed = asked
		}
		switch {
		case ok && !held:
			rctx, stop := context.WithCancel(ctx)
			if err := r.start(rctx, stationshard.Lease{}); err != nil {
				stop()
				release()
				if errors.Is(err, errRoleDeclined) {
					log.Log(ctx, "declined stream role; leaving it to other nodes", "reason", err)
					return false
				}
				log.Warn(ctx, "could not take up stream role; leaving it to other nodes for now", "error", err, "retryAfter", roleRetryAfter)
				retryAt = time.Now().Add(roleRetryAfter)
				return true
			}
			if time.Since(renewed) >= roleLeaseTTL {
				// Taking the job up (recording's lookups) outlasted the
				// lease, so a peer may hold it by now: put it straight back
				// down rather than do it alongside, and ask again.
				log.Warn(ctx, "taking up stream role outlasted its lease; putting it down", "took", time.Since(asked))
				stop()
				r.stop(context.WithoutCancel(ctx))
				return true
			}
			held, cancel = true, stop
			log.Log(ctx, "taking up stream role")
		case !ok && held:
			log.Log(ctx, "stream role taken over by another node")
			held = false
			cancel()
			r.stop(context.WithoutCancel(ctx))
		}
		return true
	}
	if !check() {
		return
	}
	ss.Go(ctx, func() error {
		defer func() {
			if held {
				cancel()
				r.stop(context.WithoutCancel(ctx))
				release()
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(roleLeaseRenew):
			}
			if !check() {
				return nil
			}
		}
	})
}

type shardRoleStore interface {
	ClaimRole(context.Context, string, string) (stationshard.Lease, error)
	RenewRole(context.Context, stationshard.Lease) error
	ReleaseRole(context.Context, stationshard.Lease) error
	TTL() time.Duration
}

func contendShard(ctx context.Context, shards shardRoleStore, did string, r role) {
	ctx = log.WithLogValues(ctx, "role", r.name, "did", did)
	defer r.finishFirstAttempt()
	ttl := shards.TTL()
	interval := ttl / 3
	for ctx.Err() == nil {
		asked := time.Now()
		claimCtx, cancel := context.WithDeadline(ctx, asked.Add(interval))
		lease, err := shards.ClaimRole(claimCtx, did, r.name)
		cancel()
		if err == nil {
			if !runShardRole(ctx, shards, &r, lease, asked.Add(ttl), ttl, interval) {
				return
			}
		} else if !errors.Is(err, stationshard.ErrHeld) {
			log.Warn(ctx, "could not claim shard role", "error", err)
		}
		r.finishFirstAttempt()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func runShardRole(ctx context.Context, shards shardRoleStore, r *role, lease stationshard.Lease, validUntil time.Time, ttl, interval time.Duration) bool {
	release := func() {
		if !time.Now().Before(validUntil) {
			return
		}
		// ReleaseRole conditionally clears only this exact lease, even if
		// a recording write already observed a takeover.
		releaseCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), validUntil)
		defer cancel()
		if err := shards.ReleaseRole(releaseCtx, lease); err != nil && !errors.Is(err, stationshard.ErrFenced) {
			log.Warn(ctx, "could not release shard role", "error", err)
		}
	}
	if ctx.Err() != nil || !time.Now().Before(validUntil) {
		r.finishFirstAttempt()
		release()
		return true
	}
	roleCtx, cancelRole := context.WithCancel(ctx)
	startTimer := time.AfterFunc(time.Until(validUntil), cancelRole)
	err := r.start(roleCtx, lease)
	startTimer.Stop()
	r.finishFirstAttempt()
	if err != nil {
		cancelRole()
		if !errors.Is(err, stationshard.ErrFenced) {
			release()
		}
		if errors.Is(err, errRoleDeclined) {
			return false
		}
		log.Warn(ctx, "could not take up shard role", "error", err)
		return true
	}
	log.Log(ctx, "taking up shard role", "epoch", lease.Epoch)
	var failed <-chan struct{}
	if r.failed != nil {
		failed = r.failed()
	}

	// Measure validity from the request, not its ACK. An uncertain renewal
	// cannot extend local validity, and a fenced token must never be released.
	renew := func(parent context.Context) bool {
		asked := time.Now()
		if !asked.Before(validUntil) {
			return false
		}
		renewCtx, cancel := context.WithDeadline(parent, minTime(validUntil, asked.Add(interval)))
		err := shards.RenewRole(renewCtx, lease)
		cancel()
		if errors.Is(err, stationshard.ErrFenced) || !time.Now().Before(validUntil) {
			return false
		}
		if err != nil {
			log.Warn(ctx, "could not renew shard role", "error", err)
		} else {
			validUntil = asked.Add(ttl)
		}
		return true
	}

	failedNow := func() bool {
		select {
		case <-failed:
			return true
		default:
			return false
		}
	}
	valid := time.Now().Before(validUntil)
	for valid && ctx.Err() == nil && !failedNow() {
		select {
		case <-ctx.Done():
		case <-failed:
		case <-time.After(min(interval, time.Until(validUntil))):
			if !failedNow() {
				valid = renew(ctx)
			}
		}
	}
	cancelRole()

	// Stop accepting work immediately; on clean shutdown keep renewing while
	// in-flight work drains, bounded by one TTL. On loss, cancel the drain.
	drainCtx, cancelDrain := context.WithTimeout(context.WithoutCancel(ctx), ttl)
	defer cancelDrain()
	if !valid || !time.Now().Before(validUntil) {
		cancelDrain()
		r.stop(drainCtx)
		return true
	}
	if failedNow() {
		cancelDrain()
		r.stop(drainCtx)
		release()
		return true
	}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		r.stop(drainCtx)
	}()
	for valid {
		select {
		case <-drained:
			release()
			return true
		case <-failed:
			cancelDrain()
			<-drained
			release()
			return true
		case <-drainCtx.Done():
			<-drained
			release()
			return true
		case <-time.After(min(interval, time.Until(validUntil))):
			if !failedNow() {
				valid = renew(drainCtx)
			}
		}
	}
	cancelDrain()
	<-drained
	return true
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
