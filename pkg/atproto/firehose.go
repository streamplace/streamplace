package atproto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/repomgr"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/streamplace"

	"github.com/gorilla/websocket"
)

type FirehoseConsumer struct {
	cli       *config.CLI
	mod       model.Model
	lastSeen  time.Time
	lastEvent time.Time
	noter     notificationpkg.FirebaseNotifier
}

func StartFirehose(ctx context.Context, cli *config.CLI, mod model.Model, noter notificationpkg.FirebaseNotifier) error {
	ctx = log.WithLogValues(ctx, "func", "StartFirehose")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dialer := websocket.DefaultDialer
	u, err := url.Parse(cli.RelayHost)
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

	fc := &FirehoseConsumer{
		cli:      cli,
		mod:      mod,
		lastSeen: time.Now(),
		noter:    noter,
	}

	rsc := &events.RepoStreamCallbacks{
		RepoCommit: func(evt *comatproto.SyncSubscribeRepos_Commit) error {
			go fc.handleCommitEventOps(ctx, evt, mod)
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
		cli.RelayHost,
		rsc.EventHandler,
	)

	log.Log(ctx, "starting firehose consumer", "relayHost", cli.RelayHost)

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
				since := time.Since(fc.lastEvent)
				goroutines := runtime.NumGoroutine()
				if since > 10*time.Second {
					log.Warn(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
				} else {
					log.Debug(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
				}
				if time.Since(fc.lastSeen) > 10*time.Second {
					log.Warn(ctx, fmt.Sprintf("firehose dry; no new events for %s", time.Since(fc.lastSeen)))
				}
			}
		}
	})

	return g.Wait()
}

var CollectionFilter = []string{
	constants.PLACE_STREAM_KEY,
	constants.PLACE_STREAM_LIVESTREAM,
	constants.APP_BSKY_GRAPH_FOLLOW,
}

func (fc *FirehoseConsumer) handleCommitEventOps(ctx context.Context, evt *comatproto.SyncSubscribeRepos_Commit, mod model.Model) error {
	ctx = log.WithLogValues(ctx, "event", "commit", "did", evt.Repo, "rev", evt.Rev, "seq", fmt.Sprintf("%d", evt.Seq), "func", "handleCommitEventOps")
	fc.lastSeen = time.Now()

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
		fc.lastEvent = aqt.Time()

		r, err := mod.GetRepo(evt.Repo)
		if err != nil {
			log.Error(ctx, "failed to get repo", "err", err)
			continue
		}
		if r == nil {
			// someone we don't know aboutd
			continue
		}
		log.Warn(ctx, "got record we care about", "collection", collection, "rkey", rkey)

		out := make(map[string]interface{})
		out["seq"] = evt.Seq
		out["repo"] = evt.Repo
		out["rev"] = evt.Rev
		out["time"] = evt.Time
		out["collection"] = collection
		out["rkey"] = rkey

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

			switch ek {
			case repomgr.EvtKindCreateRecord:
				out["action"] = "create"
			case repomgr.EvtKindUpdateRecord:
				out["action"] = "update"
			default:
				log.Error(ctx, "impossible event kind", "kind", ek)
				break
			}
			cb, err := lexutil.CborDecodeValue(*recCBOR)
			if err != nil {
				log.Error(ctx, "failed to parse record CBOR", "err", err)
				continue
			}
			switch rec := cb.(type) {
			case *bsky.GraphFollow:
				log.Debug(ctx, "creating follow", "userDID", evt.Repo, "subjectDID", rec.Subject, "rev", evt.Rev)
				err := mod.CreateFollow(ctx, evt.Repo, rkey.String(), rec)
				if err != nil {
					log.Error(ctx, "failed to create follow", "err", err)
				}
			case *streamplace.Livestream:
				var u string
				if rec.Url != nil {
					u = *rec.Url
				}
				log.Warn(ctx, "Livestream detected! Blasting followers!", "title", rec.Title, "url", u, "createdAt", rec.CreatedAt, "repo", evt.Repo)
				notifications, err := mod.GetFollowersNotificationTokens(evt.Repo)
				if err != nil {
					return err
				}

				nb := &notificationpkg.NotificationBlast{
					Title: fmt.Sprintf("🔴 @%s is LIVE!", r.Handle),
					Body:  rec.Title,
					Data: map[string]string{
						"path": fmt.Sprintf("/%s", r.Handle),
					},
				}
				if fc.noter != nil {
					err := fc.noter.Blast(ctx, notifications, nb)
					if err != nil {
						log.Error(ctx, "failed to blast notifications", "err", err)
					} else {
						log.Log(ctx, "sent notifications", "user", evt.Repo, "count", len(notifications), "content", nb)
					}
				} else {
					log.Log(ctx, "no notifier configured, skipping notifications", "user", evt.Repo, "count", len(notifications), "content", nb)
				}
			default:
				log.Debug(ctx, "unhandled record type", "type", reflect.TypeOf(rec))
			}
			d, err := data.UnmarshalCBOR(*recCBOR)
			if err != nil {
				slog.Warn("failed to parse record CBOR")
				continue
			}
			out["cid"] = op.Cid.String()
			out["record"] = d
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			log.Debug(ctx, "got record", "record", string(b))
		case repomgr.EvtKindDeleteRecord:
			out["action"] = "delete"
			if collection.String() == constants.APP_BSKY_GRAPH_FOLLOW {
				log.Debug(ctx, "deleting follow", "userDID", evt.Repo, "subjectDID", rkey.String())
				err := mod.DeleteFollow(ctx, evt.Repo, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete follow", "err", err)
				}
			}
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			log.Debug(ctx, "got record", "record", string(b))
		default:
			log.Error(ctx, "unexpected record op kind")
		}
	}
	return nil
}
