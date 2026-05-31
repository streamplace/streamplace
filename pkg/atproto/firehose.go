package atproto

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/repomgr"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/statedb"

	"slices"

	"github.com/gorilla/websocket"
)

type ATProtoSynchronizer struct {
	CLI                *config.CLI
	Model              model.Model
	StatefulDB         *statedb.StatefulDB
	LastSeen           time.Time
	LastEvent          time.Time
	Noter              notificationpkg.FirebaseNotifier
	Bus                *bus.Bus
	PLCDirectory       identity.Directory
	CachedPLCDirectory identity.Directory
	OATProxy           *oatproxy.OATProxy
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
		RepoIdentity: func(evt *comatproto.SyncSubscribeRepos_Identity) error {
			go atsync.handleIdentityEventOps(ctx, evt)
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
		err := events.HandleRepoStream(ctx, con, scheduler, nil)
		if err != nil {
			log.Error(ctx, "firehose error", "err", err)
			return err
		}
		return nil
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
	constants.APP_BSKY_GRAPH_FOLLOW,
	constants.APP_BSKY_FEED_POST,
	constants.APP_BSKY_GRAPH_BLOCK,
	constants.APP_BSKY_ACTOR_PROFILE,
	constants.PLACE_STREAM_LIVE_RECOMMENDATIONS,
}

func (atsync *ATProtoSynchronizer) handleCommitEventOps(ctx context.Context, evt *comatproto.SyncSubscribeRepos_Commit) {
	ctx = log.WithLogValues(ctx, "event", "commit", "did", evt.Repo, "rev", evt.Rev, "seq", fmt.Sprintf("%d", evt.Seq), "func", "handleCommitEventOps")
	now := time.Now()
	atsync.LastSeen = now

	if evt.TooBig {
		log.Warn(ctx, "skipping tooBig events for now")
		return
	}

	rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
	if err != nil {
		log.Error(ctx, "failed to read repo from car", "err", err)
		return
	}

	for _, op := range evt.Ops {
		collection, rkey, err := syntax.ParseRepoPath(op.Path)
		uri := fmt.Sprintf("at://%s/%s", evt.Repo, op.Path)
		if err != nil {
			log.Error(ctx, "invalid path in repo op", "eventKind", op.Action, "path", op.Path)
			return
		}
		ctx = log.WithLogValues(ctx, "eventKind", op.Action, "collection", collection.String(), "rkey", rkey.String())

		if len(CollectionFilter) > 0 {
			keep := slices.Contains(CollectionFilter, collection.String())
			if strings.HasPrefix(collection.String(), "place.stream.") {
				keep = true
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
		opTime := aqt.Time()
		atsync.LastEvent = opTime

		r, err := atsync.Model.GetRepo(evt.Repo)
		if err != nil {
			log.Error(ctx, "failed to get repo", "err", err)
			continue
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

			err = atsync.handleCreateUpdate(ctx, evt.Repo, rkey, recCBOR, op.Cid.String(), collection, ek == repomgr.EvtKindUpdateRecord, false)
			if err != nil {
				log.Error(ctx, "failed to handle create update", "err", err)
				continue
			}

		case repomgr.EvtKindDeleteRecord:
			if collection.String() == constants.APP_BSKY_GRAPH_FOLLOW {
				if r == nil {
					log.Debug(ctx, "no repo found for follow", "userDID", evt.Repo, "subjectDID", rkey.String())
					continue
				}
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

			if collection.String() == constants.PLACE_STREAM_KEY {
				log.Warn(ctx, "revoking stream key", "userDID", evt.Repo, "rkey", rkey.String())
				key, err := atsync.Model.GetSigningKeyByRKey(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to get signing key", "err", err)
					continue
				}
				if key == nil {
					log.Warn(ctx, "no signing key found for stream key", "userDID", evt.Repo, "rkey", rkey.String())
					continue
				}
				now := time.Now()
				key.RevokedAt = &now
				err = atsync.Model.UpdateSigningKey(key)
				if err != nil {
					log.Error(ctx, "failed to revoke signing key", "err", err)
				}
				atsync.Bus.Publish(evt.Repo, key)
			}

			if collection.String() == constants.PLACE_STREAM_CHAT_MESSAGE {
				msg, err := atsync.Model.GetChatMessage(uri)
				if err != nil {
					log.Error(ctx, "failed to get chat message", "err", err)
					continue
				}
				if msg == nil {
					log.Warn(ctx, "no chat message found for uri", "uri", uri)
					continue
				}
				log.Warn(ctx, "deleting chat message", "userDID", evt.Repo, "uri", uri)
				err = atsync.Model.DeleteChatMessage(ctx, uri, &opTime)
				if err != nil {
					log.Error(ctx, "failed to delete chat message", "err", err)
					continue
				}
				mv, err := msg.ToStreamplaceMessageView()
				if err != nil {
					log.Error(ctx, "failed to convert chat message to streamplace message view", "err", err)
					continue
				}
				isTrue := true
				mv.Deleted = &isTrue
				atsync.Bus.Publish(msg.StreamerRepoDID, mv)
			}

			if collection.String() == constants.PLACE_STREAM_LIVE_TELEPORT {
				err := atsync.Model.DeleteTeleport(ctx, uri)
				if err != nil {
					log.Error(ctx, "failed to delete teleport", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MODERATION_PERMISSION {
				log.Debug(ctx, "deleting moderation delegation", "userDID", evt.Repo, "rkey", rkey.String())
				err := atsync.Model.DeleteModerationDelegation(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete moderation delegation", "err", err)
				}
				// Publish deletion to WebSocket bus for real-time updates
				// Create a deleted record marker to notify frontend
				deletedRecord := map[string]any{
					"$type":    "place.stream.moderation.permission",
					"deleted":  true,
					"rkey":     rkey.String(),
					"streamer": evt.Repo,
				}
				go atsync.Bus.Publish(evt.Repo, deletedRecord)
			}

			if collection.String() == constants.PLACE_STREAM_CHAT_GATE {
				log.Debug(ctx, "deleting gate", "userDID", evt.Repo, "rkey", rkey.String())
				err := atsync.Model.DeleteGate(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete gate", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_CHAT_PINNED_RECORD {
				log.Debug(ctx, "deleting pinned record", "userDID", evt.Repo, "rkey", rkey.String())
				err := atsync.Model.DeletePinnedRecord(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete pinned record", "err", err)
				}
				deletedPin := map[string]any{
					"$type":   constants.PLACE_STREAM_CHAT_PINNED_RECORD,
					"rkey":    rkey.String(),
					"deleted": true,
				}
				go atsync.Bus.Publish(evt.Repo, deletedPin)
			}

			if collection.String() == constants.PLACE_STREAM_BADGE_DEF {
				log.Debug(ctx, "deleting badge def", "uri", uri)
				err := atsync.Model.DeleteBadgeDef(ctx, uri)
				if err != nil {
					log.Error(ctx, "failed to delete badge def", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_BADGE_ISSUANCE {
				log.Debug(ctx, "deleting badge issuance", "uri", uri)
				err := atsync.Model.DeleteBadgeIssuance(ctx, uri)
				if err != nil {
					log.Error(ctx, "failed to delete badge issuance", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_VIDEO {
				log.Debug(ctx, "deleting video", "uri", uri)
				if err := atsync.Model.DeleteVideo(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete video", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MEDIA_TRACK {
				log.Debug(ctx, "deleting media track", "uri", uri)
				if err := atsync.Model.DeleteMediaTrack(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete media track", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MEDIA_ORIGIN {
				log.Debug(ctx, "deleting media origin", "uri", uri)
				if err := atsync.Model.DeleteMediaOrigin(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete media origin", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_BETA_INVITE {
				log.Debug(ctx, "deleting beta invite", "uri", uri)
				if err := atsync.Model.DeleteBetaInvite(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete beta invite", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_BETA_REQUEST {
				log.Debug(ctx, "deleting beta request", "uri", uri)
				if err := atsync.Model.DeleteBetaRequest(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete beta request", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MEDIA_VIEW_COUNT {
				log.Debug(ctx, "deleting media view count", "uri", uri)
				if err := atsync.Model.DeleteMediaViewCount(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete media view count", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_VOD_COMMENT {
				log.Debug(ctx, "deleting VOD comment", "uri", uri)
				if err := atsync.Model.DeleteVodComment(ctx, uri, &opTime); err != nil {
					log.Error(ctx, "failed to delete VOD comment", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_VOD_LIKE {
				log.Debug(ctx, "deleting VOD like", "uri", uri)
				if err := atsync.Model.DeleteVodLike(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete VOD like", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_VOD_GATE {
				log.Debug(ctx, "deleting VOD gate", "userDID", evt.Repo, "rkey", rkey.String())
				if err := atsync.Model.DeleteVodGate(ctx, rkey.String()); err != nil {
					log.Error(ctx, "failed to delete VOD gate", "err", err)
				}
			}

		default:
			log.Error(ctx, "unexpected record op kind")
		}
	}
}

func (atsync *ATProtoSynchronizer) handleIdentityEventOps(ctx context.Context, evt *comatproto.SyncSubscribeRepos_Identity) {
	handle := ""
	if evt.Handle != nil {
		handle = *evt.Handle
	}
	ctx = log.WithLogValues(ctx, "event", "identity", "did", evt.Did, "handle", handle, "func", "handleIdentityEventOps")
	r, err := atsync.Model.GetRepo(evt.Did)
	if err != nil {
		log.Error(ctx, "failed to get repo", "err", err)
		return
	}
	if r == nil {
		log.Debug(ctx, "no repo found for identity", "did", evt.Did)
		return
	}
	_, err = atsync.RefreshIdentity(ctx, evt.Did)
	if err != nil {
		log.Error(ctx, "failed to refresh ident", "err", err)
		return
	}
}
