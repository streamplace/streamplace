package spxrpc

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"go.opentelemetry.io/otel"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamGraphGetFollowingUser(ctx context.Context, subjectDID string, userDID string) (*placestreamtypes.GraphGetFollowingUser_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamGraphGetFollowingUser")
	defer span.End()

	_, didErr := syntax.ParseDID(userDID)
	if userDID == "" || didErr != nil {
		return nil, fmt.Errorf("Missing or invalid user DID")
	}

	follow, err := s.model.GetUserFollowingUser(ctx, userDID, subjectDID)
	if err != nil {
		return nil, fmt.Errorf("Failed to get user following: %w", err)
	}

	output := &placestreamtypes.GraphGetFollowingUser_Output{}
	if follow != nil {
		output.Follow = &atproto.RepoStrongRef{
			Cid: "", // We don't store CID in our model
			Uri: fmt.Sprintf("at://%s/app.bsky.graph.follow/%s", userDID, follow.RKey),
		}
	}

	return output, nil
}
