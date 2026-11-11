package director

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spaces/spacestest"
	"stream.place/streamplace/pkg/statedb"
)

func TestRefreshMultistreamTargets(t *testing.T) {
	const did = "did:plc:streamer"
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(t.Context(), &config.CLI{DBURL: ":memory:"}, nil, mod)
	require.NoError(t, err)
	ss := &StreamSession{repoDID: did, statefulDB: state, cli: &config.CLI{}}
	target := func(active bool) *placestream.MultistreamTarget {
		return &placestream.MultistreamTarget{Url: "rtmp://localhost/live/key", CreatedAt: "2026-11-11T00:00:00Z", Active: active}
	}

	t.Run("statedb holds the targets of a streamer with no space session", func(t *testing.T) {
		_, err := state.CreateMultistreamTarget(placestream.MultistreamCreateTarget_Input{MultistreamTarget: *target(true)}, did)
		require.NoError(t, err)
		targets := &multistreamTargets{}
		ss.refreshMultistreamTargets(t.Context(), targets)
		require.Len(t, targets.active, 1)
		require.False(t, targets.inSpace)
	})

	t.Run("the space is read again only when its revision changes", func(t *testing.T) {
		space := &spacestest.Fake{}
		put := func(rkey string, active bool) {
			_, _, err := spaces.PutRecord(t.Context(), space, did, spaces.TargetCollection, rkey, target(active))
			require.NoError(t, err)
		}
		put("off", false)
		for i := range statedb.MAX_ACTIVE_MULTISTREAM_TARGETS + 1 {
			put(fmt.Sprintf("on%d", i), true)
		}
		targets := &multistreamTargets{space: space, spaceUntil: time.Now().Add(time.Hour)}
		ss.refreshMultistreamTargets(t.Context(), targets)
		require.True(t, targets.inSpace)
		require.Len(t, targets.active, statedb.MAX_ACTIVE_MULTISTREAM_TARGETS)
		require.Equal(t, spaces.TargetURI(did, "on0"), targets.active[0].Uri)

		space.Calls = nil
		ss.refreshMultistreamTargets(t.Context(), targets)
		require.Equal(t, []string{"com.atproto.space.getLatestCommit"}, space.Calls)

		require.NoError(t, spaces.DeleteRecord(t.Context(), space, did, spaces.TargetCollection, "on0"))
		ss.refreshMultistreamTargets(t.Context(), targets)
		require.Equal(t, spaces.TargetURI(did, "on1"), targets.active[0].Uri)

		// A PDS that cannot be read leaves the targets running, not statedb's in their place.
		space.Err = errors.New("PDS unavailable")
		ss.refreshMultistreamTargets(t.Context(), targets)
		ss.refreshMultistreamTargets(t.Context(), targets)
		require.Len(t, targets.active, statedb.MAX_ACTIVE_MULTISTREAM_TARGETS)
		require.Equal(t, spaces.TargetURI(did, "on1"), targets.active[0].Uri)
	})
}
