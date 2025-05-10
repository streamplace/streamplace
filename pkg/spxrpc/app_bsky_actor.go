package spxrpc

import (
	"context"
	"fmt"

	appbskytypes "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

func (s *Server) handleAppBskyActorGetProfile(ctx context.Context, actor string) (*appbskytypes.ActorDefs_ProfileViewDetailed, error) {
	session := atproto.GetOAuthSession(ctx)
	if session == nil {
		log.Error(ctx, "oauth session not found")
		return nil, fmt.Errorf("oauth session not found")
	}
	key, err := jwk.ParseKey(session.UpstreamDPoPPrivateJWK)
	if err != nil {
		log.Error(ctx, "failed to parse DPoP private JWK", "error", err)
		return nil, fmt.Errorf("failed to parse DPoP private JWK: %w", err)
	}
	authArgs := &oauth.XrpcAuthedRequestArgs{
		Did:            session.RepoDID,
		AccessToken:    session.UpstreamAccessToken,
		PdsUrl:         session.PDSUrl,
		Issuer:         session.UpstreamAuthServerIssuer,
		DpopPdsNonce:   session.UpstreamDPoPNonce,
		DpopPrivateJwk: key,
	}

	xc := atproto.GetXrpcClient(s.model)

	// brief check to make sure we can actually do stuff
	var out appbskytypes.ActorDefs_ProfileViewDetailed
	err = xc.Do(ctx, authArgs, xrpc.Query, "application/json", "app.bsky.actor.getProfile", map[string]any{"actor": actor}, nil, &out)
	if err != nil {
		log.Error(ctx, "failed to get profile", "error", err)
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return &out, nil
}
