package atproto

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/repomgr"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"

	"github.com/gorilla/websocket"
)

type ATProtoSynchronizer struct {
	CLI       *config.CLI
	Model     model.Model
	LastSeen  time.Time
	LastEvent time.Time
	Noter     notificationpkg.FirebaseNotifier
	Bus       *bus.Bus
}

func (atsync *ATProtoSynchronizer) StartFirehose(ctx context.Context) error {
	retryCount := 0
	retryWindow := time.Now()

	for {
		if ctx.Err() != nil {
			return nil
		}
		err := atsync.StartFirehoseRetry(ctx)
		if err != nil {
			log.Error(ctx, "firehose error", "err", err)

			// Check if we're within the 1-minute window
			now := time.Now()
			if now.Sub(retryWindow) > time.Minute {
				// Reset the counter if more than a minute has passed
				retryCount = 1
				retryWindow = now
			} else {
				// Increment retry count if within the window
				retryCount++
				if retryCount >= 3 {
					log.Error(ctx, "firehose failed 3 times within a minute, crashing", "err", err)
					return fmt.Errorf("firehose failed 3 times within a minute: %w", err)
				}
			}
		}
	}
}

func (atsync *ATProtoSynchronizer) StartFirehoseRetry(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "func", "StartFirehose")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dialer := websocket.DefaultDialer
	u, err := url.Parse(atsync.CLI.RelayHost)
	if err != nil {
		return fmt.Errorf("invalid relayHost URI: %w", err)
	}
	u.Path = "xrpc/com.atproto.sync.subscribeRepos"
	// if cursor != 0 {
	// 	u.RawQuery = fmt.Sprintf("cursor=%d", cursor)
	// }
	con, _, err := dialer.Dial(u.String(), http.Header{
		"User-Agent": []string{aqhttp.UserAgent},
	})
	if err != nil {
		return fmt.Errorf("subscribing to firehose failed (dialing): %w", err)
	}

	rsc := &events.RepoStreamCallbacks{
		RepoCommit: func(evt *comatproto.SyncSubscribeRepos_Commit) error {
			go atsync.handleCommitEventOps(ctx, evt)
			return nil
		},
		Error: func(evt *events.ErrorFrame) error {
			log.Error(ctx, "firehose error", "err", evt.Error, "message", evt.Message)
			cancel()
			return fmt.Errorf("firehose error: %s", evt.Error)
		},
	}

	scheduler := parallel.NewScheduler(
		10,
		100,
		atsync.CLI.RelayHost,
		rsc.EventHandler,
	)

	log.Log(ctx, "starting firehose consumer", "relayHost", atsync.CLI.RelayHost)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return events.HandleRepoStream(ctx, con, scheduler, nil)
	})

	g.Go(func() error {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				since := time.Since(atsync.LastEvent)
				goroutines := runtime.NumGoroutine()
				if since > 10*time.Second {
					log.Warn(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
				} else {
					log.Debug(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
				}
				if time.Since(atsync.LastSeen) > 10*time.Second {
					log.Warn(ctx, fmt.Sprintf("firehose dry; no new events for %s", time.Since(atsync.LastSeen)))
				}
			}
		}
	})

	return g.Wait()
}

var CollectionFilter = []string{
	constants.PLACE_STREAM_KEY,
	constants.PLACE_STREAM_LIVESTREAM,
	constants.PLACE_STREAM_CHAT_MESSAGE,
	constants.PLACE_STREAM_CHAT_PROFILE,
	constants.APP_BSKY_GRAPH_FOLLOW,
	constants.APP_BSKY_FEED_POST,
	constants.APP_BSKY_GRAPH_BLOCK,
}

func (atsync *ATProtoSynchronizer) handleCommitEventOps(ctx context.Context, evt *comatproto.SyncSubscribeRepos_Commit) error {
	ctx = log.WithLogValues(ctx, "event", "commit", "did", evt.Repo, "rev", evt.Rev, "seq", fmt.Sprintf("%d", evt.Seq), "func", "handleCommitEventOps")
	now := time.Now()
	atsync.LastSeen = now

	if evt.TooBig {
		log.Warn(ctx, "skipping tooBig events for now")
		return nil
	}

	rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
	if err != nil {
		log.Error(ctx, "failed to read repo from car", "err", err)
		return nil
	}

	for _, op := range evt.Ops {
		collection, rkey, err := syntax.ParseRepoPath(op.Path)
		if err != nil {
			log.Error(ctx, "invalid path in repo op", "eventKind", op.Action, "path", op.Path)
			return nil
		}
		ctx = log.WithLogValues(ctx, "eventKind", op.Action, "collection", collection.String(), "rkey", rkey.String())

		if len(CollectionFilter) > 0 {
			keep := false
			for _, c := range CollectionFilter {
				if collection.String() == c {
					keep = true
					break
				}
			}
			if !keep {
				continue
			}
		}

		aqt, err := aqtime.FromString(evt.Time)
		if err != nil {
			log.Error(ctx, "failed to parse time", "err", err)
			continue
		}
		atsync.LastEvent = aqt.Time()

		r, err := atsync.Model.GetRepo(evt.Repo)
		if err != nil {
			log.Error(ctx, "failed to get repo", "err", err)
			continue
		}
		if r == nil {
			// someone we don't know aboutd
			// continue
		}
		// log.Warn(ctx, "got record we care about", "collection", collection, "rkey", rkey)

		ek := repomgr.EventKind(op.Action)
		switch ek {
		case repomgr.EvtKindCreateRecord, repomgr.EvtKindUpdateRecord:
			// read the record bytes from blocks, and verify CID
			rc, recCBOR, err := rr.GetRecordBytes(ctx, op.Path)
			if err != nil {
				log.Error(ctx, "reading record from event blocks (CAR)", "err", err)
				break
			}
			if op.Cid == nil || lexutil.LexLink(rc) != *op.Cid {
				log.Error(ctx, "mismatch between commit op CID and record block", "recordCID", rc, "opCID", op.Cid)
				break
			}

			err = atsync.handleCreateUpdate(ctx, evt.Repo, rkey, recCBOR, op.Cid.String(), collection)
			if err != nil {
				log.Error(ctx, "failed to handle create update", "err", err)
				continue
			}

		case repomgr.EvtKindDeleteRecord:
			if collection.String() == constants.APP_BSKY_GRAPH_FOLLOW {
				log.Debug(ctx, "deleting follow", "userDID", evt.Repo, "subjectDID", rkey.String())
				err := atsync.Model.DeleteFollow(ctx, evt.Repo, rkey.String())
				if err != nil {
					log.Debug(ctx, "failed to delete follow", "err", err)
				}
			}

			if collection.String() == constants.APP_BSKY_GRAPH_BLOCK {
				if r == nil {
					log.Debug(ctx, "no repo found for block", "userDID", evt.Repo, "subjectDID", rkey.String())
					continue
				}
				log.Warn(ctx, "deleting block", "userDID", evt.Repo, "subjectDID", rkey.String())
				err := atsync.Model.DeleteBlock(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete block", "err", err)
				}
			}

			if err != nil {
				return err
			}
		default:
			log.Error(ctx, "unexpected record op kind")
		}
	}
	return nil
}
