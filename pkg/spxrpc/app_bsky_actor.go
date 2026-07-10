package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	appbskytypes "stream.place/streamplace/pkg/appbsky"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
)

func bskyCDNURL(kind, did, cid string) *string {
	ret := fmt.Sprintf("https://cdn.appbsky.app/img/%s/plain/%s/%s@jpeg", kind, did, cid)
	return &ret
}

func (s *Server) handleAppBskyActorGetProfile(ctx context.Context, actor string) (*appbskytypes.ActorDefs_ProfileViewDetailed, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	repo, err := s.ATSync.SyncBlueskyRepoCached(ctx, actor)
	if err != nil {
		return nil, err
	}

	if session.DID != repo.DID {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "this endpoint only implemented for the current user")
	}

	profile, err := s.model.GetBskyProfile(ctx, repo.DID, false)
	if err != nil {
		return nil, err
	}

	// user valid but we couldn't find their bsky profile, return a minimum thing
	if false {
		return &appbskytypes.ActorDefs_ProfileViewDetailed{
			Did:    repo.DID,
			Handle: repo.Handle,
		}, nil
	}

	out := &appbskytypes.ActorDefs_ProfileViewDetailed{
		Did:         repo.DID,
		Handle:      repo.Handle,
		DisplayName: profile.DisplayName,
		Description: profile.Description,
	}

	if profile.Banner != nil {
		out.Banner = bskyCDNURL("banner", repo.DID, profile.Banner.Ref.String())
	}

	if profile.Avatar != nil {
		out.Avatar = bskyCDNURL("avatar", repo.DID, profile.Avatar.Ref.String())
	}

	return out, nil
}
