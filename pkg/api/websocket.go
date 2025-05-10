package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/streamplace"
)

// todo: does this mean a whole message has to fit within the buffer?
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var pingPeriod = 5 * time.Second

func (a *StreamplaceAPI) HandleWebsocket(ctx context.Context) httprouter.Handle {
	ctx = log.WithLogValues(ctx, "func", "HandleWebsocket")
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		uu, _ := uuid.NewV7()
		ctx = log.WithLogValues(ctx, "uuid", uu.String(), "remoteAddr", req.RemoteAddr, "url", req.URL.String())
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
					count := spmetrics.GetViewCount(repoDID)
					bs, err := json.Marshal(streamplace.Livestream_ViewerCount{Count: int64(count), LexiconTypeID: "place.stream.livestream#viewerCount"})
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
			seg, err := a.Model.LatestSegmentForUser(repoDID)
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
			if a.CLI.LivepeerGatewayURL != "" {
				renditions, err := renditions.GenerateRenditions(spSeg)
				if err != nil {
					log.Error(ctx, "could not generate renditions", "error", err)
					return
				}
				outRs := streamplace.Defs_Renditions{
					LexiconTypeID: "place.stream.defs#renditions",
				}
				outRs.Renditions = []*streamplace.Defs_Rendition{}
				for _, r := range renditions {
					outRs.Renditions = append(outRs.Renditions, &streamplace.Defs_Rendition{
						LexiconTypeID: "place.stream.defs#rendition",
						Name:          r.Name,
					})
				}
				initialBurst <- outRs
			}
		}()

		go func() {
			ls, err := a.Model.GetLatestLivestreamForRepo(repoDID)
			if err != nil {
				log.Error(ctx, "could not get latest livestream", "error", err)
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
			count := spmetrics.GetViewCount(repoDID)
			initialBurst <- streamplace.Livestream_ViewerCount{Count: int64(count), LexiconTypeID: "place.stream.livestream#viewerCount"}
		}()

		go func() {
			messages, err := a.Model.MostRecentChatMessages(repoDID)
			if err != nil {
				log.Error(ctx, "could not get chat messages", "error", err)
				return
			}
			for _, message := range messages {
				initialBurst <- message
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
