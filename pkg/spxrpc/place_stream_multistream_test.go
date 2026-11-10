package spxrpc

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

func multistreamTestServer(t *testing.T) *Server {
	t.Helper()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(t.Context(), &config.CLI{DBURL: ":memory:"}, nil, mod)
	require.NoError(t, err)
	return &Server{statefulDB: state}
}

func TestMultistreamLegacyRPC(t *testing.T) {
	s := multistreamTestServer(t)
	ctx := context.WithValue(t.Context(), pdsAuthContextKey, "did:plc:multistreamrpc")
	input := &placestream.MultistreamCreateTarget_Input{MultistreamTarget: placestream.MultistreamTarget{
		Url: "rtmp://localhost/live/key", CreatedAt: "2026-11-09T00:00:00Z",
	}}
	created, err := s.handlePlaceStreamMultistreamCreateTarget(ctx, input)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(created.Uri, "at://did:plc:multistreamrpc/place.stream.multistream.target/"))
	rkey := created.Uri[strings.LastIndex(created.Uri, "/")+1:]
	name := "renamed"
	input.MultistreamTarget.Name = &name
	put := &placestream.MultistreamPutTarget_Input{Rkey: &rkey, MultistreamTarget: input.MultistreamTarget}
	other := context.WithValue(t.Context(), pdsAuthContextKey, "did:plc:other")
	_, err = s.handlePlaceStreamMultistreamPutTarget(other, put)
	require.Error(t, err)
	_, err = s.handlePlaceStreamMultistreamDeleteTarget(other, &placestream.MultistreamDeleteTarget_Input{Rkey: rkey})
	require.Error(t, err)
	updated, err := s.handlePlaceStreamMultistreamPutTarget(ctx, put)
	require.NoError(t, err)
	require.Equal(t, created.Uri, updated.Uri)
	listed, err := s.handlePlaceStreamMultistreamListTargets(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, listed.Targets, 1)
	require.Equal(t, name, *listed.Targets[0].Record.Val.(*placestream.MultistreamTarget).Name)
	_, err = s.handlePlaceStreamMultistreamDeleteTarget(ctx, &placestream.MultistreamDeleteTarget_Input{Rkey: rkey})
	require.NoError(t, err)
	listed, err = s.handlePlaceStreamMultistreamListTargets(ctx, "", 50)
	require.NoError(t, err)
	require.Empty(t, listed.Targets)
}
