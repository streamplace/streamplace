package atproto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"stream.place/streamplace/pkg/appbsky"
	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/placestream"

	glexrt "github.com/streamplace/glex/runtime"
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
	cb, err := glexrt.CborDecodeValue(*recCBOR)
	if errors.Is(err, glexrt.ErrUnrecognizedType) {
		log.Debug(ctx, "unrecognized record type", "key", rkey.String(), "type", err)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to decode record CBOR: %w", err)
	}
	switch rec := cb.(type) {
	case *appbsky.GraphFollow:
		if r == nil {
			// someone we don't know about
			return nil
		}
		log.Debug(ctx, "creating follow", "userDID", userDID, "subjectDID", rec.Subject)
		err := atsync.Model.CreateFollow(ctx, userDID, rkey.String(), *rec)
		if err != nil {
			log.Debug(ctx, "failed to create follow", "err", err)
		}

	case *appbsky.GraphBlock:
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
		if err != nil || block == nil {
			return fmt.Errorf("failed to get block after we just saved it?!: %w", err)
		}
		streamplaceBlock, err := block.ToStreamplaceBlock()
		if err != nil {
			return fmt.Errorf("failed to convert block to streamplace block: %w", err)
		}
		go atsync.Bus.Publish(userDID, streamplaceBlock)

	case *appbsky.ActorProfile:
		if r == nil {
			// someone we don't know about
			return nil
		}
		wasStreamplace, _ := d[constants.BlueskyProfileGoliveKey].(bool)
		err := atsync.Model.UpsertBskyProfile(ctx, aturi, *recCBOR, wasStreamplace)
		if err != nil {
			return fmt.Errorf("failed to upsert bsky profile: %w", err)
		}

	case *placestream.ChatMessage:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}

		go func() {
			_, err = atsync.SyncBlueskyRepoCached(ctx, rec.Streamer)
			if err != nil {
				log.Error(ctx, "failed to sync bluesky repo", "err", err)
			}
		}()

		log.Debug(ctx, "placestream.ChatMessage detected", "message", rec.Text, "repo", repo.Handle)
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
		if rec.Reply != nil && rec.Reply.Parent.Uri != "" && rec.Reply.Root.Uri != "" {
			mcm.ReplyToCID = &rec.Reply.Parent.Cid
		}

		// check if we have any link facets with 'javascript:' links
		for _, facet := range rec.Facets {
			for _, feature := range facet.Features {
				if link := feature.RichtextFacet_Link; link != nil {
					if link.Uri != "" && strings.HasPrefix(strings.ToLower(link.Uri), "javascript:") {
						log.Warn(ctx, "excluding message with javascript: link", "uri", aturi.String(), "link", link.Uri)
						return nil
					}
				}
			}
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

		// Add mod badge if the author is a moderator
		issuerDID := fmt.Sprintf("did:web:%s", atsync.CLI.BroadcasterHost)
		err = AddModBadgeIfApplicable(ctx, &scm, rec.Streamer, issuerDID, atsync.Model)
		if err != nil {
			log.Error(ctx, "failed to add mod badge", "err", err)
		}

		if scm.Author.Handle == "" || scm.Author.Handle == "handle.invalid" {
			scm.Author.Handle = atsync.ResolveAuthorHandle(ctx, scm.Author.Did)
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

	case *placestream.ChatGate:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
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

	case *placestream.ChatPinnedRecord:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		if r == nil {
			return nil
		}
		log.Debug(ctx, "creating pinned record", "userDID", userDID, "pinnedMessage", rec.PinnedMessage)
		// err = atsync.Model.DeleteAllPinnedRecords(ctx, userDID)
		// if err != nil {
		// 	log.Error(ctx, "failed to delete existing pinned records", "err", err)
		// }
		// Parse optional expiresAt
		var expiresAt *time.Time
		if rec.ExpiresAt != nil {
			t, err := time.Parse(time.RFC3339, *rec.ExpiresAt)
			if err == nil {
				expiresAt = &t
			}
		}
		// serialise createdAt
		createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to parse createdAt: %w", err)
		}

		var pinnedBy string
		if rec.PinnedBy == nil {
			pinnedBy = userDID
		} else {
			pinnedBy = *rec.PinnedBy
		}

		pin := &model.PinnedRecord{
			Uri:           aturi.String(),
			RepoDID:       userDID,
			PinnedMessage: rec.PinnedMessage,
			PinnedBy:      pinnedBy,
			IndexedAt:     &now,
			CID:           cid,
			CreatedAt:     createdAt,
			Repo:          repo,
			ExpiresAt:     expiresAt,
		}
		err = atsync.Model.CreatePinnedRecord(ctx, pin)
		if err != nil {
			return fmt.Errorf("failed to create pinned record: %w", err)
		}
		pin, err = atsync.Model.GetPinnedRecord(ctx, pin.Uri)
		if err != nil {
			return fmt.Errorf("failed to get pinned record after we just saved it: %w", err)
		}
		pinnedView, err := pin.ToStreamplacePinnedRecordView()
		if err != nil {
			return fmt.Errorf("failed to convert pinned record: %w", err)
		}
		// look up the original message, pinner
		msg, err := atsync.Model.GetChatMessage(pinnedView.Record.PinnedMessage)
		if err != nil {
			return fmt.Errorf("failed to get chat message: %w", err)
		}
		profile, err := atsync.Model.GetChatProfile(ctx, pinnedBy)
		if err != nil {
			return fmt.Errorf("failed to get chat profile: %w", err)
		}
		if msg != nil {
			msgView, err := msg.ToStreamplaceMessageView()
			if err != nil {
				return fmt.Errorf("failed to convert chat message: %w", err)
			}
			pinnedView.Message = &msgView
		}
		if profile != nil {
			profileView, err := profile.ToStreamplaceChatProfile()
			if err != nil {
				return fmt.Errorf("failed to convert chat profile: %w", err)
			}
			pinnedView.PinnedBy = &profileView
		}
		go atsync.Bus.Publish(userDID, pinnedView)

	case *placestream.ChatProfile:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
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

	case *placestream.ServerSettings:
		_, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
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

	case *appbsky.FeedPost:
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
			repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
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
			if rec.Reply == nil || rec.Reply.Root.Uri == "" {
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
			repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
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

	case *placestream.Livestream:
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

		if !isFirstSync {
			// queue a task to clean up the livestream if it's been inactive for too long
			task := &statedb.FinalizeLivestreamTask{
				LivestreamURI: aturi.String(),
			}
			if rec.LastSeenAt == nil || rec.IdleTimeoutSeconds == nil || *rec.IdleTimeoutSeconds == 0 || rec.EndedAt != nil {
				return nil
			}
			scheduledAt, err := time.Parse(time.RFC3339, *rec.LastSeenAt)
			if err != nil {
				log.Error(ctx, "failed to parse last seen at", "err", err)
				return nil
			}

			// if we check after exactly rec.IdleTimeoutSeconds we might miss the finalization by a few seconds
			scheduledAt = scheduledAt.Add((time.Duration(*rec.IdleTimeoutSeconds) * time.Second) + (10 * time.Second)).UTC()
			taskKey := fmt.Sprintf("finalize-livestream::%s::%s", aturi.String(), scheduledAt.Format(util.ISO8601))
			_, err = atsync.StatefulDB.EnqueueTask(ctx, statedb.TaskFinalizeLivestream, task, statedb.WithTaskKey(taskKey), statedb.WithScheduledAt(scheduledAt))
			if err != nil {
				return fmt.Errorf("failed to enqueue remove red circle task: %w", err)
			}

		}

	case *placestream.LiveTeleport:
		if r == nil {
			return nil
		}
		startsAt, err := time.Parse(time.RFC3339, rec.StartsAt)
		if err != nil {
			log.Error(ctx, "failed to parse startsAt", "err", err)
			return nil
		}
		viewerCount := atsync.Bus.GetViewerCount(userDID)
		tp := &model.Teleport{
			CID:             cid,
			URI:             aturi.String(),
			StartsAt:        startsAt,
			DurationSeconds: rec.DurationSeconds,
			ViewerCount:     int64(viewerCount),
			Teleport:        recCBOR,
			RepoDID:         userDID,
			TargetDID:       rec.Streamer,
		}
		err = atsync.Model.CreateTeleport(ctx, tp)
		if err != nil {
			return fmt.Errorf("failed to create teleport: %w", err)
		}
		go atsync.Bus.Publish(userDID, rec)

		// schedule arrival notification 10 seconds after startsAt
		arrivalTime := startsAt.Add(10 * time.Second)
		waitDuration := time.Until(arrivalTime)
		if waitDuration < 0 {
			waitDuration = 0
		}

		time.AfterFunc(waitDuration, func() {
			// verify teleport still exists
			existingTp, err := atsync.Model.GetTeleportByURI(aturi.String())
			if err != nil {
				log.Error(ctx, "failed to get teleport by uri", "err", err)
				return
			}
			if existingTp == nil || existingTp.Denied {
				log.Debug(ctx, "teleport no longer active, skipping arrival notification", "uri", aturi.String())
				return
			}

			// get the source profile
			sourceRepo, err := atsync.Model.GetRepo(userDID)
			if err != nil {
				log.Error(ctx, "failed to get source repo", "err", err)
				return
			}

			viewerCount := existingTp.ViewerCount

			arrivalMsg := placestream.Livestream_TeleportArrival{
				LexiconTypeID: "place.stream.livestream#teleportArrival",
				TeleportUri:   aturi.String(),
				Source: appbsky.ActorDefs_ProfileViewBasic{
					Did:    userDID,
					Handle: sourceRepo.Handle,
				},
				ViewerCount: int64(viewerCount),
				StartsAt:    rec.StartsAt,
			}

			// get the source chat profile
			chatProfile, err := atsync.Model.GetChatProfile(ctx, userDID)
			if err == nil && chatProfile != nil {
				spcp, err := chatProfile.ToStreamplaceChatProfile()
				if err == nil {
					arrivalMsg.ChatProfile = &spcp
				}
			}

			atsync.Bus.Publish(rec.Streamer, arrivalMsg)
		})

	case *placestream.Key:
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

	case *placestream.BroadcastOrigin:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync broadcast origin creator bluesky repo: %w", err)
		}
		_, err = atsync.SyncBlueskyRepoCached(ctx, rec.Streamer)
		if err != nil {
			return fmt.Errorf("failed to sync broadcast origin streamer bluesky repo: %w", err)
		}
		err = atsync.Model.UpdateBroadcastOrigin(ctx, *rec, aturi)
		if err != nil {
			log.Error(ctx, "failed to update broadcast origin", "err", err)
		}
		view := placestream.BroadcastDefs_BroadcastOriginView{
			Uri: aturi.String(),
			Cid: cid,
			Author: appbsky.ActorDefs_ProfileViewBasic{
				Did:    userDID,
				Handle: repo.Handle,
			},
			Record: &glexrt.LexiconTypeDecoder{Val: rec},
		}
		// publishes with an empty string because we're discovering the stream
		go atsync.Bus.Publish("", view)

	case *placestream.MetadataConfiguration:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
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

	case *placestream.ModerationPermission:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		log.Debug(ctx, "creating moderation delegation", "streamerDID", userDID, "moderatorDID", rec.Moderator)

		err = atsync.Model.CreateModerationDelegation(ctx, *rec, aturi)
		if err != nil {
			return fmt.Errorf("failed to create moderation delegation: %w", err)
		}

		view := placestream.ModerationDefs_PermissionView{
			Uri: aturi.String(),
			Cid: cid,
			Author: appbsky.ActorDefs_ProfileViewBasic{
				Did:    userDID,
				Handle: repo.Handle,
			},
			Record: &glexrt.LexiconTypeDecoder{Val: rec},
		}
		// Publish moderation permission view to WebSocket bus for real-time updates
		// This allows moderators to see their permissions instantly without page refresh
		go atsync.Bus.Publish(userDID, view)

	case *placestream.LiveViewerCount:
		log.Debug(ctx, "indexing view count", "streamer", rec.Streamer, "server", rec.Server, "count", rec.Count)
		// Our own record loops back through our own firehose; indexing it
		// would stack the federated copy of our local count on top of the
		// live one, double-counting every local viewer.
		if rec.Server == atsync.CLI.ServerDID() {
			break
		}
		// Check if the reporting server's DID is labeled as banned or !no-viewers
		serverLabels, err := atsync.Model.GetActiveLabels(rec.Server)
		if err != nil {
			log.Error(ctx, "failed to get labels for server", "server", rec.Server, "error", err)
		} else if IsViewerBanned(serverLabels...) {
			log.Warn(ctx, "discarding view count from labeled server", "server", rec.Server)
			break
		}
		atsync.Bus.SetFederatedViewCount(rec.Streamer, rec.Server, int(rec.Count))

	case *placestream.LiveRecommendations:
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

	case *placestream.BadgeDef:
		def := &model.BadgeDef{
			URI:       aturi.String(),
			CID:       cid,
			RepoDID:   userDID,
			RKey:      rkey.String(),
			Name:      rec.Name,
			BadgeType: rec.BadgeType,
			Record:    *recCBOR,
			IndexedAt: now,
		}
		if rec.Description != nil {
			def.Description = *rec.Description
		}
		if rec.Image != nil {
			def.ImageCID = rec.Image.Ref.String()
			def.ImageMimeType = rec.Image.MimeType
		}
		if err := atsync.Model.UpsertBadgeDef(ctx, def); err != nil {
			return fmt.Errorf("failed to upsert badge def: %w", err)
		}
		log.Debug(ctx, "indexed badge def", "uri", aturi.String(), "name", rec.Name)

	case *placestream.BadgeIssuance:
		issuance := &model.BadgeIssuance{
			URI:          aturi.String(),
			CID:          cid,
			RepoDID:      userDID,
			RKey:         rkey.String(),
			RecipientDID: rec.Did,
			BadgeURI:     rec.Badge.Uri,
			Record:       *recCBOR,
			IndexedAt:    now,
		}
		if err := atsync.Model.UpsertBadgeIssuance(ctx, issuance); err != nil {
			return fmt.Errorf("failed to upsert badge issuance: %w", err)
		}
		log.Debug(ctx, "indexed badge issuance", "uri", aturi.String(), "recipient", rec.Did)

	case *placestream.Video:
		_, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		if err := atsync.Model.UpsertVideo(ctx, *rec, aturi); err != nil {
			return fmt.Errorf("failed to upsert video: %w", err)
		}
		log.Debug(ctx, "indexed video", "uri", aturi.String(), "title", rec.Title)

	case *placestream.MediaTrack:
		// Tracks not backed by a muxlTrack (we don't define any other
		// shape yet) are skipped with a warning — there'd be no blob
		// to key the row off of.
		if rec.Track.MediaDefs_MuxlTrack == nil || rec.Track.MediaDefs_MuxlTrack == nil {
			log.Warn(ctx, "track record missing muxlTrack; skipping", "uri", aturi.String())
			return nil
		}
		if err := atsync.Model.UpsertMediaTrack(ctx, *rec, aturi); err != nil {
			return fmt.Errorf("failed to upsert media track: %w", err)
		}
		mt := rec.Track.MediaDefs_MuxlTrack
		log.Debug(ctx, "indexed media track", "uri", aturi.String(), "blob", mt.Blob, "mediaType", mt.MediaType)

	case *placestream.MediaOrigin:
		// Origin records are published by streamplace nodes (not users)
		// against their own server-repo DID. The aturi's authority is
		// the publishing server.
		if err := atsync.Model.UpsertMediaOrigin(ctx, *rec, aturi); err != nil {
			return fmt.Errorf("failed to upsert media origin: %w", err)
		}
		log.Debug(ctx, "indexed media origin", "uri", aturi.String(), "blob", rec.Blob, "server", userDID)

	case *placestream.BetaInvite:
		// Invite records grant a specific account access to a named
		// beta feature. We index all of them as they fly past; gate
		// callers filter by RepoDID to a single operator-configured
		// issuer (the `--beta-invite-did` flag), so anyone else
		// minting these records is harmless noise.
		if err := atsync.Model.UpsertBetaInvite(ctx, *rec, aturi); err != nil {
			return fmt.Errorf("failed to upsert beta invite: %w", err)
		}
		log.Debug(ctx, "indexed beta invite", "uri", aturi.String(), "did", rec.Did, "feature", rec.Feature)

		// Notify the invited account that they're off the waitlist — but
		// only for a genuinely new invite arriving live from the trusted
		// issuer. Backfill/first-sync and record updates re-index existing
		// invites on every restart and must not re-notify.
		if !isFirstSync && !isUpdate &&
			atsync.CLI.BetaInviteDID != "" && userDID == atsync.CLI.BetaInviteDID {
			atsync.notifyBetaInvite(ctx, rec)
		}

	case *placestream.BetaRequest:
		// Access requests are published by users in their own repos. We
		// index them so operators can see who's waiting and so
		// place.stream.beta.getStatus can report "requested".
		if err := atsync.Model.UpsertBetaRequest(ctx, *rec, aturi); err != nil {
			return fmt.Errorf("failed to upsert beta request: %w", err)
		}
		log.Debug(ctx, "indexed beta request", "uri", aturi.String(), "did", userDID, "feature", rec.Feature)

	case *placestream.MediaViewCount:
		// View-count records are published by streamplace nodes (in
		// their server repos) reporting on traffic they observed.
		// Multiple reporters publish records for the same video; the
		// query layer (place.stream.media.getVideo) sums across them.
		if err := atsync.Model.UpsertMediaViewCount(ctx, *rec, aturi); err != nil {
			return fmt.Errorf("failed to upsert media view count: %w", err)
		}
		log.Debug(ctx, "indexed media view count",
			"uri", aturi.String(), "video", rec.Video, "count", rec.Count, "reporter", userDID)

	case *placestream.VodComment:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}

		log.Debug(ctx, "place.stream.vod.comment detected", "video", rec.Video, "repo", repo.Handle)

		// Check if the video author has blocked the commenter
		videoATURI, parseErr := syntax.ParseATURI(rec.Video)
		var videoAuthor string
		if parseErr == nil {
			videoAuthor = videoATURI.Authority().String()
			block, err := atsync.Model.GetUserBlock(ctx, videoAuthor, userDID)
			if err != nil {
				log.Warn(ctx, "failed to check user block for VOD comment", "err", err)
			} else if block != nil {
				log.Debug(ctx, "excluding VOD comment from blocked user", "userDID", userDID, "videoAuthor", videoAuthor)
				return nil
			}
		} else {
			log.Warn(ctx, "failed to parse video URI for block check", "video", rec.Video, "err", err)
		}

		vc := &model.VodComment{
			CID:            cid,
			URI:            aturi.String(),
			CreatedAt:      now,
			Comment:        recCBOR,
			RepoDID:        userDID,
			Repo:           repo,
			VideoURI:       rec.Video,
			VideoAuthorDID: videoAuthor,
			IndexedAt:      &now,
		}
		if rec.Reply != nil && rec.Reply.Parent.Uri != "" && rec.Reply.Root.Uri != "" {
			vc.ReplyToCID = &rec.Reply.Parent.Cid
		}

		// check for javascript: links in facets
		for _, facet := range rec.Facets {
			for _, feature := range facet.Features {
				if link := feature.RichtextFacet_Link; link != nil {
					if link.Uri != "" && strings.HasPrefix(strings.ToLower(link.Uri), "javascript:") {
						log.Warn(ctx, "excluding comment with javascript: link", "uri", aturi.String(), "link", link.Uri)
						return nil
					}
				}
			}
		}

		err = atsync.Model.CreateVodComment(ctx, vc)
		if err != nil {
			log.Error(ctx, "failed to create VOD comment", "err", err)
			return nil
		}
		vc, err = atsync.Model.GetVodComment(aturi.String())
		if err != nil {
			log.Error(ctx, "failed to get just-saved VOD comment", "err", err)
			return nil
		}
		if vc == nil {
			log.Error(ctx, "failed to retrieve just-saved VOD comment")
			return nil
		}
		sc, err := vc.ToStreamplaceCommentView()
		if err != nil {
			log.Error(ctx, "failed to convert VOD comment to view", "err", err)
			return nil
		}

		if sc.Author.Handle == "" || sc.Author.Handle == "handle.invalid" {
			sc.Author.Handle = atsync.ResolveAuthorHandle(ctx, sc.Author.Did)
		}

		if videoAuthor != "" {
			go atsync.Bus.Publish(videoAuthor, sc)
		} else {
			go atsync.Bus.Publish(userDID, sc)
		}

	case *placestream.Like:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}

		log.Debug(ctx, "place.stream.like detected", "subject", rec.Subject, "repo", repo.Handle)

		// A user can only like a subject once — refuse to index a duplicate
		// rather than inflating the count with a second row.
		existing, err := atsync.Model.GetLikeBySubjectAndUser(ctx, rec.Subject, userDID)
		if err != nil {
			return fmt.Errorf("check existing like: %w", err)
		}
		if existing != nil {
			log.Debug(ctx, "ignoring duplicate like", "subject", rec.Subject, "repo", userDID)
			return nil
		}

		like := &model.Like{
			CID:       cid,
			URI:       aturi.String(),
			Subject:   rec.Subject,
			RepoDID:   userDID,
			Repo:      repo,
			IndexedAt: &now,
			CreatedAt: now,
		}
		err = atsync.Model.CreateLike(ctx, like)
		if err != nil {
			log.Error(ctx, "failed to create VOD like", "err", err)
			return nil
		}

	case *placestream.VodGate:
		repo, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		if r == nil {
			// someone we don't know about
			return nil
		}
		log.Debug(ctx, "creating VOD gate", "userDID", userDID, "hiddenComment", rec.HiddenComment)
		gate := &model.VodGate{
			RKey:          rkey.String(),
			RepoDID:       userDID,
			HiddenComment: rec.HiddenComment,
			CID:           cid,
			CreatedAt:     now,
			Repo:          repo,
		}
		err = atsync.Model.CreateVodGate(ctx, gate)
		if err != nil {
			return fmt.Errorf("failed to create VOD gate: %w", err)
		}

	default:
		log.Debug(ctx, "unhandled record type", "type", reflect.TypeOf(rec))
	}
	return nil
}

