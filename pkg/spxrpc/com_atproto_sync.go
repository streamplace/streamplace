package spxrpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"stream.place/streamplace/pkg/comatproto"

	"github.com/bluesky-social/indigo/events"
	"github.com/gorilla/websocket"
	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

func (s *Server) handleComAtprotoSyncListRepos(ctx context.Context, cursor string, limit int) (*comatproto.SyncListRepos_Output, error) {
	active := true

	if s.isServerPDS(ctx) {
		// Server PDS: only the server repo
		return &comatproto.SyncListRepos_Output{
			Repos: []comatproto.SyncListRepos_Repo{
				{
					Did:    atproto.ServerRepo.RepoDid(),
					Head:   atproto.ServerRepo.SignedCommit().Data.String(),
					Rev:    atproto.ServerRepo.SignedCommit().Rev,
					Active: &active,
				},
			},
		}, nil
	}

	// Broadcaster PDS: only the lexicon repo
	return &comatproto.SyncListRepos_Output{
		Repos: []comatproto.SyncListRepos_Repo{
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
	if s.isServerPDS(ctx) {
		bs, err := atproto.ServerRepoMerkleProof(ctx, collection, rkey)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(bs), nil
	}
	bs, err := atproto.LexiconRepoMerkleProof(ctx, collection, rkey)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(bs), nil
}

// maxGetBlocksCIDs caps how many blocks one com.atproto.sync.getBlocks request
// may ask for. Every CID costs a store lookup and a copy into the response, so
// an uncapped list is a free amplification lever for an anonymous caller.
//
// 100 is comfortably above what any sane client sends: getBlocks passes CIDs as
// repeated query params, and the reference PDS effectively tops out at 20
// because of its query parser (see reposync.DefaultChunkSize), so this bound
// only ever fires on abuse.
const maxGetBlocksCIDs = 100

// handleComAtprotoSyncGetLatestCommit reports the head commit of this node's
// repo, so peers can anchor a verified MST walk (pkg/reposync) instead of
// downloading the whole repo as a CAR.
func (s *Server) handleComAtprotoSyncGetLatestCommit(ctx context.Context, did string) (*comatproto.SyncGetLatestCommit_Output, error) {
	var (
		commit cid.Cid
		rev    string
		err    error
	)
	if s.isServerPDS(ctx) {
		if did != atproto.ServerRepo.RepoDid() {
			return nil, echo.NewHTTPError(http.StatusNotFound, "RepoNotFound")
		}
		commit, rev, err = atproto.ServerRepoLatestCommit(ctx)
	} else {
		if did != atproto.LexiconRepo.RepoDid() {
			return nil, echo.NewHTTPError(http.StatusNotFound, "RepoNotFound")
		}
		commit, rev, err = atproto.LexiconRepoLatestCommit(ctx)
	}
	if err != nil {
		return nil, err
	}
	return &comatproto.SyncGetLatestCommit_Output{
		Cid: commit.String(),
		Rev: rev,
	}, nil
}

// handleComAtprotoSyncGetBlocks serves individual repo blocks (commit, MST
// nodes, records) as a rootless CARv1. Together with getLatestCommit this is
// what makes a streamplace node walkable by a verifying peer.
//
// Public, like the rest of sync.*: everything here is already published in the
// signed repo.
func (s *Server) handleComAtprotoSyncGetBlocks(ctx context.Context, cids []string, did string) (io.Reader, error) {
	if len(cids) > maxGetBlocksCIDs {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("InvalidRequest: at most %d cids per request, got %d", maxGetBlocksCIDs, len(cids)))
	}
	want := make([]cid.Cid, 0, len(cids))
	for _, str := range cids {
		c, err := cid.Decode(str)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("InvalidRequest: undecodable cid %q", str))
		}
		want = append(want, c)
	}

	var (
		bs  []byte
		err error
	)
	if s.isServerPDS(ctx) {
		if did != atproto.ServerRepo.RepoDid() {
			return nil, echo.NewHTTPError(http.StatusNotFound, "RepoNotFound")
		}
		bs, err = atproto.ServerRepoGetBlocks(ctx, want)
	} else {
		if did != atproto.LexiconRepo.RepoDid() {
			return nil, echo.NewHTTPError(http.StatusNotFound, "RepoNotFound")
		}
		bs, err = atproto.LexiconRepoGetBlocks(ctx, want)
	}
	if errors.Is(err, atproto.ErrBlockNotFound) {
		// Fail the whole request rather than returning a partial bag: a
		// silently short response is indistinguishable from a truncated
		// one to the client.
		return nil, echo.NewHTTPError(http.StatusNotFound, "BlockNotFound")
	}
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

