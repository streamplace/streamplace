package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/integrations/webhook"
	"stream.place/streamplace/pkg/log"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/streamplace"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
)

var TaskNotification = "notification"
var TaskChat = "chat"
var TaskFinalizeLivestream = "finalize_livestream"
var TaskVODProcess = "vod_process"
var TaskViewCountAggregate = "view_count_aggregate"

type NotificationTask struct {
	Livestream  *streamplace.Livestream_LivestreamView
	FeedPost    *bsky.FeedDefs_PostView
	ChatProfile *streamplace.ChatProfile
	PDSURL      string
}

type ChatTask struct {
	MessageView *streamplace.ChatDefs_MessageView
}

type FinalizeLivestreamTask struct {
	LivestreamURI string `json:"livestreamURI"`
}

// VODProcessTask is enqueued by the upload manager when a resumable user
// upload completes. The processor probes the file, generates MUXL tracks,
// and creates the place.stream.video record set. The actual processing is
// not yet implemented — for now this just acknowledges receipt.
type VODProcessTask struct {
	UploadID string `json:"uploadId"`
	RepoDID  string `json:"repoDID"`
	MimeType string `json:"mimeType"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size"`
	Backend  string `json:"backend"`
	Location string `json:"location"`
}

// ViewCountAggregateTask is the payload for one aggregation window.
// Enqueued by every streamplace node at the configured interval; the
// unique task key (built from WindowStart/End) ensures only one node's
// enqueue + dequeue actually runs each window.
type ViewCountAggregateTask struct {
	WindowStart time.Time `json:"windowStart"`
	WindowEnd   time.Time `json:"windowEnd"`
}

func (state *StatefulDB) ProcessQueue(ctx context.Context) error {
	for {
		task, err := state.DequeueTask(ctx, "queue_processor")
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if task != nil {
			err := state.processTask(ctx, task)
			if err != nil {
				log.Error(ctx, "failed to process task", "err", err)
			}
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				continue
			case <-state.pokeQueue:
				continue
			}
		}

	}
}

func (state *StatefulDB) processTask(ctx context.Context, task *AppTask) error {
	switch task.Type {
	case TaskNotification:
		return state.processNotificationTask(ctx, task)
	case TaskChat:
		return state.processChatMessageTask(ctx, task)
	case TaskFinalizeLivestream:
		return state.processFinalizeLivestreamTask(ctx, task)
	case TaskVODProcess:
		return state.processVODProcessTask(ctx, task)
	case TaskViewCountAggregate:
		return state.processViewCountAggregateTask(ctx, task)
	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}
}

// VODProcessor runs the gstreamer + muxl + S3 pipeline for one upload
// and returns the resulting BDASL CID. The function-pointer indirection
// keeps pkg/statedb from importing pkg/vod (which transitively pulls in
// gstreamer); the bootstrap (pkg/cmd) installs the concrete
// implementation at startup.
type VODProcessor func(ctx context.Context, t VODProcessTask) (cid string, err error)

func (state *StatefulDB) SetVODProcessor(f VODProcessor) { state.vodProcessor = f }

func (state *StatefulDB) processVODProcessTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processVODProcessTask")
	var t VODProcessTask
	if err := json.Unmarshal(task.Payload, &t); err != nil {
		return err
	}
	if state.vodProcessor == nil {
		log.Warn(ctx, "no VOD processor configured; dropping task",
			"uploadId", t.UploadID, "did", t.RepoDID)
		return state.CompleteTask(ctx, task.ID)
	}
	cid, err := state.vodProcessor(ctx, t)
	if err != nil {
		return fmt.Errorf("vod processing: %w", err)
	}
	log.Log(ctx, "vod processed", "uploadId", t.UploadID, "cid", cid)
	return state.CompleteTask(ctx, task.ID)
}

// ViewCountAggregator runs the view-log → place.stream.media.viewCount
// aggregation for one window. Same function-pointer indirection trick
// as VODProcessor: pkg/statedb stays ignorant of pkg/viewlog (which
// pulls in blob storage + atproto publishing).
type ViewCountAggregator func(ctx context.Context, t ViewCountAggregateTask) error

func (state *StatefulDB) SetViewCountAggregator(f ViewCountAggregator) {
	state.viewCountAggregator = f
}

