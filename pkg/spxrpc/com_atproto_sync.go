package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/carstore"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
	"stream.place/streamplace/pkg/atproto"
)

func (s *Server) handleComAtprotoSyncListRepos(ctx context.Context, cursor string, limit int) (*comatprototypes.SyncListRepos_Output, error) {
	active := true
	root, err := atproto.LexiconRepo.GetRepoRoot(ctx, atproto.RepoUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo root: %w", err)
	}
	rev, err := atproto.LexiconRepo.GetRepoRev(ctx, atproto.RepoUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo rev: %w", err)
	}
	return &comatprototypes.SyncListRepos_Output{
		Repos: []*comatprototypes.SyncListRepos_Repo{
			{
				Did:    fmt.Sprintf("did:web:%s", s.cli.PublicHost),
				Head:   root.String(),
				Rev:    rev,
				Active: &active,
			},
		},
	}, nil
}

func (s *Server) handleComAtprotoSyncGetRecord(ctx context.Context, collection string, did string, rkey string) (io.Reader, error) {
	root, blocks, err := atproto.LexiconRepo.GetRecordProof(ctx, atproto.RepoUser, collection, rkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get record proof: %w", err)
	}

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

	return buf, nil
}
