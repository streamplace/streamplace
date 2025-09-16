package spxrpc

import (
	"context"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
)

func (s *Server) handleComAtprotoIdentityResolveHandle(ctx context.Context, handle string) (*comatprototypes.IdentityResolveHandle_Output, error) {
	did, err := oatproxy.ResolveHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	return &comatprototypes.IdentityResolveHandle_Output{Did: did}, nil
}

func (s *Server) handleComAtprotoIdentityRefreshIdentity(ctx context.Context, body *comatprototypes.IdentityRefreshIdentity_Input) (*comatprototypes.IdentityDefs_IdentityInfo, error) {
	ident, err := s.ATSync.RefreshIdentity(ctx, body.Identifier)
	if err != nil {
		return nil, err
	}
	return &comatprototypes.IdentityDefs_IdentityInfo{
		Did:    ident.DID.String(),
		Handle: ident.Handle.String(),
		DidDoc: ident.DIDDocument(),
	}, nil
}
