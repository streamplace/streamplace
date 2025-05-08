package spxrpc

import (
	"context"

	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamGraphGetFollowingUser(ctx context.Context, subjectDID string) (*placestreamtypes.GraphGetFollowingUser_Output, error) {
	// this is where following check needs to be implemented
	return nil, nil
}
