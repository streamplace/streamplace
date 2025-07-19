package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/carstore"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/util"
	"github.com/gorilla/websocket"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
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
	_, robs, err := atproto.OpenLexiconRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to open repo: %w", err)
	}

	bs := util.NewLoggingBstore(robs)

	root, err := atproto.CarStore.GetUserRepoHead(ctx, atproto.RepoUser)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to get user repo head: %w", err)
	}

	r, err := repo.OpenRepo(ctx, bs, root)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to open repo: %w", err)
	}

	_, _, err = r.GetRecordBytes(ctx, collection+"/"+rkey)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to get record bytes: %w", err)
	}

	blocks := bs.GetLoggedBlocks()

	buf := new(bytes.Buffer)
	hb, err := cbor.DumpObject(&car.CarHeader{
		Roots:   []cid.Cid{root},
		Version: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dump car header: %w", err)
	}
	if _, err := carstore.LdWrite(buf, hb); err != nil {
		return nil, err
	}

	for _, blk := range blocks {
		if _, err := carstore.LdWrite(buf, blk.Cid().Bytes(), blk.RawData()); err != nil {
			return nil, err
		}
	}

	return bytes.NewReader(buf.Bytes()), nil
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

	evts, err := s.model.GetCommitEventsSinceSeq(atproto.LexiconRepo.RepoDid(), int64(seq))
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
