package spxrpc

import (
	"context"
	"fmt"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
)

func (s *Server) handleComAtprotoServerDescribeServer(ctx context.Context) (*comatprototypes.ServerDescribeServer_Output, error) {
	did := fmt.Sprintf("did:web:%s", s.cli.BroadcasterHost)
	trueVar := true
	return &comatprototypes.ServerDescribeServer_Output{
		Did:                did,
		InviteCodeRequired: &trueVar,
		AvailableUserDomains: []string{
			fmt.Sprintf(".%s", s.cli.BroadcasterHost),
		},
	}, nil
}