// notifyBetaInvite pushes a "you're off the waitlist" notification to the
// account named by a freshly-issued, trusted beta invite. Best-effort: any
// failure is logged, never returned, since the invite is already indexed and
// the upload gate works regardless of whether the push lands.
func (atsync *ATProtoSynchronizer) notifyBetaInvite(ctx context.Context, rec *placestream.BetaInvite) {
	if atsync.Noter == nil || atsync.StatefulDB == nil {
		return
	}
	tokens, err := atsync.StatefulDB.GetManyNotificationTokens([]string{rec.Did})
	if err != nil {
		log.Error(ctx, "beta invite notification: failed to load tokens", "did", rec.Did, "err", err)
		return
	}
	if len(tokens) == 0 {
		log.Debug(ctx, "beta invite notification: no device tokens for invitee", "did", rec.Did, "feature", rec.Feature)
		return
	}
	blast := betaInviteBlast(rec.Feature)
	if err := atsync.Noter.Blast(ctx, tokens, blast); err != nil {
		log.Error(ctx, "beta invite notification: blast failed", "did", rec.Did, "feature", rec.Feature, "err", err)
		return
	}
	log.Log(ctx, "sent beta invite notification", "did", rec.Did, "feature", rec.Feature, "tokens", len(tokens))
}

// betaInviteBlast builds the push payload for a newly-granted beta feature.
// Copy is feature-aware where we have something specific to say.
func betaInviteBlast(feature string) *notificationpkg.NotificationBlast {
	switch feature {
	case "vod":
		// Uploads are a web flow today, and pushes land on the native app, so
		// we route to home rather than a route the app doesn't register.
		return &notificationpkg.NotificationBlast{
			Title: "🎉 You're off the waitlist!",
			Body:  "You can now upload videos to Streamplace.",
			Data:  map[string]string{"path": "/"},
		}
	default:
		return &notificationpkg.NotificationBlast{
			Title: "🎉 You're off the waitlist!",
			Body:  fmt.Sprintf("You've been granted access to the %s beta.", feature),
			Data:  map[string]string{"path": "/"},
		}
	}
}
