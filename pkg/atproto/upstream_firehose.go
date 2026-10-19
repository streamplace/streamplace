package atproto

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

const OAuthSessionTopic = "atproto:oauth-sessions"

// The set of users comes from persisted successful local logins. The map here
// owns only live subscriptions, never authorization or an independent allowlist.
func (atsync *ATProtoSynchronizer) startUpstreamFirehose(ctx context.Context) error {
	if atsync.UpstreamStation == nil {
		return fmt.Errorf("upstream station has not been resolved")
	}
	relay := atsync.UpstreamStation.FirehoseURL
	spmetrics.TouchRelay(relay, "moq")
	changes := atsync.Bus.Subscribe(OAuthSessionTopic)
	defer atsync.Bus.Unsubscribe(OAuthSessionTopic, changes)
	// Login and revocation wake immediately; the timer also observes session
	// expiry and changes persisted by another process sharing the state DB.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	active := make(map[string]context.CancelFunc)
	var workers sync.WaitGroup
	defer func() {
		for _, cancel := range active {
			cancel()
		}
		workers.Wait()
	}()
	for {
		dids, err := atsync.StatefulDB.IndexingDIDs(ctx)
		if err != nil {
			return fmt.Errorf("load upstream subscriptions: %w", err)
		}
		wanted := make(map[string]struct{}, len(dids))
		for _, did := range dids {
			wanted[did] = struct{}{}
		}
		for did, cancel := range active {
			if _, ok := wanted[did]; !ok {
				cancel()
				delete(active, did)
			}
		}
		for _, did := range dids {
			if _, ok := active[did]; ok {
				continue
			}
			userCtx, cancel := context.WithCancel(ctx)
			active[did] = cancel
			workers.Add(1)
			go func() {
				defer workers.Done()
				atsync.consumeRelay(userCtx, relay, did)
			}()
			log.Log(ctx, "subscribing to upstream user", "did", did, "relay", relay)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changes:
		case <-ticker.C:
		}
	}
}

// A consumed request leaves room for one follow-up if another reconnect
// happens during repair. Disconnects do not cancel it; user revocation does.
func (atsync *ATProtoSynchronizer) upstreamRepairLoop(ctx context.Context, did string, requests <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-requests:
			if err := atsync.repairSubscribedRepo(ctx, did); err != nil && ctx.Err() == nil {
				log.Error(ctx, "failed to repair upstream repo after subscribing", "did", did, "err", err)
			}
		}
	}
}
