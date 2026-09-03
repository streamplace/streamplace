package spxrpc

import (
	"context"
	"fmt"

	placestreamtypes "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamBroadcastGetBroadcaster(ctx context.Context) (*placestreamtypes.BroadcastGetBroadcaster_Output, error) {
	broadcaster := fmt.Sprintf("did:web:%s", s.cli.BroadcasterHost)
	server := fmt.Sprintf("did:web:%s", s.cli.ServerHost)
	// The admin roster is no longer published here: clients learn their own
	// roles from place.stream.access.getStatus instead.
	return &placestreamtypes.BroadcastGetBroadcaster_Output{
		Broadcaster: broadcaster,
		Server:      &server,
	}, nil
}
