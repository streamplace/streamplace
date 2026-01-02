package atproto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
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
	d, err := atdata.UnmarshalCBOR(*recCBOR)
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

		go func() {
			_, err = atsync.SyncBlueskyRepoCached(ctx, rec.Streamer, atsync.Model)
			if err != nil {
				log.Error(ctx, "failed to sync bluesky repo", "err", err)
			}
		}()

		log.Debug(ctx, "streamplace.ChatMessage detected", "message", rec.Text, "repo", repo.Handle)
		block, err := atsync.Model.GetUserBlock(ctx, rec.Streamer, userDID)
		if err != nil {
			return fmt.Errorf("failed to get user block: %w", err)
		}
		if block != nil {
			log.Debug(ctx, "excluding message from blocked user", "userDID", userDID, "subjectDID", rec.Streamer)
			return nil
		}
		mcm := &model.ChatMessage{
			CID:             cid,
			URI:             aturi.String(),
			CreatedAt:       now,
			ChatMessage:     recCBOR,
			RepoDID:         userDID,
			Repo:            repo,
			StreamerRepoDID: rec.Streamer,
			IndexedAt:       &now,
		}
		if rec.Reply != nil && rec.Reply.Parent != nil && rec.Reply.Root != nil {
			mcm.ReplyToCID = &rec.Reply.Parent.Cid
		}
		err = atsync.Model.CreateChatMessage(ctx, mcm)
		if err != nil {
			log.Error(ctx, "failed to create chat message", "err", err)
			return nil
		}
		mcm, err = atsync.Model.GetChatMessage(aturi.String())
		if err != nil {
			log.Error(ctx, "failed to get just-saved chat message", "err", err)
			return nil
		}
		if mcm == nil {
			log.Error(ctx, "failed to retrieve just-saved chat message", "err", err)
			return nil
		}
		scm, err := mcm.ToStreamplaceMessageView()
		if err != nil {
			log.Error(ctx, "failed to convert chat message to streamplace message view", "err", err)
			return nil
		}
		go atsync.Bus.Publish(rec.Streamer, scm)

		if !isUpdate && !isFirstSync {

			task := &statedb.ChatTask{
				MessageView: scm,
			}

			_, err = atsync.StatefulDB.EnqueueTask(ctx, statedb.TaskChat, task, statedb.WithTaskKey(fmt.Sprintf("chat-message::%s", aturi.String())))
			if err != nil {
				log.Error(ctx, "failed to enqueue notification task", "err", err)
			}
		}

	case *streamplace.ChatGate:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		if r == nil {
			// someone we don't know about
			return nil
		}
		log.Debug(ctx, "creating gate", "userDID", userDID, "hiddenMessage", rec.HiddenMessage)
		gate := &model.Gate{
			RKey:          rkey.String(),
			RepoDID:       userDID,
			HiddenMessage: rec.HiddenMessage,
			CID:           cid,
			CreatedAt:     now,
			Repo:          repo,
		}
		err = atsync.Model.CreateGate(ctx, gate)
		if err != nil {
			return fmt.Errorf("failed to create gate: %w", err)
		}
		gate, err = atsync.Model.GetGate(ctx, rkey.String())
		if err != nil {
			return fmt.Errorf("failed to get gate after we just saved it?!: %w", err)
		}
		streamplaceGate, err := gate.ToStreamplaceGate()
		if err != nil {
			return fmt.Errorf("failed to convert gate to streamplace gate: %w", err)
		}
		go atsync.Bus.Publish(userDID, streamplaceGate)

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

	case *streamplace.ServerSettings:
		_, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		settings := &model.ServerSettings{
			Server:  rkey.String(),
			RepoDID: userDID,
			Record:  recCBOR,
		}
		err = atsync.Model.UpdateServerSettings(ctx, settings)
		if err != nil {
			log.Error(ctx, "failed to create server settings", "err", err)
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
			livestream, err := atsync.Model.GetLivestreamByPostURI(rec.Reply.Root.Uri)
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
				ReplyRootURI:     &livestream.PostURI,
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

		var postView *bsky.FeedDefs_PostView
		if lsHydrated.Post != nil {
			postView, err = lsHydrated.Post.ToBskyPostView()
			if err != nil {
				return fmt.Errorf("failed to convert livestream post to bsky post view: %w", err)
			}
		}

		task := &statedb.NotificationTask{
			Livestream: lsv,
			FeedPost:   postView,
			PDSURL:     r.PDS,
		}

		cp, err := atsync.Model.GetChatProfile(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to get chat profile: %w", err)
		}
		if cp != nil {
			spcp, err := cp.ToStreamplaceChatProfile()
			if err != nil {
				return fmt.Errorf("failed to convert chat profile to streamplace chat profile: %w", err)
			}
			task.ChatProfile = spcp
		}

	case *streamplace.Key:
		log.Debug(ctx, "creating key", "key", rec)
		time, err := aqtime.FromString(rec.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to parse createdAt: %w", err)
		}
		key := model.SigningKey{
			DID:       rec.SigningKey,
			RKey:      rkey.String(),
			CreatedAt: time.Time(),
			RepoDID:   userDID,
		}
		err = atsync.Model.UpdateSigningKey(&key)
		if err != nil {
			log.Error(ctx, "failed to create signing key", "err", err)
		}

	case *streamplace.BroadcastOrigin:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync broadcast origin creator bluesky repo: %w", err)
		}
		_, err = atsync.SyncBlueskyRepoCached(ctx, rec.Streamer, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync broadcast origin streamer bluesky repo: %w", err)
		}
		err = atsync.Model.UpdateBroadcastOrigin(ctx, rec, aturi)
		if err != nil {
			log.Error(ctx, "failed to update broadcast origin", "err", err)
		}
		view := &streamplace.BroadcastDefs_BroadcastOriginView{
			Uri: aturi.String(),
			Cid: cid,
			Author: &bsky.ActorDefs_ProfileViewBasic{
				Did:    userDID,
				Handle: repo.Handle,
			},
			Record: &lexutil.LexiconTypeDecoder{Val: rec},
		}
		// publishes with an empty string because we're discovering the stream
		go atsync.Bus.Publish("", view)

	case *streamplace.MetadataConfiguration:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID, atsync.Model)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		log.Debug(ctx, "creating metadata configuration", "metadata", rec)
		metadata := &model.MetadataConfiguration{
			RepoDID: userDID,
			Record:  recCBOR,
			Repo:    repo,
		}
		err = atsync.Model.CreateMetadataConfiguration(ctx, metadata)
		if err != nil {
			log.Error(ctx, "failed to create metadata configuration", "err", err)
		}

	case *streamplace.LiveRecommendations:
		log.Debug(ctx, "creating recommendations", "userDID", userDID, "count", len(rec.Streamers))

		// Validate max 8 streamers
		if len(rec.Streamers) > 8 {
			log.Warn(ctx, "recommendations exceed maximum of 8", "count", len(rec.Streamers))
			return fmt.Errorf("maximum 8 recommendations allowed, got %d", len(rec.Streamers))
		}

		// Marshal streamers to JSON
		streamersJSON, err := json.Marshal(rec.Streamers)
		if err != nil {
			return fmt.Errorf("failed to marshal streamers: %w", err)
		}

		// Parse createdAt timestamp
		createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to parse createdAt: %w", err)
		}

		recommendation := &model.Recommendation{
			UserDID:   userDID,
			Streamers: json.RawMessage(streamersJSON),
			CreatedAt: createdAt,
		}

		err = atsync.Model.UpsertRecommendation(recommendation)
		if err != nil {
			return fmt.Errorf("failed to upsert recommendation: %w", err)
		}

	default:
		log.Debug(ctx, "unhandled record type", "type", reflect.TypeOf(rec))
	}
	return nil
}
