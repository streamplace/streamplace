package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/model"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) buildPackView(ctx context.Context, pack *model.EmotePack) (*placestreamtypes.EmoteDefs_PackView, error) {
	repo, err := s.model.GetRepo(pack.RepoDID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}

	items, err := s.model.GetEmoteItemsByPack(ctx, pack.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get emote items: %w", err)
	}

	emotes := make([]*placestreamtypes.EmoteDefs_EmoteView, 0, len(items))
	for _, item := range items {
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
			emoteView.ImageUrl = fmt.Sprintf("https://cdn.bsky.app/img/feed_fullsize/plain/%s/%s@png", item.RepoDID, item.ImageCID)
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

func (s *Server) handlePlaceStreamEmoteGetEmotePacks(ctx context.Context) (*placestreamtypes.EmoteGetEmotePacks_Output, error) {
	packs, err := s.model.GetAllEmotePacks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get emote packs: %w", err)
	}

	packViews := make([]*placestreamtypes.EmoteDefs_PackView, 0, len(packs))
	for _, pack := range packs {
		view, err := s.buildPackView(ctx, pack)
		if err != nil {
			return nil, err
		}
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

	packView, err := s.buildPackView(ctx, pack)
	if err != nil {
		return nil, err
	}

	return &placestreamtypes.EmoteGetEmotePack_Output{Pack: packView}, nil
}
