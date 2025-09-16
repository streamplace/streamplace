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
	"stream.place/streamplace/pkg/streamplace"
)

var TaskNotification = "notification"
var TaskChat = "chat"

type NotificationTask struct {
	Livestream  *streamplace.Livestream_LivestreamView
	FeedPost    *bsky.FeedDefs_PostView
	ChatProfile *streamplace.ChatProfile
	PDSURL      string
}

type ChatTask struct {
	MessageView *streamplace.ChatDefs_MessageView
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
	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}
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
