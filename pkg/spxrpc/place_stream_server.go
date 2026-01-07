package spxrpc

import (
	"context"
	"time"

	"github.com/bluesky-social/indigo/util"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamServerGetServerTime(ctx context.Context) (*placestreamtypes.ServerGetServerTime_Output, error) {
	serverTime := time.Now().UTC().Format(util.ISO8601)
	return &placestreamtypes.ServerGetServerTime_Output{
		ServerTime: serverTime,
	}, nil
}
