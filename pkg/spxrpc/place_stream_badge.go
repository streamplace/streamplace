package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/badges"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamBadgeGetValidBadges(ctx context.Context, streamer string) (*placestream.BadgeGetValidBadges_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	badgeList, err := badges.GetValidBadges(ctx, session.DID, streamer, s.cli.BroadcasterDID(), s.model)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to get valid badges")
	}

	return &placestream.BadgeGetValidBadges_Output{
		Badges: badgeList,
	}, nil
}

func (s *Server) handlePlaceStreamBadgeGetIssuedBadges(ctx context.Context, streamer string) (*placestream.BadgeGetIssuedBadges_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	userDID := session.DID

	// Compute server badge (streamer/mod/bot) for the given channel context.
	var serverBadge *placestream.BadgeDefs_BadgeView
	if streamer != "" {
		computed, err := badges.GetValidBadges(ctx, userDID, streamer, s.cli.BroadcasterDID(), s.model)
		if err != nil {
			log.Error(ctx, "failed to compute server badges", "err", err)
		}
		// Take the first server badge (streamer, mod, or bot). These are always first in the list.
		for _, b := range computed {
			if b.BadgeType == constants.BadgeTypeStreamer || b.BadgeType == constants.BadgeTypeMod || b.BadgeType == constants.BadgeTypeBot {
				serverBadge = &b
				break
			}
		}
	}

	// Build a set of currently selected issuance URIs.
	selectedURIs := map[string]bool{}
	chatProfile, err := s.model.GetChatProfile(ctx, userDID)
	if err != nil {
		log.Error(ctx, "failed to get chat profile", "err", err)
	}
	if chatProfile != nil && chatProfile.Badges != nil {
		for _, sel := range chatProfile.Badges.Streamer {
			if sel.Badge.Uri != "" {
				selectedURIs[sel.Badge.Uri] = true
			}
		}
		if chatProfile.Badges.Global != nil {
			selectedURIs[chatProfile.Badges.Global.Uri] = true
		}
	}

	// Fetch all issuances granted to this user.
	issuances, err := s.model.GetBadgeIssuancesForRecipient(ctx, userDID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch badge issuances")
	}

	streamerSlot := placestream.BadgeDefs_BadgeSlot{
		Available: []placestream.BadgeDefs_BadgeIssuanceView{},
	}
	userSlot := placestream.BadgeDefs_BadgeSlot{
		Available: []placestream.BadgeDefs_BadgeIssuanceView{},
	}

	for _, issuance := range issuances {
		def, err := s.model.GetBadgeDefByURI(ctx, issuance.BadgeURI)
		if err != nil {
			log.Error(ctx, "failed to get badge def", "err", err, "uri", issuance.BadgeURI)
			continue
		}
		if def == nil {
			continue // def was deleted
		}

		view := placestream.BadgeDefs_BadgeIssuanceView{
			IssuanceUri: issuance.URI,
			IssuanceCid: &issuance.CID,
			BadgeType:   def.BadgeType,
			Issuer:      issuance.RepoDID,
		}
		if def.Name != "" {
			view.Name = &def.Name
		}
		if def.Description != "" {
			view.Description = &def.Description
		}
		if def.ImageCID != "" {
			imageUrl := fmt.Sprintf("https://cdn.bsky.app/img/feed_fullsize/plain/%s/%s@png", def.RepoDID, def.ImageCID)
			view.ImageUrl = &imageUrl
		}
		isSelected := selectedURIs[issuance.URI]
		view.Selected = &isSelected

		if badges.IsGlobalIssuer(issuance.RepoDID) {
			userSlot.Available = append(userSlot.Available, view)
			if isSelected && userSlot.Selected == nil {
				userSlot.Selected = &view
			}
		} else {
			streamerSlot.Available = append(streamerSlot.Available, view)
			if isSelected && streamerSlot.Selected == nil {
				streamerSlot.Selected = &view
			}
		}
	}

	return &placestream.BadgeGetIssuedBadges_Output{
		Server:   serverBadge,
		Streamer: streamerSlot,
		User:     userSlot,
	}, nil
}
