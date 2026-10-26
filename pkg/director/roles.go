package director

import (
	"context"
	"errors"
	"time"

	"stream.place/streamplace/pkg/log"
)

// The jobs a live stream needs doing exactly once however many nodes carry
// it: recording it to S3 for live-to-VOD, driving its multistream targets,
// transcoding its renditions. Every node with a session for the stream,
// whether it ingested the segments or received them from a peer, competes
// for each job it is equipped to do through a lease in the station's shared
// statedb. The holder renews the lease; a holder that dies stops renewing,
// and the next node to ask after roleLeaseTTL takes the job over. A node on
// its own statedb simply holds every lease.
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
	name  string
	start func(ctx context.Context) error
	stop  func()
}

// startRoles enters the session into contention for the stream's jobs.
// Once per session, on the first segment of either provenance. For the
// session's first segment the director runs Start in the background and
// NewSegment right after it, so the two calls can race; sync.Once lets one
// of them do the work and the other return. The loops run on the
// session's context, so ending the session puts every job down.
func (ss *StreamSession) startRoles(repoDID string) {
	ss.roles.Do(func() {
		ctx := ss.ctx
		if ss.cli.S3Configured() {
			ss.contend(ctx, repoDID, role{
				name:  roleRecord,
				start: func(context.Context) error { return ss.startS3Upload(ctx, repoDID) },
				stop:  func() { ss.s3Close(ctx) },
			})
		}
		ss.contend(ctx, repoDID, role{
			name: roleMultistream,
			start: func(rctx context.Context) error {
				ss.Go(ctx, func() error { return ss.HandleMultistreamTargets(rctx) })
				return nil
			},
			stop: func() {},
		})
	})
}

// startTranscodeRole enters the session into contention for transcoding,
// once, and only on a node that is an origin for the stream: one that
// ingests it without pushing it on (see ingestOrigin), or that a peer
// pushes it to. Peers take a stream's
// renditions from its origin, so renditions minted anywhere else would
// reach nobody but that node's own viewers.
func (ss *StreamSession) startTranscodeRole(repoDID string) {
	if ss.cli.LivepeerGatewayURL == "" {
		return
	}
	ss.transcodeRole.Do(func() {
		ss.contend(ss.ctx, repoDID, role{
			name: roleTranscode,
			start: func(context.Context) error {
				ss.transcoding.Store(true)
				return nil
			},
			stop: func() { ss.transcoding.Store(false) },
		})
	})
}

// contend runs one role's lease loop for the life of the session: take the
// lease when it is free, renew it while held, give the job up when it is
// lost, and release it on the way out so a peer takes over at once. A job
// won but not taken up is released at once too, and asked for again after
// roleRetryAfter, or never, when declined. The first check happens before
// contend returns, so a job this node wins is up before the segment that
// started the contention is handled; the rest run in the background.
func (ss *StreamSession) contend(ctx context.Context, repoDID string, r role) {
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
			if err := r.start(rctx); err != nil {
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
			held, cancel = true, stop
			log.Log(ctx, "taking up stream role")
		case !ok && held:
			log.Log(ctx, "stream role taken over by another node")
			held = false
			cancel()
			r.stop()
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
				r.stop()
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
