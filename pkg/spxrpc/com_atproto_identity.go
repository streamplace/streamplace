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
