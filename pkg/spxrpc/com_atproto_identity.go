package spxrpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/comatproto"
)

func (s *Server) handleComAtprotoIdentityResolveHandle(ctx context.Context, handle string) (*comatproto.IdentityResolveHandle_Output, error) {
	did, err := oatproxy.ResolveHandleWithClient(ctx, handle, &aqhttp.Client)
	if err != nil {
		return nil, err
	}
	return &comatproto.IdentityResolveHandle_Output{Did: did}, nil
}

func (s *Server) handleComAtprotoIdentityResolveIdentity(ctx context.Context, identifier string) (*comatproto.IdentityDefs_IdentityInfo, error) {
	id, err := syntax.ParseAtIdentifier(identifier)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidRequest").WithInternal(err)
	}
	ident, err := s.identityDirectory().Lookup(ctx, id)
	if err != nil {
		if errors.Is(err, identity.ErrDIDNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "DidNotFound").WithInternal(err)
		}
		if errors.Is(err, identity.ErrHandleNotFound) ||
			errors.Is(err, identity.ErrHandleReservedTLD) ||
			errors.Is(err, identity.ErrInvalidHandle) ||
			errors.Is(err, identity.ErrHandleMismatch) ||
			errors.Is(err, identity.ErrHandleNotDeclared) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "HandleNotFound").WithInternal(err)
		}
		return nil, fmt.Errorf("resolve identity: %w", err)
	}
	didDoc, err := glex.Unknown(ident.DIDDocument())
	if err != nil {
		return nil, fmt.Errorf("encode DID document: %w", err)
	}
	return &comatproto.IdentityDefs_IdentityInfo{
		Did:    ident.DID.String(),
		Handle: ident.Handle.String(),
		DidDoc: didDoc,
	}, nil
}

func (s *Server) handleComAtprotoIdentityRefreshIdentity(ctx context.Context, body *comatproto.IdentityRefreshIdentity_Input) (*comatproto.IdentityDefs_IdentityInfo, error) {
	ident, err := s.ATSync.RefreshIdentity(ctx, body.Identifier)
	if err != nil {
		return nil, err
	}
	didDoc, err := glex.Unknown(ident.DIDDocument())
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to encode DID document")
	}
	return &comatproto.IdentityDefs_IdentityInfo{
		Did:    ident.DID.String(),
		Handle: ident.Handle.String(),
		DidDoc: didDoc,
	}, nil
}

func (s *Server) handleComAtprotoIdentityUpdateHandle(ctx context.Context, body *comatproto.IdentityUpdateHandle_Input) error {
	return echo.NewHTTPError(501, "not implemented")
}
