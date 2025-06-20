package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/lexicon"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

func (s *Server) handleComAtprotoRepoUploadBlob(ctx context.Context, r io.Reader, contentType string) (*comatprototypes.RepoUploadBlob_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handleComAtprotoRepoUploadBlob")
	defer span.End()

	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	var out comatprototypes.RepoUploadBlob_Output

	var xrpcType string
	var err error
	xrpcType = xrpc.Procedure
	err = client.Do(ctx, xrpcType, contentType, "com.atproto.repo.uploadBlob", nil, r, &out)

	if err != nil {
		log.Error(ctx, "upstream xrpc error", "error", err)
		return nil, err
	}

	return &out, nil
}

func (s *Server) handleComAtprotoRepoDescribeRepo(ctx context.Context, repo string) (*comatprototypes.RepoDescribeRepo_Output, error) {
	return &comatprototypes.RepoDescribeRepo_Output{
		Handle: s.cli.PublicHost,
		Did:    fmt.Sprintf("did:web:%s", s.cli.PublicHost),
		DidDoc: atproto.DIDDoc(s.cli.PublicHost),
		Collections: []string{
			"com.atproto.lexicon.schema",
		},
		HandleIsCorrect: true,
	}, nil
}

func (s *Server) handleComAtprotoRepoListRecords(ctx context.Context, collection string, cursor string, limit int, repoStr string, reverse *bool) (*comatprototypes.RepoListRecords_Output, error) {
	out := &comatprototypes.RepoListRecords_Output{
		Records: []*comatprototypes.RepoListRecords_Record{},
	}

	root, err := atproto.LexiconRepo.GetRepoRoot(ctx, atproto.RepoUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo root: %w", err)
	}
	cs := atproto.LexiconRepo.CarStore()
	ses, err := cs.ReadOnlySession(atproto.RepoUser)
	if err != nil {
		return nil, fmt.Errorf("failed to create read-only session: %w", err)
	}
	r, err := repo.OpenRepo(ctx, ses, root)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	err = r.ForEach(ctx, collection, func(rpath string, cid cid.Cid) error {
		b, err := ses.Get(ctx, cid)
		if err != nil {
			return fmt.Errorf("failed to get block: %w", err)
		}
		var val cbg.CBORMarshaler
		if collection == "com.atproto.lexicon.schema" {
			sfMap, err := data.UnmarshalCBOR(b.RawData())
			if err != nil {
				return fmt.Errorf("failed to unmarshal schema file: %w", err)
			}
			jbs, err := json.Marshal(sfMap)
			if err != nil {
				return fmt.Errorf("failed to marshal schema file: %w", err)
			}
			sf := lexicon.SchemaFile{}
			err = json.Unmarshal(jbs, &sf)
			if err != nil {
				return fmt.Errorf("failed to unmarshal schema file: %w", err)
			}
			val = &atproto.SchemaFileWrapper{
				SchemaFile: sf,
			}
		} else {
			val, err = lexutil.CborDecodeValue(b.RawData())
			if err != nil {
				return fmt.Errorf("failed to decode record: %w", err)
			}
		}

		out.Records = append(out.Records, &comatprototypes.RepoListRecords_Record{
			Uri:   fmt.Sprintf("at://%s/%s", repoStr, rpath),
			Cid:   cid.String(),
			Value: &lexutil.LexiconTypeDecoder{Val: val},
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate through records: %w", err)
	}
	return out, nil
}

func (s *Server) handleComAtprotoRepoGetRecord(ctx context.Context, _ string, collection string, repo string, rkey string) (*comatprototypes.RepoGetRecord_Output, error) {
	var val cbg.CBORMarshaler
	outCID, val, err := atproto.LexiconRepo.GetRecord(ctx, atproto.RepoUser, collection, rkey, cid.Cid{})
	if err != nil {
		return nil, err
	}
	if collection == "com.atproto.lexicon.schema" {
		// repomgr doesn't support getting the raw blocks for a schema file, so we
		// have to get the proof and find the schema file in it.
		_, blks, err := atproto.LexiconRepo.GetRecordProof(ctx, atproto.RepoUser, collection, rkey)
		if err != nil {
			return nil, fmt.Errorf("failed to get record proof: %w", err)
		}
		var b blocks.Block
		for _, blk := range blks {
			if blk.Cid().Equals(outCID) {
				b = blk
				break
			}
		}
		if b == nil {
			return nil, fmt.Errorf("couldn't find schema file in merkle proof")
		}
		sfMap, err := data.UnmarshalCBOR(b.RawData())
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema file: %w", err)
		}
		jbs, err := json.Marshal(sfMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema file: %w", err)
		}
		sf := lexicon.SchemaFile{}
		err = json.Unmarshal(jbs, &sf)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema file: %w", err)
		}
		val = &atproto.SchemaFileWrapper{
			SchemaFile: sf,
		}
	}
	str := outCID.String()
	return &comatprototypes.RepoGetRecord_Output{
		Uri:   fmt.Sprintf("at://%s/%s/%s", repo, collection, rkey),
		Cid:   &str,
		Value: &lexutil.LexiconTypeDecoder{Val: val},
	}, nil
}
