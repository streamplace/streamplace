package spxrpc

import (
	"container/list"
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"stream.place/streamplace/pkg/linking"
	"stream.place/streamplace/pkg/vod"
)

const vodStarterBudget = 512 * 1024
const vodPartsCacheSize = 64

type vodParts struct {
	cid         string
	meta        *vod.Metafile
	initOnce    sync.Once
	inits       map[string]string
	initErr     error
	starterOnce sync.Once
	starter     *linking.VODStarter
	starterErr  error
}

type vodPartsCache struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	lru     list.List
}

func (s *Server) videoParts(ctx context.Context, cid string) (*vodParts, error) {
	cache := &s.vodCache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if el := cache.entries[cid]; el != nil {
		cache.lru.MoveToFront(el)
		return el.Value.(*vodParts), nil
	}
	meta, err := s.fetchMetafile(ctx, cid)
	if err != nil {
		return nil, err
	}
	parts := &vodParts{cid: cid, meta: meta}
	if cache.entries == nil {
		cache.entries = make(map[string]*list.Element)
	}
	cache.entries[cid] = cache.lru.PushFront(parts)
	if cache.lru.Len() > vodPartsCacheSize {
		el := cache.lru.Back()
		delete(cache.entries, el.Value.(*vodParts).cid)
		cache.lru.Remove(el)
	}
	return parts, nil
}

func (s *Server) cachedMetafile(ctx context.Context, cid string) (*vod.Metafile, error) {
	parts, err := s.videoParts(ctx, cid)
	if err != nil {
		return nil, err
	}
	return parts.meta, nil
}

func (s *Server) discardVideoParts(parts *vodParts) {
	cache := &s.vodCache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if el := cache.entries[parts.cid]; el != nil && el.Value == parts {
		delete(cache.entries, parts.cid)
		cache.lru.Remove(el)
	}
}

func (s *Server) readPlaybackBytes(ctx context.Context, cid string, offset, size int64) ([]byte, error) {
	r, err := s.playbackStore.Open(ctx, vod.BlobsPrefix+cid+".mp4")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	if size < 0 {
		size = r.Size()
	}
	if offset < 0 || size < 0 || offset > r.Size() || size > r.Size()-offset {
		return nil, fmt.Errorf("playback range outside blob %s", cid)
	}
	data := make([]byte, size)
	if n, err := r.ReadAt(data, offset); n < len(data) {
		return nil, fmt.Errorf("reading playback blob %s: %w", cid, err)
	}
	return data, nil
}

func (s *Server) loadInits(ctx context.Context, parts *vodParts) error {
	parts.initOnce.Do(func() {
		parts.inits = make(map[string]string, len(parts.meta.Tracks))
		for id, track := range parts.meta.Tracks {
			data, err := s.readPlaybackBytes(ctx, track.InitCID, 0, -1)
			if err != nil {
				parts.initErr = err
				return
			}
			parts.inits[id] = base64.StdEncoding.EncodeToString(data)
		}
	})
	if parts.initErr != nil {
		s.discardVideoParts(parts)
	}
	return parts.initErr
}

func budgetStarter(video, audio string, key, audioData, rest []byte) *linking.VODStarter {
	remaining := vodStarterBudget
	take := func(data []byte) *string {
		if len(data) == 0 || len(data) > remaining {
			return nil
		}
		remaining -= len(data)
		encoded := base64.StdEncoding.EncodeToString(data)
		return &encoded
	}
	out := &linking.VODStarter{Video: linking.VODStarterVideo{Track: video, Key: take(key)}}
	if audio != "" {
		out.Audio = &linking.VODStarterAudio{Track: audio, Data: take(audioData)}
	}
	out.Video.Rest = take(rest)
	return out
}

func starterAllowed(playback *videoPlayback, hasTime bool) bool {
	return !hasTime && (playback.start == nil || *playback.start == 0)
}

func (s *Server) loadStarter(ctx context.Context, parts *vodParts) (*linking.VODStarter, error) {
	parts.starterOnce.Do(func() {
		videos := videoTrackIDs(parts.meta)
		if len(videos) == 0 {
			return
		}
		videoID := videos[0]
		video := parts.meta.Tracks[videoID]
		if len(video.Segments) == 0 {
			return
		}
		readFirst := func(track vod.MetafileTrack) ([]byte, error) {
			if len(track.Segments) == 0 {
				return nil, nil
			}
			seg := track.Segments[0]
			return s.readPlaybackBytes(ctx, track.BlobCID, parts.meta.FlatHeaderSize+seg.Offset, seg.Size)
		}
		data, err := readFirst(video)
		if err != nil {
			parts.starterErr = err
			return
		}
		key, rest, err := vod.SplitFirstSample(data)
		if err != nil {
			parts.starterErr = err
			return
		}
		audioID := pickDefaultAudio(parts.meta, mainAudioIDs(parts.meta, tracksOfType(parts.meta, "audio")))
		var audio []byte
		if audioID != "" {
			audio, err = readFirst(parts.meta.Tracks[audioID])
			if err != nil {
				parts.starterErr = err
				return
			}
		}
		parts.starter = budgetStarter(videoID, audioID, key, audio, rest)
	})
	if parts.starterErr != nil {
		s.discardVideoParts(parts)
	}
	return parts.starter, parts.starterErr
}
