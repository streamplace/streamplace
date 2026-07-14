package spxrpc

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/comatproto"
)

func (s *Server) handleComAtprotoServerDescribeServer(ctx context.Context) (*comatproto.ServerDescribeServer_Output, error) {
	did := fmt.Sprintf("did:web:%s", s.cli.BroadcasterHost)
	trueVar := true
	return &comatproto.ServerDescribeServer_Output{
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
