package spxrpc

import (
	"context"
	"fmt"
	"io"
	"strings"
)

func (s *Server) handlePlaceStreamPlaybackGetVideoPlaylist(ctx context.Context, did string, end *int, rkey string, start *int, track string) (io.Reader, error) {
	// TODO: look up mempool by DID, generate playlist
	return strings.NewReader(""), fmt.Errorf("not yet implemented")
}

func (s *Server) handlePlaceStreamPlaybackGetVideoBlob(ctx context.Context, cid string, did string, rkey string) (io.Reader, error) {
	// TODO: look up mempool by DID, serve segment data or byte range
	return strings.NewReader(""), fmt.Errorf("not yet implemented")
}
