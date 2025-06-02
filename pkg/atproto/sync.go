package atproto

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/integrations/discord"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/streamplace"

	lexutil "github.com/bluesky-social/indigo/lex/util"
)

func (atsync *ATProtoSynchronizer) handleCreateUpdate(ctx context.Context, userDID string, rkey syntax.RecordKey, recCBOR *[]byte, cid string, collection syntax.NSID, isUpdate bool, isFirstSync bool) error {
	ctx = log.WithLogValues(ctx, "func", "handleCreateUpdate", "userDID", userDID, "rkey", rkey.String(), "cid", cid, "collection", collection.String())
	now := time.Now()
	r, err := atsync.Model.GetRepo(userDID)
	if err != nil {
		return fmt.Errorf("failed to get repo: %w", err)
	}
	maybeATURI := fmt.Sprintf("at://%s/%s/%s", userDID, collection.String(), rkey.String())
	aturi, err := syntax.ParseATURI(maybeATURI)
	if err != nil {
		return fmt.Errorf("failed to parse ATURI: %w", err)
	}
	d, err := data.UnmarshalCBOR(*recCBOR)
	if err != nil {
		return fmt.Errorf("failed to unmarhsal record CBOR: %w", err)
	}
	cb, err := lexutil.CborDecodeValue(*recCBOR)
	if errors.Is(err, lexutil.ErrUnrecognizedType) {
		log.Debug(ctx, "unrecognized record type", "key", rkey.String(), "type", err)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to decode record CBOR: %w", err)
	}
	switch rec := cb.(type) {
	case *bsky.GraphFollow:
		if r == nil {
			// someone we don't know about
			return nil
		}
		log.Debug(ctx, "creating follow", "userDID", userDID, "subjectDID", rec.Subject)
		err := atsync.Model.CreateFollow(ctx, userDID, rkey.String(), rec)
		if err != nil {
			log.Debug(ctx, "failed to create follow", "err", err)
		}

	case *bsky.GraphBlock:
		if r == nil {
			// someone we don't know about
			return nil
		}
		log.Debug(ctx, "creating block", "userDID", userDID, "subjectDID", rec.Subject)
		block := &model.Block{
			RKey:       rkey.String(),
			RepoDID:    userDID,
			SubjectDID: rec.Subject,
			Record:     *recCBOR,
			CID:        cid,
		}
		err := atsync.Model.CreateBlock(ctx, block)
		if err != nil {
			return fmt.Errorf("failed to create block: %w", err)
		}
		block, err = atsync.Model.GetBlock(ctx, rkey.String())
		if err != nil {
			return fmt.Errorf("failed to get block after we just saved it?!: %w", err)
		}
		streamplaceBlock, err := block.ToStreamplaceBlock()
		if err != nil {
			return fmt.Errorf("failed to convert block to streamplace block: %w", err)
		}
		go atsync.Bus.Publish(userDID, streamplaceBlock)

	case *streamplace.ChatMessage:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		streamerRepo, err := atsync.SyncBlueskyRepoCached(ctx, rec.Streamer, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		log.Debug(ctx, "streamplace.ChatMessage detected", "message", rec.Text, "repo", repo.Handle)
		block, err := atsync.Model.GetUserBlock(ctx, streamerRepo.DID, userDID)
		if err != nil {
			return fmt.Errorf("failed to get user block: %w", err)
		}
		if block != nil {
			log.Debug(ctx, "excluding message from blocked user", "userDID", userDID, "subjectDID", streamerRepo.DID)
			return nil
		}
		mcm := &model.ChatMessage{
			CID:             cid,
			URI:             aturi.String(),
			CreatedAt:       now,
			ChatMessage:     recCBOR,
			RepoDID:         userDID,
			Repo:            repo,
			StreamerRepoDID: streamerRepo.DID,
			IndexedAt:       &now,
		}
		if rec.Reply != nil && rec.Reply.Parent != nil && rec.Reply.Root != nil {
			mcm.ReplyToCID = &rec.Reply.Parent.Cid
		}
		err = atsync.Model.CreateChatMessage(ctx, mcm)
		if err != nil {
			log.Error(ctx, "failed to create chat message", "err", err)
		}
		mcm, err = atsync.Model.GetChatMessage(cid)
		if err != nil {
			log.Error(ctx, "failed to get just-saved chat message", "err", err)
		}
		if mcm == nil {
			log.Error(ctx, "failed to retrieve just-saved chat message", "err", err)
			return nil
		}
		scm, err := mcm.ToStreamplaceMessageView()
		if err != nil {
			log.Error(ctx, "failed to convert chat message to streamplace message view", "err", err)
		}
		go atsync.Bus.Publish(streamerRepo.DID, scm)

		if !isUpdate && !isFirstSync {
			for _, webhook := range atsync.CLI.DiscordWebhooks {
				if webhook.DID == streamerRepo.DID && webhook.Type == "chat" {
					go func() {
						err := discord.SendChat(ctx, webhook, r, scm)
						if err != nil {
							log.Error(ctx, "failed to send livestream to discord", "err", err)
						} else {
							log.Log(ctx, "sent livestream to discord", "user", userDID, "webhook", webhook.URL)
						}
					}()
				}
			}
		}

	case *streamplace.ChatProfile:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		mcm := &model.ChatProfile{
			RepoDID: userDID,
			Repo:    repo,
			Record:  recCBOR,
		}
		err = atsync.Model.CreateChatProfile(ctx, mcm)
		if err != nil {
			log.Error(ctx, "failed to create chat profile", "err", err)
		}

	case *bsky.FeedPost:
		// jsonData, err := json.Marshal(d)
		// if err != nil {
		// 	log.Error(ctx, "failed to marshal record data", "err", err)
		// } else {
		// 	log.Log(ctx, "record data", "json", string(jsonData))
		// }

		createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to parse createdAt: %w", err)
		}

		if livestream, ok := d["place.stream.livestream"]; ok {
			repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
			if err != nil {
				return fmt.Errorf("failed to sync bluesky repo: %w", err)
			}
			livestream, ok := livestream.(map[string]interface{})
			if !ok {
				return fmt.Errorf("livestream is not a map")
			}
			url, ok := livestream["url"].(string)
			if !ok {
				return fmt.Errorf("livestream url is not a string")
			}
			log.Debug(ctx, "livestream url", "url", url)
			if err := atsync.Model.CreateFeedPost(ctx, &model.FeedPost{
				CID:       cid,
				CreatedAt: createdAt,
				FeedPost:  recCBOR,
				RepoDID:   userDID,
				Repo:      repo,
				Type:      "livestream",
				URI:       aturi.String(),
				IndexedAt: &now,
			}); err != nil {
				return fmt.Errorf("failed to create bluesky post: %w", err)
			}
		} else {
			if rec.Reply == nil || rec.Reply.Root == nil {
				return nil
			}
			livestream, err := atsync.Model.GetLivestreamByPostCID(rec.Reply.Root.Cid)
			if err != nil {
				return fmt.Errorf("failed to get livestream: %w", err)
			}
			if livestream == nil {
				return nil
			}
			// log.Warn(ctx, "chat message detected", "uri", livestream.URI)
			// if this post is a reply to someone's livestream post
			// log.Warn(ctx, "chat message detected", "message", rec.Text)
			repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
			if err != nil {
				return fmt.Errorf("failed to sync bluesky repo: %w", err)
			}

			// log.Warn(ctx, "chat message detected", "message", rec.Text, "repo", repo.Handle)
			block, err := atsync.Model.GetUserBlock(ctx, livestream.RepoDID, userDID)
			if err != nil {
				return fmt.Errorf("failed to get user block: %w", err)
			}
			if block != nil {
				log.Warn(ctx, "excluding message from blocked user", "userDID", userDID, "subjectDID", livestream.RepoDID)
				return nil
			}
			// if fc.cli.PrintChat {
			// 	fmt.Printf("@%s%s %s\n", blue.Sprintf(repo.Handle), green.Sprintf(":"), rec.Text)
			// }
			fp := &model.FeedPost{
				CID:              cid,
				CreatedAt:        createdAt,
				FeedPost:         recCBOR,
				RepoDID:          userDID,
				Type:             "reply",
				Repo:             repo,
				ReplyRootCID:     &livestream.PostCID,
				ReplyRootRepoDID: &livestream.RepoDID,
				URI:              aturi.String(),
				IndexedAt:        &now,
			}
			err = atsync.Model.CreateFeedPost(ctx, fp)
			if err != nil {
				log.Error(ctx, "failed to create feed post", "err", err)
			}
			postView, err := fp.ToBskyPostView()
			if err != nil {
				log.Error(ctx, "failed to convert feed post to bsky post view", "err", err)
			}
			go atsync.Bus.Publish(livestream.RepoDID, postView)
		}

	case *streamplace.Livestream:
		var u string
		if rec.Url != nil {
			u = *rec.Url
		}
		if r == nil {
			// we don't know about this repo
			return nil
		}
		createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
		if err != nil {
			log.Error(ctx, "failed to parse createdAt", "err", err)
			return nil
		}
		ls := &model.Livestream{
			CID:        cid,
			URI:        aturi.String(),
			CreatedAt:  createdAt,
			Livestream: recCBOR,
			RepoDID:    userDID,
		}
		if rec.Post != nil {
			ls.PostCID = rec.Post.Cid
			ls.PostURI = rec.Post.Uri
		}
		err = atsync.Model.CreateLivestream(ctx, ls)
		if err != nil {
			return fmt.Errorf("failed to create livestream: %w", err)
		}
		lsHydrated, err := atsync.Model.GetLatestLivestreamForRepo(userDID)
		if err != nil {
			return fmt.Errorf("failed to get latest livestream for repo: %w", err)
		}
		lsv, err := lsHydrated.ToLivestreamView()
		if err != nil {
			return fmt.Errorf("failed to convert livestream to bsky post view: %w", err)
		}
		go atsync.Bus.Publish(userDID, lsv)

		if !isUpdate && !isFirstSync {
			log.Warn(ctx, "Livestream detected! Blasting followers!", "title", rec.Title, "url", u, "createdAt", rec.CreatedAt, "repo", userDID)
			notifications, err := atsync.Model.GetFollowersNotificationTokens(userDID)
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
			if atsync.Noter != nil {
				err := atsync.Noter.Blast(ctx, notifications, nb)
				if err != nil {
					log.Error(ctx, "failed to blast notifications", "err", err)
				} else {
					log.Log(ctx, "sent notifications", "user", userDID, "count", len(notifications), "content", nb)
				}
			} else {
				log.Log(ctx, "no notifier configured, skipping notifications", "user", userDID, "count", len(notifications), "content", nb)
			}

			var postView *bsky.FeedDefs_PostView
			if lsHydrated.Post != nil {
				postView, err = lsHydrated.Post.ToBskyPostView()
				if err != nil {
					log.Error(ctx, "failed to convert livestream post to bsky post view", "err", err)
				}
			} else {
				log.Warn(ctx, "no post found for livestream", "livestream", lsHydrated)
			}

			var spcp *streamplace.ChatProfile
			cp, err := atsync.Model.GetChatProfile(ctx, userDID)
			if err != nil {
				log.Error(ctx, "failed to get chat profile", "err", err)
			}
			if cp != nil {
				spcp, err = cp.ToStreamplaceChatProfile()
				if err != nil {
					log.Error(ctx, "failed to convert chat profile to streamplace chat profile", "err", err)
				}
			}

			for _, webhook := range atsync.CLI.DiscordWebhooks {
				if webhook.DID == userDID && webhook.Type == "livestream" {
					go func() {
						err := discord.SendLivestream(ctx, webhook, r, lsv, postView, spcp)
						if err != nil {
							log.Error(ctx, "failed to send livestream to discord", "err", err)
						} else {
							log.Log(ctx, "sent livestream to discord", "user", userDID, "webhook", webhook.URL)
						}
					}()
				}
			}
		}

	case *streamplace.Key:
		log.Debug(ctx, "creating key", "key", rec)
		time, err := aqtime.FromString(rec.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to parse createdAt: %w", err)
		}
		key := model.SigningKey{
			DID:       rec.SigningKey,
			CreatedAt: time.Time(),
			RepoDID:   userDID,
		}
		err = atsync.Model.UpdateSigningKey(&key)
		if err != nil {
			log.Error(ctx, "failed to create signing key", "err", err)
		}

	default:
		log.Debug(ctx, "unhandled record type", "type", reflect.TypeOf(rec))
	}
	return nil
}
