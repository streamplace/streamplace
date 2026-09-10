package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/appbsky"

	"stream.place/streamplace/pkg/atproto"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
)

// todo: does this mean a whole message has to fit within the buffer?
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var pingPeriod = 5 * time.Second

func (a *StreamplaceAPI) HandleWebsocket(ctx context.Context) httprouter.Handle {
	ctx = log.WithLogValues(ctx, "func", "HandleWebsocket")
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		ip, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			ip = req.RemoteAddr
		}

		if a.CLI.RateLimitWebsocket > 0 {
			if !a.connTracker.AddConnection(ip) {
				log.Warn(ctx, "rate limit exceeded", "ip", ip, "path", req.URL.Path)
				apierrors.WriteHTTPTooManyRequests(w, "rate limit exceeded")
				return
			}

			defer a.connTracker.RemoveConnection(ip)
		}

		uu, _ := uuid.NewV7()
		connID := uu.String()

		ctx = log.WithLogValues(ctx, "uuid", connID, "remoteAddr", req.RemoteAddr, "url", req.URL.String())
		log.Log(ctx, "websocket opened")
		spmetrics.WebsocketsOpen.Inc()
		defer spmetrics.WebsocketsOpen.Dec()
		user := params.ByName("repoDID")
		if user == "" {
			apierrors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		repoDID, err := a.NormalizeUser(ctx, user)
		if err != nil {
			apierrors.WriteHTTPNotFound(w, "user not found", err)
			return
		}
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not upgrade to websocket", err)
			return
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer conn.Close()

		initialBurst := make(chan any, 200)
		err = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		if err != nil {
			log.Error(ctx, "could not set read deadline", "error", err)
			return
		}

		pongCh := make(chan struct{})

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-pongCh:
					err := conn.SetReadDeadline(time.Now().Add(30 * time.Second))
					if err != nil {
						log.Error(ctx, "could not set read deadline", "error", err)
						return
					}
				case <-time.After(30 * time.Second):
					log.Log(ctx, "websocket timeout, closing connection")
					// timeout!
					conn.Close()
					cancel()
					return
				}
			}
		}()

		conn.SetPongHandler(func(appData string) error {
			log.Debug(ctx, "received pong", "appData", appData)
			pongCh <- struct{}{}
			return nil
		})
		// The rendition list last sent to this viewer, as its joined names.
		var sentRenditions atomic.Value
		sentRenditions.Store("")
		go func() {

			ch := a.Bus.Subscribe(repoDID)
			defer a.Bus.Unsubscribe(repoDID, ch)
			// Create a ticker that fires every 3 seconds
			ticker := time.NewTicker(3 * time.Second)
			pingTicker := time.NewTicker(pingPeriod)
			defer ticker.Stop()
			defer pingTicker.Stop()

			send := func(msg any) {
				bs, err := json.Marshal(msg)
				if err != nil {
					log.Error(ctx, "could not marshal message", "error", err)
					return
				}
				log.Debug(ctx, "sending message", "message", string(bs))
				err = conn.WriteMessage(websocket.TextMessage, bs)
				if err != nil {
					log.Error(ctx, "could not write message", "error", err)
					return
				}
			}

			for {
				select {
				case msg := <-ch:
					send(msg)
				case msg := <-initialBurst:
					send(msg)
				case <-ticker.C:
					// The renditions a viewer can pick are whatever this
					// node's live window holds now: they appear a little
					// after the stream starts (the transcoder's round trip),
					// and on a syndicating node they arrive from the origin.
					if names := a.MediaManager.LiveRenditionNames(repoDID); len(names) > 0 {
						if key := strings.Join(names, ","); sentRenditions.Swap(key) != key {
							send(renditionsMessage(names))
						}
					}
					bs, err := json.Marshal(a.viewerCountMessage(ctx, repoDID))
					if err != nil {
						log.Error(ctx, "could not marshal view count", "error", err)
						continue
					}
					err = conn.WriteMessage(websocket.TextMessage, bs)
					if err != nil {
						log.Error(ctx, "could not write ping message", "error", err)
						return
					}
				case <-pingTicker.C:
					err := conn.WriteMessage(websocket.PingMessage, []byte{})
					if err != nil {
						log.Error(ctx, "could not write ping message", "error", err)
						return
					}
				case <-ctx.Done():
					log.Debug(ctx, "context done, stopping websocket sender")
					return
				}
			}
		}()

		go func() {
			profile, err := a.Model.GetRepo(repoDID)
			if err != nil {
				log.Error(ctx, "could not get profile", "error", err)
				return
			}
			if profile != nil {
				p := map[string]any{
					"$type":  "app.bsky.actor.defs#profileViewBasic",
					"did":    repoDID,
					"handle": profile.Handle,
				}
				// The streamer's verified badge comes from the same place as
				// chat authors'.
				if state := a.ATSync.VerificationState(ctx, repoDID); state != nil {
					p["verification"] = state
				}
				initialBurst <- p
			}
		}()

		go func() {
			seg, err := a.LocalDB.LatestSegmentForUser(repoDID)
			if err != nil {
				log.Error(ctx, "could not get replies", "error", err)
				return
			}
			spSeg, err := seg.ToStreamplaceSegment()
			if err != nil {
				log.Error(ctx, "could not convert segment to streamplace segment", "error", err)
				return
			}
			initialBurst <- spSeg
			names := a.MediaManager.LiveRenditionNames(repoDID)
			if len(names) == 0 && a.CLI.LivepeerGatewayURL != "" {
				// This node transcodes but hasn't a rendition in its window
				// yet: the profiles it is about to produce.
				videoRenditions, err := renditions.GenerateRenditionsFrom(spSeg, a.CLI.RenditionLadder())
				if err != nil {
					log.Error(ctx, "could not generate renditions", "error", err)
					return
				}
				for _, r := range videoRenditions {
					names = append(names, r.Name)
				}
			}
			sentRenditions.Store(strings.Join(names, ","))
			initialBurst <- renditionsMessage(names)
		}()

		go func() {
			ls, err := a.Model.GetLatestLivestreamForRepo(repoDID)
			if err != nil {
				log.Error(ctx, "could not get latest livestream", "error", err)
				return
			}
			if ls == nil {
				log.Error(ctx, "no livestream found", "repoDID", repoDID)
				return
			}
			lsv, err := ls.ToLivestreamView()
			if err != nil {
				log.Error(ctx, "could not marshal livestream", "error", err)
				return
			}
			initialBurst <- lsv
		}()

		go func() {
			initialBurst <- a.viewerCountMessage(ctx, repoDID)
		}()

		go func() {
			messages, err := a.Model.MostRecentChatMessages(repoDID)
			if err != nil {
				log.Error(ctx, "could not get chat messages", "error", err)
				return
			}

			// Add mod badges to messages
			issuerDID := fmt.Sprintf("did:web:%s", a.CLI.BroadcasterHost)
			for _, message := range messages {
				if !a.ATSync.ChatAllowed(ctx, message.Author.Did) {
					continue
				}
				err := atproto.AddModBadgeIfApplicable(ctx, &message, repoDID, issuerDID, a.Model)
				if err != nil {
					log.Error(ctx, "failed to add mod badge to message", "error", err)
				}
				a.ATSync.DecorateVerification(ctx, &message)
				if message.Author.Handle == "" || message.Author.Handle == "handle.invalid" {
					message.Author.Handle = a.ATSync.ResolveAuthorHandle(ctx, message.Author.Did)
				}
				initialBurst <- message
			}
		}()

		// get the latest active pinned message for the repo
		go func() {
			pin, err := a.Model.GetActivePinnedRecord(ctx, repoDID)
			if err != nil {
				log.Error(ctx, "could not get pinned record", "error", err)
				return
			}
			if pin != nil {
				prv, err := pin.ToStreamplacePinnedRecordView()
				if err != nil {
					log.Error(ctx, "could not convert pinned record to streamplace view", "error", err)
					return
				}
				// look up the original message, pinner
				msg, err := a.Model.GetChatMessage(prv.Record.PinnedMessage)
				if err != nil {
					log.Error(ctx, "failed to get pinned message", err)
					return
				}
				// if the message was deleted, treat as no pinned message
				if msg != nil && msg.DeletedAt != nil {
					log.Log(ctx, "pinned message was deleted, skipping", "uri", msg.URI)
					return
				}
				// if no pinned by, use the repo owner as the pinner
				if prv.Record.PinnedBy == nil {
					prv.Record.PinnedBy = &repoDID
				}
				profile, err := a.Model.GetChatProfile(ctx, *prv.Record.PinnedBy)
				if err != nil {
					log.Error(ctx, "failed to get chat profile", err)
					return
				}
				if msg != nil {
					msgView, err := msg.ToStreamplaceMessageView()
					if err != nil {
						log.Error(ctx, "failed to convert chat message: %w", err)
						return
					}
					if msgView.Author.Handle == "" || msgView.Author.Handle == "handle.invalid" {
						msgView.Author.Handle = a.ATSync.ResolveAuthorHandle(ctx, msgView.Author.Did)
					}
					prv.Message = msgView
				}
				if profile != nil {
					profileView, err := profile.ToStreamplaceChatProfile()
					if err != nil {
						log.Error(ctx, "failed to convert chat profile: %w", err)
						return
					}
					prv.PinnedBy = &profileView
				}
				initialBurst <- prv
			}
		}()

		go func() {
			teleports, err := a.Model.GetActiveTeleportsToRepo(repoDID)
			if err != nil {
				log.Error(ctx, "could not get active teleports", "error", err)
				return
			}
			// just send the latest one if it started <3m ago
			if len(teleports) > 0 && teleports[0].StartsAt.After(time.Now().Add(-3*time.Minute)) {
				tp := teleports[0]
				if tp.Repo == nil {
					log.Error(ctx, "teleportee repo is nil", "uri", tp.URI)
					return
				}
				viewerCount := a.Bus.GetViewerCount(tp.RepoDID)
				arrivalMsg := placestream.Livestream_TeleportArrival{
					LexiconTypeID: "place.stream.livestream#teleportArrival",
					TeleportUri:   tp.URI,
					Source: appbsky.ActorDefs_ProfileViewBasic{
						Did:    tp.RepoDID,
						Handle: tp.Repo.Handle,
					},
					ViewerCount: int64(viewerCount),
					StartsAt:    tp.StartsAt.Format(time.RFC3339),
				}

				// get the source chat profile
				chatProfile, err := a.Model.GetChatProfile(ctx, tp.RepoDID)
				if err == nil && chatProfile != nil {
					spcp, err := chatProfile.ToStreamplaceChatProfile()
					if err == nil {
						arrivalMsg.ChatProfile = &spcp
					}
				}

				initialBurst <- arrivalMsg
			}
		}()

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Error(ctx, "error reading message", "error", err)
				break
			}
			log.Log(ctx, "received message", "messageType", messageType, "message", string(message))
		}
	}
}

