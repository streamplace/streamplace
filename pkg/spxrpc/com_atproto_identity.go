package spxrpc

import (
	"context"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/aqhttp"
)

func (s *Server) handleComAtprotoIdentityResolveHandle(ctx context.Context, handle string) (*indigoatproto.IdentityResolveHandle_Output, error) {
	did, err := oatproxy.ResolveHandleWithClient(ctx, handle, &aqhttp.Client)
	if err != nil {
		return nil, err
	}
	return &indigoatproto.IdentityResolveHandle_Output{Did: did}, nil
}

func (s *Server) handleComAtprotoIdentityRefreshIdentity(ctx context.Context, body *indigoatproto.IdentityRefreshIdentity_Input) (*indigoatproto.IdentityDefs_IdentityInfo, error) {
	ident, err := s.ATSync.RefreshIdentity(ctx, body.Identifier)
	if err != nil {
		return nil, err
	}
	return &indigoatproto.IdentityDefs_IdentityInfo{
		Did:    ident.DID.String(),
		Handle: ident.Handle.String(),
		DidDoc: ident.DIDDocument(),
	}, nil
}
