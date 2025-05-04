package spxrpc

import (
	"context"

	"stream.place/streamplace/pkg/atproto"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamAccountLogin(ctx context.Context, body *placestreamtypes.AccountLogin_Input) (*placestreamtypes.AccountDefs_LoginResponse, error) {
	return atproto.Login(ctx, s.cli, body)
}
