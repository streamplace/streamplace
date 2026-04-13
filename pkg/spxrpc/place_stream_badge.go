package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/badges"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/streamplace"
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
				serverBadge = b
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
	if chatProfile != nil {
		spcp, err := chatProfile.ToStreamplaceChatProfile()
		if err == nil && spcp != nil {
			for _, ref := range spcp.Selection {
				if ref != nil {
					selectedURIs[ref.Uri] = true
				}
			}
		}
	}

	// Fetch all issuances granted to this user.
	issuances, err := s.model.GetBadgeIssuancesForRecipient(ctx, userDID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch badge issuances")
	}

	streamerSlot := &placestream.BadgeDefs_BadgeSlot{
		Available: []*placestream.BadgeDefs_BadgeIssuanceView{},
	}
	userSlot := &placestream.BadgeDefs_BadgeSlot{
		Available: []*placestream.BadgeDefs_BadgeIssuanceView{},
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

		view := &placestream.BadgeDefs_BadgeIssuanceView{
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
				userSlot.Selected = view
			}
		} else {
			streamerSlot.Available = append(streamerSlot.Available, view)
			if isSelected && streamerSlot.Selected == nil {
				streamerSlot.Selected = view
			}
		}
	}

	return &placestream.BadgeGetIssuedBadges_Output{
		Server:   serverBadge,
		Streamer: streamerSlot,
		User:     userSlot,
	}, nil
}

func (s *Server) handlePlaceStreamBadgeIssueBadge(ctx context.Context, input *placestream.BadgeIssueBadge_Input) (*placestream.BadgeIssueBadge_Output, error) {
	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil || client == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Create the badge definition record in the issuer's repo.
	def := &placestream.BadgeDef{
		LexiconTypeID: "place.stream.badge.def",
		Name:          input.Name,
		BadgeType:     input.BadgeType,
		CreatedAt:     now,
		Description:   input.Description,
	}

	defInput := comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_BADGE_DEF,
		Record:     &lexutil.LexiconTypeDecoder{Val: def},
		Repo:       session.DID,
	}
	defOutput := comatproto.RepoCreateRecord_Output{}
	err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, defInput, &defOutput)
	if err != nil {
		log.Error(ctx, "failed to create badge def record", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create badge def: %v", err))
	}

	// Create the badge issuance record referencing the new def.
	issuance := &placestream.BadgeIssuance{
		LexiconTypeID: "place.stream.badge.issuance",
		Did:           input.RecipientDid,
		Badge: &comatproto.RepoStrongRef{
			Uri: defOutput.Uri,
			Cid: defOutput.Cid,
		},
		CreatedAt: now,
	}

	issuanceInput := comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_BADGE_ISSUANCE,
		Record:     &lexutil.LexiconTypeDecoder{Val: issuance},
		Repo:       session.DID,
	}
	issuanceOutput := comatproto.RepoCreateRecord_Output{}
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, issuanceInput, &issuanceOutput)
	if err != nil {
		log.Error(ctx, "failed to create badge issuance record", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create badge issuance: %v", err))
	}

	return &placestream.BadgeIssueBadge_Output{
		DefUri:      defOutput.Uri,
		DefCid:      defOutput.Cid,
		IssuanceUri: issuanceOutput.Uri,
		IssuanceCid: issuanceOutput.Cid,
	}, nil
}
