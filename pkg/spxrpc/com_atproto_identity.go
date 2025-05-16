package spxrpc

import (
	"context"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"stream.place/streamplace/pkg/oproxy"
)

func (s *Server) handleComAtprotoIdentityResolveHandle(ctx context.Context, handle string) (*comatprototypes.IdentityResolveHandle_Output, error) {
	did, err := oproxy.ResolveHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	return &comatprototypes.IdentityResolveHandle_Output{Did: did}, nil
}
