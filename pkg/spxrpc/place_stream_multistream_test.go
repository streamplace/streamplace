package spxrpc

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces/spacestest"
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

func TestMultistreamSpaceTargets(t *testing.T) {
	s := multistreamTestServer(t)
	ctx := t.Context()
	did := "did:plc:multistreamspace"
	space := &spacestest.Fake{}
	target := func(active bool) *placestream.MultistreamTarget {
		return &placestream.MultistreamTarget{Url: "rtmp://localhost/live/key", CreatedAt: "2026-11-11T00:00:00Z", Active: active}
	}

	_, err := putSpaceTarget(ctx, space, did, "missing", target(false))
	require.ErrorContains(t, err, "not found")

	var views []*placestream.MultistreamDefs_TargetView
	for range statedb.MAX_ACTIVE_MULTISTREAM_TARGETS {
		view, err := putSpaceTarget(ctx, space, did, "", target(true))
		require.NoError(t, err)
		views = append(views, view)
	}
	_, err = putSpaceTarget(ctx, space, did, "", target(true))
	require.ErrorContains(t, err, "active")
	inactive, err := putSpaceTarget(ctx, space, did, "", target(false))
	require.NoError(t, err)
	rkey := func(view *placestream.MultistreamDefs_TargetView) string {
		return view.Uri[strings.LastIndex(view.Uri, "/")+1:]
	}
	// One more cannot be switched on, but an active one can still be edited.
	_, err = putSpaceTarget(ctx, space, did, rkey(inactive), target(true))
	require.ErrorContains(t, err, "active")
	_, err = putSpaceTarget(ctx, space, did, rkey(views[0]), target(true))
	require.NoError(t, err)

	// The list is in rkey order, pages by offset, and carries statedb's events.
	require.NoError(t, s.statefulDB.CreateMultistreamEvent(views[1].Uri, "connected", "active"))
	listed, err := s.listSpaceTargets(ctx, space, did, 2, 1)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, views[1].Uri, listed[0].Uri)
	require.Equal(t, "active", listed[0].LatestEvent.Status)
	require.Equal(t, views[2].Uri, listed[1].Uri)
	require.Nil(t, listed[1].LatestEvent)
	// Nothing reached statedb's own table.
	rows, err := s.statefulDB.ListMultistreamTargets(did, 100, 0, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
}
