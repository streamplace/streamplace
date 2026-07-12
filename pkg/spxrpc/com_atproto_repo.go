package spxrpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	glex "github.com/streamplace/glex/runtime"

	"stream.place/streamplace/pkg/comatproto"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

func resolveRepoService(ctx context.Context, repo string) (string, string, string, error) {
	did := repo
	var err error
	if !strings.HasPrefix(repo, "did:") {
		did, err = oatproxy.ResolveHandleWithClient(ctx, repo, &aqhttp.Client)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to resolve handle %q: %w", repo, err)
		}
	}

	service, handle, err := oatproxy.ResolveServiceWithClient(ctx, did, &aqhttp.Client)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to resolve service for did %q: %w", did, err)
	}

	return did, service, handle, nil
}

var maxBlobSize int64 = 1024 * 1024 * 10 // 10MB

func (s *Server) handleComAtprotoRepoUploadBlob(ctx context.Context, r io.Reader, contentType string) (*comatproto.RepoUploadBlob_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handleComAtprotoRepoUploadBlob")
	defer span.End()

	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// we need to buffer the blob so we can successfully retry upon dpop nonce changes
	var err error
	buf := bytes.Buffer{}
	_, err = io.CopyN(&buf, r, maxBlobSize+1)
	if err == nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "blob size exceeds max size")
	}
	if !errors.Is(err, io.EOF) {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to copy reader to buffer")
	}

	var out comatproto.RepoUploadBlob_Output

	err = client.Do(ctx, xrpc.Procedure, contentType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(buf.Bytes()), &out)

	if err != nil {
		log.Error(ctx, "upstream xrpc error", "error", err)
		return nil, err
	}

	return &out, nil
}

func (s *Server) handleComAtprotoRepoDescribeRepo(ctx context.Context, repo string) (*comatproto.RepoDescribeRepo_Output, error) {
	isLocal, svc, err := s.isLocalPDS(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error checking for local PDS: %w", err)
	}
	if !isLocal {
		var out comatproto.RepoDescribeRepo_Output
		params := make(map[string]interface{})
		params["repo"] = repo

		err = makeUnauthenticatedRequest(ctx, svc, "com.atproto.repo.describeRepo", params, &out)
		if err != nil {
			log.Error(ctx, "upstream xrpc error", "error", err)
			return nil, err
		}
		return &out, nil
	}

	if s.isServerPDS(ctx) {
		collections, err := atproto.ServerRepoListCollections(ctx)
		if err != nil {
			return nil, fmt.Errorf("list server repo collections: %w", err)
		}
		didDoc, err := glex.RawJSON(atproto.DIDDoc(s.cli.ServerHost, atproto.ServerPubMultibase))
		if err != nil {
			return nil, fmt.Errorf("marshal did doc: %w", err)
		}
		return &comatproto.RepoDescribeRepo_Output{
			Handle:          s.cli.ServerDID(),
			Did:             s.cli.ServerDID(),
			DidDoc:          &glex.LexiconTypeDecoder{Val: didDoc},
			Collections:     collections,
			HandleIsCorrect: true,
		}, nil
	}

	didDoc, err := glex.RawJSON(atproto.DIDDoc(s.cli.BroadcasterHost, atproto.LexiconPubMultibase))
	if err != nil {
		return nil, fmt.Errorf("marshal did doc: %w", err)
	}
	return &comatproto.RepoDescribeRepo_Output{
		Handle: s.cli.BroadcasterDID(),
		Did:    s.cli.BroadcasterDID(),
		DidDoc: &glex.LexiconTypeDecoder{Val: didDoc},
		Collections: []string{
			"com.atproto.lexicon.schema",
		},
		HandleIsCorrect: true,
	}, nil
}

func (s *Server) handleComAtprotoRepoListRecords(ctx context.Context, collection string, cursor string, limit int, repo string, reverse bool) (*comatproto.RepoListRecords_Output, error) {
	isLocal, svc, err := s.isLocalPDS(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error checking for local PDS: %w", err)
	}
	if !isLocal {
		var out comatproto.RepoListRecords_Output
		params := make(map[string]interface{})
		params["collection"] = collection
		if cursor != "" {
			params["cursor"] = cursor
		}
		if limit != 0 {
			params["limit"] = limit
		}
		params["reverse"] = reverse
		params["repo"] = repo

		err = makeUnauthenticatedRequest(ctx, svc, "com.atproto.repo.listRecords", params, &out)
		if err != nil {
			log.Error(ctx, "upstream xrpc error", "error", err)
			return nil, err
		}
		return &out, nil
	}

	if s.isServerPDS(ctx) {
		return atproto.ServerRepoListRecords(ctx, collection, cursor, limit, repo, &reverse)
	}
	return atproto.LexiconRepoListRecords(ctx, collection, cursor, limit, repo, &reverse)
}

func (s *Server) handleComAtprotoRepoGetRecord(ctx context.Context, c string, collection string, repo string, rkey string) (*comatproto.RepoGetRecord_Output, error) {
	isLocal, svc, err := s.isLocalPDS(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error checking for local PDS: %w", err)
	}
	if !isLocal {
		var out comatproto.RepoGetRecord_Output
		params := make(map[string]interface{})
		params["repo"] = repo
		params["collection"] = collection
		params["rkey"] = rkey
		if c != "" {
			params["cid"] = c
		}

		err = makeUnauthenticatedRequest(ctx, svc, "com.atproto.repo.getRecord", params, &out)
		if err != nil {
			log.Error(ctx, "upstream xrpc error", "error", err)
			return nil, err
		}
		return &out, nil
	}

	if s.isServerPDS(ctx) {
		return atproto.ServerRepoGetRecord(ctx, repo, collection, rkey)
	}
	return atproto.LexiconRepoGetRecord(ctx, repo, collection, rkey)
}

func (s *Server) handleComAtprotoRepoCreateRecord(ctx context.Context, body *comatproto.RepoCreateRecord_Input) (*comatproto.RepoCreateRecord_Output, error) {
	return nil, echo.NewHTTPError(501, "not implemented")
}

func (s *Server) handleComAtprotoRepoDeleteRecord(ctx context.Context, body *comatproto.RepoDeleteRecord_Input) (*comatproto.RepoDeleteRecord_Output, error) {
	return nil, echo.NewHTTPError(501, "not implemented")
}

func (s *Server) handleComAtprotoRepoPutRecord(ctx context.Context, body *comatproto.RepoPutRecord_Input) (*comatproto.RepoPutRecord_Output, error) {
	return nil, echo.NewHTTPError(501, "not implemented")
}