func (state *StatefulDB) processViewCountAggregateTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processViewCountAggregateTask")
	var t ViewCountAggregateTask
	if err := json.Unmarshal(task.Payload, &t); err != nil {
		return err
	}
	if state.viewCountAggregator == nil {
		log.Warn(ctx, "no view-count aggregator configured; dropping task",
			"windowStart", t.WindowStart, "windowEnd", t.WindowEnd)
		return state.CompleteTask(ctx, task.ID)
	}
	if err := state.viewCountAggregator(ctx, t); err != nil {
		return fmt.Errorf("view-count aggregation: %w", err)
	}
	return state.CompleteTask(ctx, task.ID)
}

func (state *StatefulDB) processFinalizeLivestreamTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processFinalizeLivestreamTask")
	log.Debug(ctx, "processing finalize livestream task")
	var finalizeLivestreamTask FinalizeLivestreamTask
	if err := json.Unmarshal(task.Payload, &finalizeLivestreamTask); err != nil {
		return err
	}
	livestream, err := state.model.GetLivestream(finalizeLivestreamTask.LivestreamURI)
	if err != nil {
		return fmt.Errorf("failed to get latest livestream for userDID: %w", err)
	}
	if livestream == nil {
		return fmt.Errorf("no livestream found for URI: %s", finalizeLivestreamTask.LivestreamURI)
	}
	lastLivestreamView, err := livestream.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("failed to convert livestream to streamplace livestream: %w", err)
	}
	rec, ok := lastLivestreamView.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}
	if rec.LastSeenAt == nil {
		return fmt.Errorf("livestream has no last seen at")
	}
	lastSeenTime, err := time.Parse(time.RFC3339, *rec.LastSeenAt)
	if err != nil {
		return fmt.Errorf("could not parse last seen at: %w", err)
	}
	if rec.IdleTimeoutSeconds == nil || *rec.IdleTimeoutSeconds == 0 {
		log.Debug(ctx, "livestream has no idle timeout, skipping finalization", "uri", livestream.URI)
		return nil
	}
	if time.Since(lastSeenTime) < (time.Duration(*rec.IdleTimeoutSeconds) * time.Second) {
		log.Debug(ctx, "livestream is active, skipping finalization", "lastSeenAt", lastSeenTime)
		return nil
	}
	session, err := state.GetSessionByDID(livestream.RepoDID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	session, err = state.OATProxy.RefreshIfNeeded(session)
	if err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}
	client, err := state.OATProxy.GetXrpcClient(session)
	if err != nil {
		return fmt.Errorf("failed to get xrpc client: %w", err)
	}
	if rec.EndedAt != nil {
		log.Debug(ctx, "livestream has already ended, skipping", "uri", livestream.URI, "endedAt", *rec.EndedAt)
		return nil
	}

	uri, err := syntax.ParseATURI(livestream.URI)
	if err != nil {
		return fmt.Errorf("failed to parse ATURI: %w", err)
	}

	rec.EndedAt = rec.LastSeenAt

	inp := comatproto.RepoPutRecord_Input{
		Collection: "place.stream.livestream",
		Record:     &lexutil.LexiconTypeDecoder{Val: rec},
		Rkey:       uri.RecordKey().String(),
		Repo:       livestream.RepoDID,
		SwapRecord: &livestream.CID,
	}
	out := comatproto.RepoPutRecord_Output{}

	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out)
	if err != nil {
		return fmt.Errorf("failed to update livestream record: %w", err)
	}

	log.Log(ctx, "livestream finalized", "uri", livestream.URI, "endedAt", *rec.EndedAt)

	return nil
}

