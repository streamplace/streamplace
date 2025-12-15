package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/events"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

func (s *Server) handleComAtprotoSyncListRepos(ctx context.Context, cursor string, limit int) (*comatprototypes.SyncListRepos_Output, error) {
	active := true
	return &comatprototypes.SyncListRepos_Output{
		Repos: []*comatprototypes.SyncListRepos_Repo{
			{
				Did:    atproto.LexiconRepo.RepoDid(),
				Head:   atproto.LexiconRepo.SignedCommit().Data.String(),
				Rev:    atproto.LexiconRepo.SignedCommit().Rev,
				Active: &active,
			},
		},
	}, nil
}

func (s *Server) handleComAtprotoSyncGetRecord(ctx context.Context, collection string, did string, rkey string) (io.Reader, error) {
	bs, err := atproto.LexiconRepoMerkleProof(ctx, collection, rkey)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(bs), nil
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handleComAtprotoSyncSubscribeRepos(c echo.Context) error {
	ctx := log.WithLogValues(c.Request().Context(), "client_ip", c.RealIP(), "user_agent", c.Request().UserAgent())
	cursor := c.QueryParam("cursor")

	if cursor == "" {
		cursor = "0"
	}

	seq, err := strconv.Atoi(cursor)
	if err != nil {
		return err
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	evts, err := s.statefulDB.GetCommitEventsSinceSeq(atproto.LexiconRepo.RepoDid(), int64(seq))
	if err != nil {
		return err
	}

	log.Log(ctx, "got com.atproto.sync.subscribeRepos", "cursor", c.QueryParam("cursor"), "eventCount", len(evts))

	header := events.EventHeader{Op: events.EvtKindMessage}
	for _, evt := range evts {
		commit, err := evt.ToCommitEvent()
		if err != nil {
			return err
		}

		wc, err := conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return err
		}
		header.MsgType = "#commit"

		if err := header.MarshalCBOR(wc); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}

		if err := commit.MarshalCBOR(wc); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}

		if err := wc.Close(); err != nil {
			return fmt.Errorf("failed to flush-close our event write: %w", err)
		}
	}

	// We don't have anything else to do but we'll keep the socket open until the client disconnects
	for {
		_, _, err = conn.ReadMessage()
		if err != nil {
			log.Log(c.Request().Context(), "client disconnected", "error", err)
			return nil
		}
	}
}