// renditionsMessage is the place.stream.defs#renditions message listing the
// video renditions a viewer can pick, plus the audio-only rendition every
// stream has.
func renditionsMessage(names []string) placestream.Defs_Renditions {
	out := placestream.Defs_Renditions{
		LexiconTypeID: "place.stream.defs#renditions",
		Renditions:    make([]placestream.Defs_Rendition, 0, len(names)+1),
	}
	for _, n := range names {
		out.Renditions = append(out.Renditions, placestream.Defs_Rendition{LexiconTypeID: "place.stream.defs#rendition", Name: n})
	}
	out.Renditions = append(out.Renditions, placestream.Defs_Rendition{LexiconTypeID: "place.stream.defs#rendition", Name: renditions.AudioRendition.Name})
	return out
}

// viewerCountMessage is the viewerCount event: who is watching now (the
// bus) and how many sessions this livestream has had (statedb's running
// total, when the node keeps one).
func (a *StreamplaceAPI) viewerCountMessage(ctx context.Context, repoDID string) placestream.Livestream_ViewerCount {
	msg := placestream.Livestream_ViewerCount{
		Count:         int64(a.Bus.GetViewerCount(repoDID)),
		LexiconTypeID: "place.stream.livestream#viewerCount",
	}
	if a.StatefulDB != nil {
		if total, _, err := a.StatefulDB.GetStreamViewTotal(ctx, repoDID); err == nil && total > 0 {
			msg.Total = &total
		}
	}
	return msg
}
