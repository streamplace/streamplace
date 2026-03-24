package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
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

	if cid == "" {
		return nil, fmt.Errorf("cid parameter required")
	}

	// Strip .m4s suffix appended for HLS player file extension sniffing
	cid = strings.TrimSuffix(cid, ".m4s")

	// Look up segment by BDASL CID
	data := mp.GetByCID(cid)
	if data == nil {
		return nil, fmt.Errorf("blob not found: %s", cid)
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

