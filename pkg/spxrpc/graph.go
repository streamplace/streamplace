package spxrpc

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/comatproto"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamGraphGetFollowingUser(ctx context.Context, subjectDID string, userDID string) (*placestream.GraphGetFollowingUser_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamGraphGetFollowingUser")
	defer span.End()

	_, didErr := syntax.ParseDID(userDID)
	if userDID == "" || didErr != nil {
		return nil, fmt.Errorf("missing or invalid user DID")
	}

	follow, err := s.model.GetUserFollowingUser(ctx, userDID, subjectDID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user following: %w", err)
	}

	output := placestream.GraphGetFollowingUser_Output{}
	if follow != nil {
		output.Follow = &comatproto.RepoStrongRef{
			Cid: "", // We don't store CID in our model
			Uri: fmt.Sprintf("at://%s/app.appbsky.graph.follow/%s", userDID, follow.RKey),
		}
	}

	return &output, nil
}
