package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/model"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

// buildPackView constructs a PackView for the given pack. allowedEmotes, if non-nil,
// restricts which emote items are included to those whose URI is in the set.
func (s *Server) buildPackView(ctx context.Context, pack *model.EmotePack, allowedEmotes map[string]bool) (*placestreamtypes.EmoteDefs_PackView, error) {
	repo, err := s.model.GetRepo(pack.RepoDID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}

	items, err := s.model.GetEmoteItemsByPack(ctx, pack.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get emote items: %w", err)
	}

	uris := make([]string, len(items))
	for i, item := range items {
		uris[i] = item.URI
	}
	labelsByURI, err := s.model.GetActiveLabelsBatch(uris)
	if err != nil {
		return nil, fmt.Errorf("failed to get labels for pack %s: %w", pack.URI, err)
	}

	emotes := make([]*placestreamtypes.EmoteDefs_EmoteView, 0, len(items))
	for _, item := range items {
		// if we have any labels, skip this emote
		if labelsByURI[item.URI] != nil {
			continue
		}
		// if a specific emote set is specified, skip emotes not in it
		if allowedEmotes != nil && !allowedEmotes[item.URI] {
			continue
		}
		emoteView := &placestreamtypes.EmoteDefs_EmoteView{
			Uri:       item.URI,
			Cid:       item.CID,
			Name:      item.Name,
			IndexedAt: item.IndexedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if item.Alt != "" {
			emoteView.Alt = &item.Alt
		}
		if item.ImageCID != "" {
			// TODO: flag for CDN
			emoteView.ImageUrl = fmt.Sprintf("https://cdn.bsky.app/img/feed_fullsize/plain/%s/%s@png", item.RepoDID, item.ImageCID)
		}
		if item.CreatorDID != "" {
			did := item.CreatorDID
			emoteView.Creator = &did
		}
		emotes = append(emotes, emoteView)
	}

	var handle string
	var did string
	if repo != nil {
		handle = repo.Handle
		did = repo.DID
	} else {
		did = pack.RepoDID
	}

	return &placestreamtypes.EmoteDefs_PackView{
		Uri:       pack.URI,
		Cid:       pack.CID,
		Name:      pack.Name,
		Emotes:    emotes,
		IndexedAt: pack.IndexedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Author: &bsky.ActorDefs_ProfileViewBasic{
			Did:    did,
			Handle: handle,
		},
	}, nil
}

func (s *Server) handlePlaceStreamEmoteGetEmotePacks(ctx context.Context, streamer string) (*placestreamtypes.EmoteGetEmotePacks_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return &placestreamtypes.EmoteGetEmotePacks_Output{Packs: nil}, nil
	}

	packViews := make([]*placestreamtypes.EmoteDefs_PackView, 0)

	// Source 1: streamer's packs with openInMyChat=true, if viewer follows the streamer.
	follows, err := s.model.GetUserFollowing(ctx, session.DID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user follows: %w", err)
	}
	followsStreamer := false
	for _, f := range follows {
		if f.SubjectDID == streamer {
			followsStreamer = true
			break
		}
	}
	if followsStreamer {
		openPacks, err := s.model.GetStreamerOpenPacks(ctx, streamer)
		if err != nil {
			return nil, fmt.Errorf("failed to get streamer open packs: %w", err)
		}
		for _, pack := range openPacks {
			view, err := s.buildPackView(ctx, pack, nil)
			if err != nil {
				return nil, err
			}
			rel := "follow"
			view.Relationship = &rel
			packViews = append(packViews, view)
		}
	}

	// Source 2: packs delegated to this viewer globally.
	delegated, err := s.model.GetDelegatedPacksForUser(ctx, session.DID)
	if err != nil {
		return nil, fmt.Errorf("failed to get delegated packs: %w", err)
	}
	for _, dp := range delegated {
		allowed, err := dp.Delegation.AllowedEmoteSet()
		if err != nil {
			return nil, fmt.Errorf("failed to parse delegation for pack %s: %w", dp.Pack.URI, err)
		}
		view, err := s.buildPackView(ctx, dp.Pack, allowed)
		if err != nil {
			return nil, err
		}
		rel := "delegation"
		view.Relationship = &rel
		packViews = append(packViews, view)
	}

	return &placestreamtypes.EmoteGetEmotePacks_Output{Packs: packViews}, nil
}

func (s *Server) handlePlaceStreamEmoteGetEmotePack(ctx context.Context, uri string) (*placestreamtypes.EmoteGetEmotePack_Output, error) {
	pack, err := s.model.GetEmotePackByURI(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get emote pack: %w", err)
	}
	if pack == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "emote pack not found")
	}

	packView, err := s.buildPackView(ctx, pack, nil)
	if err != nil {
		return nil, err
	}

	return &placestreamtypes.EmoteGetEmotePack_Output{Pack: packView}, nil
}