func (s *Server) handleComAtprotoSyncGetRepo(ctx context.Context, did string, since string) (io.Reader, error) {
	if s.isServerPDS(ctx) {
		if did != atproto.ServerRepo.RepoDid() {
			return nil, echo.NewHTTPError(http.StatusNotFound, "RepoNotFound")
		}
		bs, err := atproto.ServerRepoGetRepo(ctx, since)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(bs), nil
	}

	if did != atproto.LexiconRepo.RepoDid() {
		return nil, echo.NewHTTPError(http.StatusNotFound, "RepoNotFound")
	}
	bs, err := atproto.LexiconRepoGetRepo(ctx, since)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(bs), nil
}

// writeCommitToWS writes a single commit event to a websocket connection.
func writeCommitToWS(conn *websocket.Conn, header *events.EventHeader, commit *comatproto.SyncSubscribeRepos_Commit) error {
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
	return nil
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

	header := events.EventHeader{Op: events.EvtKindMessage}

	if s.isServerPDS(ctx) {
		// Server PDS: replay historical events then stream live
		sub := atproto.SubscribeServerCommits()
		defer atproto.UnsubscribeServerCommits(sub)

		// Replay historical events
		serverEvts, err := atproto.GetServerCommitEventsSinceSeq(atproto.ServerRepo.RepoDid(), int64(seq))
		if err != nil {
			return err
		}
		log.Log(ctx, "com.atproto.sync.subscribeRepos (server PDS)", "cursor", cursor, "historical", len(serverEvts))

		lastSeq := int64(seq)
		for _, evt := range serverEvts {
			commit, err := evt.ToCommitEvent()
			if err != nil {
				return err
			}
			if err := writeCommitToWS(conn, &header, commit); err != nil {
				return err
			}
			lastSeq = evt.Seq
		}

		// Read pump: detect client disconnect
		disconnected := make(chan struct{})
		go func() {
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					close(disconnected)
					return
				}
			}
		}()

		// Stream live events
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-disconnected:
				log.Log(ctx, "client disconnected from subscribeRepos")
				return nil
			case evt, ok := <-sub:
				if !ok {
					return nil
				}
				// Skip events we already sent during replay
				if evt.Seq <= lastSeq {
					continue
				}
				commit, err := evt.ToCommitEvent()
				if err != nil {
					log.Error(ctx, "failed to decode live commit event", "error", err)
					continue
				}
				if err := writeCommitToWS(conn, &header, commit); err != nil {
					log.Log(ctx, "failed to write live event, client likely disconnected", "error", err)
					return nil
				}
				lastSeq = evt.Seq
			}
		}
	}

	// Broadcaster PDS: stream lexicon repo events (rarely changes, keep old behavior)
	evts, err := s.statefulDB.GetCommitEventsSinceSeq(atproto.LexiconRepo.RepoDid(), int64(seq))
	if err != nil {
		return err
	}
	log.Log(ctx, "com.atproto.sync.subscribeRepos (broadcaster PDS)", "cursor", cursor, "eventCount", len(evts))
	for _, evt := range evts {
		commit, err := evt.ToCommitEvent()
		if err != nil {
			return err
		}
		if err := writeCommitToWS(conn, &header, commit); err != nil {
			return err
		}
	}

	// Broadcaster repo rarely changes — just hold the connection open
	for {
		_, _, err = conn.ReadMessage()
		if err != nil {
			log.Log(ctx, "client disconnected", "error", err)
			return nil
		}
	}
}
