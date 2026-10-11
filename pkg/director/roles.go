package director

import (
	"context"
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
)

// role is one job: what taking it up and putting it down means. start gets
// a context that ends when the role is lost or the session ends.
type role struct {
	name  string
	start func(ctx context.Context)
	stop  func()
}

// startRoles enters the session into contention for the stream's jobs.
// Once per session, on the first segment of either provenance. For the
// session's first segment the director runs Start in the background and
// NewSegment right after it, so the two calls can race; sync.Once lets one
// of them do the work and the other return.
func (ss *StreamSession) startRoles(ctx context.Context, repoDID string) {
	ss.roles.Do(func() {
		if ss.cli.S3Configured() {
			ss.contend(ctx, repoDID, role{
				name:  roleRecord,
				start: func(context.Context) { ss.startS3Upload(ctx, repoDID) },
				stop:  func() { ss.s3Close(ctx) },
			})
		}
		ss.contend(ctx, repoDID, role{
			name: roleMultistream,
			start: func(rctx context.Context) {
				ss.Go(ctx, func() error { return ss.HandleMultistreamTargets(rctx) })
			},
			stop: func() {},
		})
		if ss.cli.LivepeerGatewayURL != "" {
			ss.contend(ctx, repoDID, role{
				name:  roleTranscode,
				start: func(context.Context) { ss.transcoding.Store(true) },
				stop:  func() { ss.transcoding.Store(false) },
			})
		}
	})
}

// contend runs one role's lease loop for the life of the session: take the
// lease when it is free, renew it while held, give the job up when it is
// lost, and release it on the way out so a peer takes over at once.
func (ss *StreamSession) contend(ctx context.Context, repoDID string, r role) {
	name := "stream:" + r.name + ":" + repoDID
	holder := ss.cli.ServerDID()
	ctx = log.WithLogValues(ctx, "role", r.name)
	ss.Go(ctx, func() error {
		var held bool
		var renewed time.Time
		var cancel context.CancelFunc
		drop := func() {
			held = false
			cancel()
			r.stop()
		}
		defer func() {
			if held {
				drop()
				if err := ss.statefulDB.ReleaseLease(name, holder); err != nil {
					log.Warn(ctx, "could not release stream role lease", "error", err)
				}
			}
		}()
		for {
			ok, err := ss.statefulDB.AcquireLease(name, holder, roleLeaseTTL)
			if err != nil {
				// A database we cannot reach cannot have handed the
				// lease to anyone either, unless this node alone is cut
				// off: keep the job until the lease would have lapsed.
				log.Warn(ctx, "stream role lease check failed", "error", err)
				ok = held && time.Since(renewed) < roleLeaseTTL
			} else if ok {
				renewed = time.Now()
			}
			switch {
			case ok && !held:
				held = true
				rctx, stop := context.WithCancel(ctx)
				cancel = stop
				log.Log(ctx, "taking up stream role")
				r.start(rctx)
			case !ok && held:
				log.Log(ctx, "stream role taken over by another node")
				drop()
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(roleLeaseRenew):
			}
		}
	})
}
