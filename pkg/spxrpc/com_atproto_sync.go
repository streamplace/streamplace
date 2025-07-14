package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/carstore"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/util"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
	"stream.place/streamplace/pkg/atproto"
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
	_, robs, err := atproto.OpenLexiconRepo(ctx, s.cli)
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

// func (s *Server) handleComAtprotoSyncSubscribeRepos(c echo.Context) error {
	// conn, err := websocket.Upgrade(c.Response().Writer, c.Request(), c.Response().Header(), 1<<10, 1<<10)
	// if err != nil {
	// 	return err
	// }

	// ctx := c.Request().Context()

	// ident := c.RealIP() + "-" + c.Request().UserAgent()

	// evts, cancel, err := s.events.Subscribe(ctx, ident, func(evt *events.XRPCStreamEvent) bool {
	// 	if !s.enforcePeering {
	// 		return true
	// 	}
	// 	if peering.ID == 0 {
	// 		return true
	// 	}

	// 	for _, pid := range evt.PrivRelevantPds {
	// 		if pid == peering.ID {
	// 			return true
	// 		}
	// 	}

	// 	return false
	// }, nil)
	// if err != nil {
	// 	return err
	// }
	// defer cancel()

	// header := events.EventHeader{Op: events.EvtKindMessage}
	// for evt := range evts {
	// 	wc, err := conn.NextWriter(websocket.BinaryMessage)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	var obj lexutil.CBOR

	// 	switch {
	// 	case evt.Error != nil:
	// 		header.Op = events.EvtKindErrorFrame
	// 		obj = evt.Error
	// 	case evt.RepoCommit != nil:
	// 		header.MsgType = "#commit"
	// 		obj = evt.RepoCommit
	// 	case evt.RepoSync != nil:
	// 		header.MsgType = "#sync"
	// 		obj = evt.RepoSync
	// 	case evt.RepoIdentity != nil:
	// 		header.MsgType = "#identity"
	// 		obj = evt.RepoIdentity
	// 	case evt.RepoAccount != nil:
	// 		header.MsgType = "#account"
	// 		obj = evt.RepoAccount
	// 	case evt.RepoInfo != nil:
	// 		header.MsgType = "#info"
	// 		obj = evt.RepoInfo
	// 	default:
	// 		return fmt.Errorf("unrecognized event kind")
	// 	}

	// 	if err := header.MarshalCBOR(wc); err != nil {
	// 		return fmt.Errorf("failed to write header: %w", err)
	// 	}

	// 	if err := obj.MarshalCBOR(wc); err != nil {
	// 		return fmt.Errorf("failed to write event: %w", err)
	// 	}

	// 	if err := wc.Close(); err != nil {
	// 		return fmt.Errorf("failed to flush-close our event write: %w", err)
	// 	}
	// }

// 	return nil
// }