func (state *StatefulDB) processNotificationTask(ctx context.Context, task *AppTask) error {
	var notificationTask NotificationTask
	if err := json.Unmarshal(task.Payload, &notificationTask); err != nil {
		return err
	}
	lsv := notificationTask.Livestream
	rec, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("invalid livestream record")
	}
	userDID := lsv.Author.Did

	log.Warn(ctx, "Livestream detected! Blasting followers!", "title", rec.Title, "url", rec.Url, "createdAt", rec.CreatedAt, "repo", userDID)
	followers, err := state.model.GetUserFollowers(ctx, userDID)
	if err != nil {
		return err
	}

	followersDIDs := make([]string, 0, len(followers))
	for _, follower := range followers {
		followersDIDs = append(followersDIDs, follower.UserDID)
	}

	log.Log(ctx, "found followers", "count", len(followersDIDs))

	notifications, err := state.GetManyNotificationTokens(followersDIDs)
	if err != nil {
		return err
	}

	if state.noter != nil {
		nb := &notificationpkg.NotificationBlast{
			Title: fmt.Sprintf("🔴 @%s is LIVE!", lsv.Author.Handle),
			Body:  rec.Title,
			Data: map[string]string{
				"path": fmt.Sprintf("/%s", lsv.Author.Handle),
			},
		}
		err = state.noter.Blast(ctx, notifications, nb)
		if err != nil {
			log.Error(ctx, "failed to blast notifications", "err", err)
		} else {
			log.Log(ctx, "sent notifications", "user", userDID, "count", len(notifications), "content", nb)
		}
	} else {
		log.Log(ctx, "no notifier configured, skipping notifications", "user", userDID, "count", len(notifications))
	}

	// Send to webhooks using webhook manager
	webhooks, err := state.GetActiveWebhooksForUser(userDID, "livestream")
	if err != nil {
		log.Error(ctx, "failed to get livestream webhooks", "err", err)
	} else {
		for _, w := range webhooks {
			lexiconWebhook, err := w.ToLexicon()
			if err != nil {
				log.Error(ctx, "failed to convert webhook to lexicon", "err", err, "webhook_id", w.ID)
				continue
			}
			go func(lexiconWebhook *streamplace.ServerDefs_Webhook, wid string) {
				err := webhook.SendLivestreamWebhook(ctx, lexiconWebhook, notificationTask.PDSURL, lsv, notificationTask.FeedPost, notificationTask.ChatProfile)
				if err != nil {
					log.Error(ctx, "failed to send livestream to webhook", "err", err, "webhook_id", wid)
					err := state.IncrementWebhookError(wid)
					if err != nil {
						log.Error(ctx, "failed to increment webhook error count", "err", err, "webhook_id", wid)
					}
				} else {
					log.Log(ctx, "sent livestream to webhook", "webhook_id", wid)
					err := state.ResetWebhookError(wid)
					if err != nil {
						log.Error(ctx, "failed to reset webhook error count", "err", err, "webhook_id", wid)
					}
				}
			}(lexiconWebhook, w.ID)
		}
	}
	return nil
}

func (state *StatefulDB) processChatMessageTask(ctx context.Context, task *AppTask) error {
	var chatTask ChatTask
	if err := json.Unmarshal(task.Payload, &chatTask); err != nil {
		return err
	}
	scm := chatTask.MessageView
	rec, ok := scm.Record.Val.(*streamplace.ChatMessage)
	if !ok {
		return fmt.Errorf("invalid chat message record")
	}

	// Send to webhooks using webhook manager
	webhooks, err := state.GetActiveWebhooksForUser(rec.Streamer, "chat")
	if err != nil {
		log.Error(ctx, "failed to get chat webhooks", "err", err)
	} else {
		for _, w := range webhooks {
			lexiconWebhook, err := w.ToLexicon()
			if err != nil {
				log.Error(ctx, "failed to convert webhook to lexicon", "err", err, "webhook_id", w.ID)
				continue
			}
			go func(lexiconWebhook *streamplace.ServerDefs_Webhook, wid string) {
				err := webhook.SendChatWebhook(ctx, lexiconWebhook, scm.Author.Did, scm)
				if err != nil {
					log.Error(ctx, "failed to send chat to webhook", "err", err, "webhook_id", wid)
					err = state.IncrementWebhookError(wid)
					if err != nil {
						log.Error(ctx, "failed to increment webhook error count", "err", err, "webhook_id", wid)
					}
				} else {
					log.Log(ctx, "sent chat to webhook", "webhook_id", wid)
					err = state.ResetWebhookError(wid)
					if err != nil {
						log.Error(ctx, "failed to reset webhook error count", "err", err, "webhook_id", wid)
					}
				}
			}(lexiconWebhook, w.ID)
		}
	}
	return nil
}
