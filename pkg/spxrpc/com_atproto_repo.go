package spxrpc

import (
	"context"
	"fmt"
	"io"
	"net/http"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
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

func (s *Server) handleComAtprotoRepoListRecords(ctx context.Context, collection string, cursor string, limit int, repo string, reverse *bool) (*comatprototypes.RepoListRecords_Output, error) {
	r, ses, err := atproto.OpenLexiconRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoListRecords: failed to open repo: %w", err)
	}
	out := &comatprototypes.RepoListRecords_Output{
		Records: []*comatprototypes.RepoListRecords_Record{},
	}
	err = r.ForEach(ctx, "", func(rkey string, c cid.Cid) error {
		val, err := atproto.GetRecordCBOR(ctx, ses, c, collection, rkey)
		if err != nil {
			return fmt.Errorf("handleComAtprotoRepoListRecords: failed to get record for collection %q, rkey %q: %w", collection, rkey, err)
		}
		out.Records = append(out.Records, &comatprototypes.RepoListRecords_Record{
			Uri:   fmt.Sprintf("at://%s/%s/%s", repo, collection, rkey),
			Cid:   c.String(),
			Value: &lexutil.LexiconTypeDecoder{Val: val},
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoListRecords: error iterating records for collection %q: %w", collection, err)
	}
	return out, nil
}

func (s *Server) handleComAtprotoRepoGetRecord(ctx context.Context, c string, collection string, repo string, rkey string) (*comatprototypes.RepoGetRecord_Output, error) {
	r, ses, err := atproto.OpenLexiconRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to open repo: %w", err)
	}
	outCID, _, err := r.GetRecord(ctx, fmt.Sprintf("%s/%s", collection, rkey))
	if err != nil {
		return nil, err
	}
	rec, err := atproto.GetRecordCBOR(ctx, ses, outCID, collection, rkey)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to get record: %w", err)
	}
	str := outCID.String()
	return &comatprototypes.RepoGetRecord_Output{
		Uri:   fmt.Sprintf("at://%s/%s/%s", repo, collection, rkey),
		Cid:   &str,
		Value: &lexutil.LexiconTypeDecoder{Val: rec},
	}, nil
}
