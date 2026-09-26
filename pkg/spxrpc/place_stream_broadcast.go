package spxrpc

import (
	"context"
	"fmt"

	"stream.place/streamplace/pkg/branding"

	placestreamtypes "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamBroadcastGetBroadcaster(ctx context.Context) (*placestreamtypes.BroadcastGetBroadcaster_Output, error) {
	broadcaster := fmt.Sprintf("did:web:%s", s.cli.BroadcasterHost)
	server := fmt.Sprintf("did:web:%s", s.cli.ServerHost)
	// The brand follows the hostname asked on: a custom domain is branded
	// (and its branding changed) by its owner, anything else by the node.
	brand, domain := branding.ResolveHost(s.statefulDB, s.cli.BroadcasterHost, branding.RequestHost(ctx))
	brandAdmins := s.cli.AdminDIDs
	if domain != nil {
		brandAdmins = []string{domain.OwnerDID}
	}
	return &placestreamtypes.BroadcastGetBroadcaster_Output{
		Broadcaster: broadcaster,
		Server:      &server,
		Admins:      s.cli.AdminDIDs,
		Brand:       &brand,
		BrandAdmins: brandAdmins,
	}, nil
}
