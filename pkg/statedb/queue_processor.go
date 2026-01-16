package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/integrations/webhook"
	"stream.place/streamplace/pkg/log"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/redcircle"
	"stream.place/streamplace/pkg/streamplace"
)

var TaskNotification = "notification"
var TaskChat = "chat"
var TaskAddRedCircle = "add_red_circle"
var TaskRemoveRedCircle = "remove_red_circle"

type NotificationTask struct {
	Livestream  *streamplace.Livestream_LivestreamView
	FeedPost    *bsky.FeedDefs_PostView
	ChatProfile *streamplace.ChatProfile
	PDSURL      string
}

type ChatTask struct {
	MessageView *streamplace.ChatDefs_MessageView
}

type AddRedCircleTask struct {
	UserDID string
}

type RemoveRedCircleTask struct {
	UserDID string
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
	case TaskAddRedCircle:
		return state.processAddRedCircleTask(ctx, task)
	case TaskRemoveRedCircle:
		return state.processRemoveRedCircleTask(ctx, task)
	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}
}

func (state *StatefulDB) processAddRedCircleTask(ctx context.Context, task *AppTask) error {
	var addRedCircleTask AddRedCircleTask
	if err := json.Unmarshal(task.Payload, &addRedCircleTask); err != nil {
		return err
	}
	repoDID := addRedCircleTask.UserDID
	session, err := state.GetSessionByDID(repoDID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("no session found for repoDID: %s", repoDID)
	}
	session, err = state.OATProxy.RefreshIfNeeded(session)
	if err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}
	client, err := state.OATProxy.GetXrpcClient(session)
	if err != nil {
		return fmt.Errorf("failed to get xrpc client: %w", err)
	}
	err = redcircle.UpdateProfilePicture(ctx, repoDID, client, state.model)
	if err != nil {
		return fmt.Errorf("failed to update profile picture: %w", err)
	}
	return nil
}

func (state *StatefulDB) processRemoveRedCircleTask(ctx context.Context, task *AppTask) error {
	var removeRedCircleTask RemoveRedCircleTask
	if err := json.Unmarshal(task.Payload, &removeRedCircleTask); err != nil {
		return err
	}
	userDID := removeRedCircleTask.UserDID
	lastLivestream, err := state.model.GetLatestLivestreamForRepo(userDID)
	if err != nil {
		return fmt.Errorf("failed to get latest livestream for userDID: %w", err)
	}
	if lastLivestream == nil {
		return fmt.Errorf("no livestream found for userDID: %s", userDID)
	}
	lastLivestreamView, err := lastLivestream.ToLivestreamView()
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
	if time.Since(lastSeenTime) < 60*time.Second {
		log.Warn(ctx, "livestream is active, skipping removal of red circle", "lastSeenAt", lastSeenTime)
		return nil
	}
	log.Warn(ctx, "removing red circle", "userDID", userDID, "lastSeenAt", lastSeenTime)
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
