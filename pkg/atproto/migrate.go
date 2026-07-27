package atproto

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

func (atsync *ATProtoSynchronizer) Migrate(ctx context.Context) error {
	// Accounts that are deactivated, deleted, or taken down fail their backfill
	// the same way on every boot forever. One query up front keeps them out of
	// the sweep entirely, instead of one logged failure each.
	terminalDIDs, err := atsync.Model.TerminalRepoDIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list repos in terminal states: %w", err)
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
			return err
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
	log.Log(ctx, "starting migration sync", "totalRepos", len(allDIDs))

	g, ctx := errgroup.WithContext(ctx)
	var syncedCount int64

	syncErrors := map[string]error{}
	syncErrorMu := sync.Mutex{}

	// Start progress logging goroutine
	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-progressCtx.Done():
				return
			case <-ticker.C:
				current := atomic.LoadInt64(&syncedCount)
				log.Log(ctx, "migration progress", "synced", current, "total", len(allDIDs))
			}
		}
	}()

	for i, did := range allDIDs {
		currentIndex := i
		currentDID := did
		g.Go(func() error {
			log.Debug(ctx, "syncing repo", "did", currentDID, "progress", currentIndex+1, "total", len(allDIDs))
			_, err := atsync.SyncBlueskyRepoCached(ctx, currentDID)
			if err != nil {
				log.Error(ctx, "failed to sync repo", "did", currentDID, "err", err)
				syncErrorMu.Lock()
				syncErrors[currentDID] = err
				syncErrorMu.Unlock()
			} else {
				atomic.AddInt64(&syncedCount, 1)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Error(ctx, "migration failed", "err", err, "synced", atomic.LoadInt64(&syncedCount), "total", len(allDIDs))
		return err
	}

	for did, err := range syncErrors {
		log.Error(ctx, "migration failed for user", "did", did, "err", err)
	}

	if len(allDIDs) > 0 && len(syncErrors) == len(allDIDs) {
		return fmt.Errorf("all users failed to migrate")
	}

	log.Log(ctx, "migration completed", "synced", len(allDIDs))
	return nil
}
