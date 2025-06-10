package resync

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

// resync a fresh database from the PDSses, copying over the few pieces of local state
// that we have
func Resync(ctx context.Context, cli *config.CLI) error {
	oldMod, err := model.MakeDB(cli.DBPath)
	if err != nil {
		return err
	}
	tempDBPath := cli.DBPath + ".temp." + fmt.Sprintf("%d", time.Now().UnixNano())
	newMod, err := model.MakeDB(tempDBPath)
	if err != nil {
		return err
	}
	repos, err := oldMod.GetAllRepos()
	if err != nil {
		return err
	}

	atsync := &atproto.ATProtoSynchronizer{
		CLI:   cli,
		Model: newMod,
		Noter: nil,
		Bus:   bus.NewBus(),
	}

	doneMap := make(map[string]bool)

	g, ctx := errgroup.WithContext(ctx)

	doneChan := make(chan string)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case did := <-doneChan:
				doneMap[did] = true
			case <-time.After(10 * time.Second):
				for _, repo := range repos {
					if !doneMap[repo.DID] {
						log.Warn(ctx, "remaining repos to sync", "did", repo.DID, "handle", repo.Handle, "pds", repo.PDS)
					}
				}
			}
		}
	}()

	for _, repo := range repos {
		repo := repo // capture range variable
		doneMap[repo.DID] = false
		g.Go(func() error {
			log.Warn(ctx, "syncing repo", "did", repo.DID, "handle", repo.Handle)
			ctx := log.WithLogValues(ctx, "resyncDID", repo.DID, "resyncHandle", repo.Handle)
			_, err := atsync.SyncBlueskyRepoCached(ctx, repo.Handle, newMod)
			if err != nil {
				log.Error(ctx, "failed to sync repo", "did", repo.DID, "handle", repo.Handle, "err", err)
				return nil
			}
			log.Log(ctx, "synced repo", "did", repo.DID, "handle", repo.Handle)
			doneChan <- repo.DID
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	oauthSessions, err := oldMod.ListOAuthSessions()
	if err != nil {
		return err
	}
	for _, session := range oauthSessions {
		err := newMod.CreateOAuthSession(session.DownstreamDPoPJKT, &session)
		if err != nil {
			return fmt.Errorf("failed to create oauth session: %w", err)
		}
	}
	log.Log(ctx, "migrated oauth sessions", "count", len(oauthSessions))

	notificationTokens, err := oldMod.ListNotifications()
	if err != nil {
		return err
	}
	for _, token := range notificationTokens {
		err := newMod.CreateNotification(token.Token, token.RepoDID)
		if err != nil {
			return fmt.Errorf("failed to create notification: %w", err)
		}
	}
	log.Log(ctx, "migrated notification tokens", "count", len(notificationTokens))

	log.Log(ctx, "resync complete!", "newDBPath", tempDBPath)

	return nil
}
