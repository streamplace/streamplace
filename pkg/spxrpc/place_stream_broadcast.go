package spxrpc

import (
	"context"
	"fmt"

	placestreamtypes "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamBroadcastGetBroadcaster(ctx context.Context) (*placestreamtypes.BroadcastGetBroadcaster_Output, error) {
	broadcaster := fmt.Sprintf("did:web:%s", s.cli.BroadcasterHost)
	server := fmt.Sprintf("did:web:%s", s.cli.ServerHost)
	return &placestreamtypes.BroadcastGetBroadcaster_Output{
		Broadcaster: broadcaster,
		Server:      &server,
		Admins:      s.cli.AdminDIDs,
	}, nil
}
