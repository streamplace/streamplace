package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
)

func (s *Server) handlePlaceStreamPlaybackGetVideoPlaylist(ctx context.Context, did string, end *int, rkey string, start *int, track string) (io.Reader, error) {
	mp := s.mempools.Get(did)
	if mp == nil {
		return nil, fmt.Errorf("no active stream for %s", did)
	}

	// Master playlist if no track specified
	if track == "" {
		return bytes.NewReader([]byte(mp.MasterPlaylist(did, rkey))), nil
	}

	// Media playlist for a specific track
	var startMs, endMs int64
	if start != nil {
		startMs = int64(*start)
	}
	if end != nil {
		endMs = int64(*end)
	}

	playlist, err := mp.MediaPlaylist(track, startMs, endMs, did, rkey)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader([]byte(playlist)), nil
}

func (s *Server) handlePlaceStreamPlaybackGetVideoBlob(ctx context.Context, cid string, did string, rkey string) (io.Reader, error) {
	mp := s.mempools.Get(did)
	if mp == nil {
		return nil, fmt.Errorf("no active stream for %s", did)
	}

	// If no CID, this is a request for the init segment
	if cid == "" {
		initData := mp.InitData()
		if initData == nil {
			return nil, fmt.Errorf("no init segment available")
		}
		return bytes.NewReader(initData), nil
	}

	// CID format: "{trackID}/{segmentNumber}.m4s" — parse it as a segment reference
	// This matches the URIs we emit in the media playlists
	trackID, segNum, err := parseSegmentCID(cid)
	if err != nil {
		return nil, fmt.Errorf("invalid segment reference %q: %w", cid, err)
	}

	data := mp.TrackSegmentData(trackID, segNum)
	if data == nil {
		return nil, fmt.Errorf("segment %s/%d not found", trackID, segNum)
	}
	return bytes.NewReader(data), nil
}

func (s *Server) handlePlaceStreamPlaybackGetInitSegment(ctx context.Context, did string, rkey string, track string) (io.Reader, error) {
	mp := s.mempools.Get(did)
	if mp == nil {
		return nil, fmt.Errorf("no active stream for %s", did)
	}

	data := mp.TrackInitData(track)
	if data == nil {
		return nil, fmt.Errorf("no init segment for track %s", track)
	}
	return bytes.NewReader(data), nil
}

// parseSegmentCID parses a segment reference like "1/0.m4s" into trackID and segment number.
func parseSegmentCID(ref string) (string, int, error) {
	// Strip .m4s suffix if present
	cleaned := ref
	if len(cleaned) > 4 && cleaned[len(cleaned)-4:] == ".m4s" {
		cleaned = cleaned[:len(cleaned)-4]
	}

	// Find the slash separator
	for i := len(cleaned) - 1; i >= 0; i-- {
		if cleaned[i] == '/' {
			trackID := cleaned[:i]
			segStr := cleaned[i+1:]
			segNum, err := strconv.Atoi(segStr)
			if err != nil {
				return "", 0, fmt.Errorf("invalid segment number %q", segStr)
			}
			return trackID, segNum, nil
		}
	}
	return "", 0, fmt.Errorf("expected trackID/segmentNumber format")
}
