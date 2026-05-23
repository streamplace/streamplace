package spxrpc

import (
	"context"

	placestream "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamConfigGetEnv(ctx context.Context) (*placestream.ConfigGetEnv_Output, error) {
	out := &placestream.ConfigGetEnv_Output{}
	if s.cli.PlaybackWorkerURL != "" {
		out.PlaybackWorkerUrl = &s.cli.PlaybackWorkerURL
	}
	if s.cli.GamesAPIURL != "" {
		t := true
		out.GamesEnabled = &t
	}
	return out, nil
}
