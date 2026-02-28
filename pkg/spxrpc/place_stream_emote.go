package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamEmoteGetEmotePack(ctx context.Context, uri string) (*placestreamtypes.EmoteGetEmotePack_Output, error) {
	pack, err := s.model.GetEmotePackByURI(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get emote pack: %w", err)
	}
	if pack == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "emote pack not found")
	}

	repo, err := s.model.GetRepo(pack.RepoDID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}

	items, err := s.model.GetEmoteItemsByPack(ctx, uri)
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
		if item.ImageCID != "" && repo != nil && repo.PDS != "" {
			imageUrl := fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s", repo.PDS, item.RepoDID, item.ImageCID)
			emoteView.ImageUrl = imageUrl
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

	packView := &placestreamtypes.EmoteDefs_PackView{
		Uri:       pack.URI,
		Cid:       pack.CID,
		Name:      pack.Name,
		Emotes:    emotes,
		IndexedAt: pack.IndexedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Author: &bsky.ActorDefs_ProfileViewBasic{
			Did:    did,
			Handle: handle,
		},
	}

	return &placestreamtypes.EmoteGetEmotePack_Output{Pack: packView}, nil
}
