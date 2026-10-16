package spxrpc

import (
	"context"
	"fmt"

	placestreamtypes "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamBroadcastGetBroadcaster(ctx context.Context) (*placestreamtypes.BroadcastGetBroadcaster_Output, error) {
	broadcaster := fmt.Sprintf("did:web:%s", s.cli.BroadcasterHost)
	server := fmt.Sprintf("did:web:%s", s.cli.ServerHost)
	out := &placestreamtypes.BroadcastGetBroadcaster_Output{
		Broadcaster: broadcaster,
		Server:      &server,
		Admins:      s.cli.AdminDIDs,
	}
	// Asked for on every dial by a peer pushing streams here (--peers):
	// the URL carries a self-signed certificate's hash, which rotates.
	if s.MoqURL != nil {
		if u := s.MoqURL(); u != "" {
			out.MoqURL = &u
		}
	}
	return out, nil
}
