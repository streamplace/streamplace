package atproto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/log"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"

	glex "github.com/streamplace/glex/runtime"
)

func (atsync *ATProtoSynchronizer) handleCreateUpdate(ctx context.Context, userDID string, rkey syntax.RecordKey, recCBOR *[]byte, cid string, collection syntax.NSID, isUpdate bool, isFirstSync bool) error {
	ctx = log.WithLogValues(ctx, "func", "handleCreateUpdate", "userDID", userDID, "rkey", rkey.String(), "cid", cid, "collection", collection.String())
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
	cb, err := glex.CborDecodeValue(*recCBOR)
	if errors.Is(err, glex.ErrUnrecognizedType) {
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
		err := atsync.Model.UpsertBlock(ctx, *rec, aturi)
		if err != nil {
			return fmt.Errorf("failed to create block: %w", err)
		}
		block, err := atsync.Model.GetBlock(ctx, rkey.String())
		if err != nil || block == nil {
			return fmt.Errorf("failed to get block after we just saved it?!: %w", err)
		}
		streamplaceBlock, err := block.ToBlockView()
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
		err := atsync.Model.UpsertBskyProfile(ctx, *rec, aturi, wasStreamplace)
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

		err = atsync.Model.UpsertChatMessage(ctx, *rec, aturi)
		if err != nil {
			log.Error(ctx, "failed to create chat message", "err", err)
			return nil
		}
		mcm, err := atsync.Model.GetChatMessage(aturi.String())
		if err != nil {
			log.Error(ctx, "failed to get just-saved chat message", "err", err)
			return nil
		}
		if mcm == nil {
			log.Error(ctx, "failed to retrieve just-saved chat message", "err", err)
			return nil
		}
		scm, err := mcm.ToMessageView()
		if err != nil {
			log.Error(ctx, "failed to convert chat message to streamplace message view", "err", err)
			return nil
		}

		// Add mod badge if the author is a moderator
		issuerDID := fmt.Sprintf("did:web:%s", atsync.CLI.BroadcasterHost)
		err = AddModBadgeIfApplicable(ctx, scm, rec.Streamer, issuerDID, atsync.Model)
		if err != nil {
			log.Error(ctx, "failed to add mod badge", "err", err)
		}

		if scm.Author.Handle == "" || scm.Author.Handle == "handle.invalid" {
			scm.Author.Handle = atsync.ResolveAuthorHandle(ctx, scm.Author.Did)
		}

		go atsync.Bus.Publish(rec.Streamer, scm)

		if !isUpdate && !isFirstSync {

			task := &statedb.ChatTask{
				MessageView: *scm,
			}

			_, err = atsync.StatefulDB.EnqueueTask(ctx, statedb.TaskChat, task, statedb.WithTaskKey(fmt.Sprintf("chat-message::%s", aturi.String())))
			if err != nil {
				log.Error(ctx, "failed to enqueue notification task", "err", err)
			}
		}

	case *placestream.ChatGate:
		_, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		if r == nil {
			// someone we don't know about
			return nil
		}
		log.Debug(ctx, "creating gate", "userDID", userDID, "hiddenMessage", rec.HiddenMessage)
		err = atsync.Model.UpsertGate(ctx, *rec, aturi)
		if err != nil {
			return fmt.Errorf("failed to create gate: %w", err)
		}
		savedGate, err := atsync.Model.GetGate(ctx, rkey.String())
		if err != nil {
			return fmt.Errorf("failed to get gate after we just saved it?!: %w", err)
		}
		if savedGate == nil {
			return fmt.Errorf("failed to get gate after we just saved it?!: not found")
		}
		go atsync.Bus.Publish(userDID, *savedGate)

	case *placestream.ChatPinnedRecord:
		_, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		if r == nil {
			return nil
		}
		log.Debug(ctx, "creating pinned record", "userDID", userDID, "pinnedMessage", rec.PinnedMessage)
		err = atsync.Model.UpsertPinnedRecord(ctx, *rec, aturi)
		if err != nil {
			return fmt.Errorf("failed to create pinned record: %w", err)
		}
		savedPin, err := atsync.Model.GetPinnedRecord(ctx, aturi.String())
		if err != nil {
			return fmt.Errorf("failed to get pinned record after we just saved it: %w", err)
		}
		if savedPin == nil {
			return fmt.Errorf("failed to get pinned record after we just saved it: not found")
		}
		pinnedView := *savedPin
		pinnedBy := userDID
		if rec.PinnedBy != nil {
			pinnedBy = *rec.PinnedBy
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
			msgView, err := msg.ToMessageView()
			if err != nil {
				return fmt.Errorf("failed to convert chat message: %w", err)
			}
			pinnedView.Message = msgView
		}
		if profile != nil {
			pinnedView.PinnedBy = profile
		}
		go atsync.Bus.Publish(userDID, pinnedView)

	case *placestream.ChatProfile:
		if _, err := atsync.SyncBlueskyRepoCached(ctx, userDID); err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		err := atsync.Model.UpsertChatProfile(ctx, *rec, aturi)
		if err != nil {
			log.Error(ctx, "failed to create chat profile", "err", err)
		}

	case *placestream.ServerSettings:
		_, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		err = atsync.Model.UpsertServerSettings(ctx, *rec, aturi)
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

		if livestream, ok := d["place.stream.livestream"]; ok {
			if _, err := atsync.SyncBlueskyRepoCached(ctx, userDID); err != nil {
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
			if err := atsync.Model.UpsertFeedPost(ctx, rec, aturi, "livestream"); err != nil {
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
			block, err := atsync.Model.GetUserBlock(ctx, livestream.Author.Did, userDID)
			if err != nil {
				return fmt.Errorf("failed to get user block: %w", err)
			}
			if block != nil {
				log.Warn(ctx, "excluding message from blocked user", "userDID", userDID, "subjectDID", livestream.Author.Did)
				return nil
			}
			// if fc.cli.PrintChat {
			// 	fmt.Printf("@%s%s %s\n", blue.Sprintf(repo.Handle), green.Sprintf(":"), rec.Text)
			// }
			livestreamRec, ok := livestream.Record.Val.(*placestream.Livestream)
			if !ok || livestreamRec.Post == nil {
				return fmt.Errorf("livestream %s has no linked post record", livestream.Uri)
			}
			err = atsync.Model.UpsertFeedPost(ctx, rec, aturi, "reply")
			if err != nil {
				log.Error(ctx, "failed to create feed post", "err", err)
			}
			postView := appbsky.FeedDefs_PostView{
				LexiconTypeID: "app.bsky.feed.defs#postView",
				Uri:           aturi.String(),
				Cid:           cid,
				Author: appbsky.ActorDefs_ProfileViewBasic{
					Did:    userDID,
					Handle: repo.Handle,
				},
				Record:    &glex.LexiconTypeDecoder{Val: rec},
				IndexedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			go atsync.Bus.Publish(livestream.Author.Did, postView)
		}

	case *placestream.Livestream:
		if r == nil {
			// we don't know about this repo
			return nil
		}
		err = atsync.Model.UpsertLivestream(ctx, *rec, aturi)
		if err != nil {
			return fmt.Errorf("failed to create livestream: %w", err)
		}
		lsv, err := atsync.Model.GetLatestLivestreamForRepo(userDID)
		if err != nil {
			return fmt.Errorf("failed to get latest livestream for repo: %w", err)
		}
		if lsv == nil {
			return fmt.Errorf("no livestream found after we just saved it: %s", userDID)
		}
		go atsync.Bus.Publish(userDID, lsv)

		if !isFirstSync {
			if atsync.CLI.StreamIsAllowed(userDID) != nil {
				// they're live somewhere but they don't have nothin' to do with us
				return nil
			}
			log.Log(ctx, "stream is allowed, queuing finalize task")
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
		err = atsync.Model.UpsertTeleport(ctx, *rec, aturi, int64(viewerCount))
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
				arrivalMsg.ChatProfile = chatProfile
			}

			atsync.Bus.Publish(rec.Streamer, arrivalMsg)
		})

	case *placestream.Key:
		log.Debug(ctx, "creating key", "key", rec)
		time, err := aqtime.FromString(rec.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to parse createdAt: %w", err)
		}
		key := indexdb.SigningKey{
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
			Record: &glex.LexiconTypeDecoder{Val: rec},
		}
		// publishes with an empty string because we're discovering the stream
		go atsync.Bus.Publish("", view)

	case *placestream.MetadataConfiguration:
		if _, err := atsync.SyncBlueskyRepoCached(ctx, userDID); err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		log.Debug(ctx, "creating metadata configuration", "metadata", rec)
		err := atsync.Model.UpsertMetadataConfiguration(ctx, *rec, aturi)
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
			Record: &glex.LexiconTypeDecoder{Val: rec},
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

		recommendation := &indexdb.Recommendation{
			UserDID:   userDID,
			Streamers: json.RawMessage(streamersJSON),
			CreatedAt: createdAt,
		}

		err = atsync.Model.UpsertRecommendation(recommendation)
		if err != nil {
			return fmt.Errorf("failed to upsert recommendation: %w", err)
		}

	case *placestream.BadgeDef:
		if err := atsync.Model.UpsertBadgeDef(ctx, *rec, aturi); err != nil {
			return fmt.Errorf("failed to upsert badge def: %w", err)
		}
		log.Debug(ctx, "indexed badge def", "uri", aturi.String(), "name", rec.Name)

	case *placestream.BadgeIssuance:
		if err := atsync.Model.UpsertBadgeIssuance(ctx, *rec, aturi); err != nil {
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
		if rec.Track.MediaDefs_MuxlTrack == nil {
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

		err = atsync.Model.UpsertVodComment(ctx, *rec, aturi)
		if err != nil {
			log.Error(ctx, "failed to create VOD comment", "err", err)
			return nil
		}
		scv, err := atsync.Model.GetVodComment(aturi.String())
		if err != nil {
			log.Error(ctx, "failed to get just-saved VOD comment", "err", err)
			return nil
		}
		if scv == nil {
			log.Error(ctx, "failed to retrieve just-saved VOD comment")
			return nil
		}
		sc := *scv

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

		err = atsync.Model.UpsertLike(ctx, *rec, aturi)
		if err != nil {
			log.Error(ctx, "failed to create VOD like", "err", err)
			return nil
		}

	case *placestream.VodGate:
		_, err := atsync.SyncBlueskyRepoCached(ctx, userDID)
		if err != nil {
			return fmt.Errorf("failed to sync bluesky repo: %w", err)
		}
		if r == nil {
			// someone we don't know about
			return nil
		}
		log.Debug(ctx, "creating VOD gate", "userDID", userDID, "hiddenComment", rec.HiddenComment)
		err = atsync.Model.UpsertVodGate(ctx, *rec, aturi)
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
	notifications, err := atsync.StatefulDB.GetManyNotifications([]string{rec.Did})
	if err != nil {
		log.Error(ctx, "beta invite notification: failed to load tokens", "did", rec.Did, "err", err)
		return
	}
	if len(notifications) == 0 {
		log.Debug(ctx, "beta invite notification: no device tokens for invitee", "did", rec.Did, "feature", rec.Feature)
		return
	}
	blast := betaInviteBlast(rec.Feature)
	targets := make([]notificationpkg.NotificationTarget, len(notifications))
	for i, n := range notifications {
		targets[i] = notificationpkg.NotificationTarget{Token: n.Token, Type: n.Type}
	}
	if err := atsync.Noter.Blast(ctx, targets, blast); err != nil {
		log.Error(ctx, "beta invite notification: blast failed", "did", rec.Did, "feature", rec.Feature, "err", err)
	} else {
		log.Log(ctx, "sent beta invite notification", "did", rec.Did, "feature", rec.Feature, "tokens", len(notifications))
	}
	// Prune dead web push subscriptions so they don't accumulate.
	for _, token := range notificationpkg.ExpiredTokens(err) {
		if delErr := atsync.StatefulDB.DeleteNotification(token); delErr != nil {
			log.Error(ctx, "beta invite notification: failed to prune expired", "token", token, "err", delErr)
		}
	}
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
