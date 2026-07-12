package spxrpc

import (
	"context"
	"fmt"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/comatproto"
)

func (s *Server) handleComAtprotoServerDescribeServer(ctx context.Context) (*indigoatproto.ServerDescribeServer_Output, error) {
	did := fmt.Sprintf("did:web:%s", s.cli.BroadcasterHost)
	trueVar := true
	return &indigoatproto.ServerDescribeServer_Output{
		Did:                did,
		InviteCodeRequired: &trueVar,
		AvailableUserDomains: []string{
			fmt.Sprintf(".%s", s.cli.BroadcasterHost),
		},
	}, nil
}

func (s *Server) handleComAtprotoServerCreateSession(ctx context.Context, body *comatproto.ServerCreateSession_Input) (*comatproto.ServerCreateSession_Output, error) {
	return nil, echo.NewHTTPError(501, "not implemented")
}
