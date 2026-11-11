package statedb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces/spacestest"
)

const moveDID = "did:plc:mover"

// editOnPut edits a target's row while the move is copying it.
type editOnPut struct {
	spacestest.Fake
	edit func()
}

func (c *editOnPut) Do(ctx context.Context, kind, inpenc, method string, params map[string]any, body, out any) error {
	if c.edit != nil && method == "com.atproto.space.putRecord" {
		c.edit()
		c.edit = nil
	}
	return c.Fake.Do(ctx, kind, inpenc, method, params, body, out)
}

func moveTestTargets(t *testing.T, n int) (*StatefulDB, []string) {
	t.Helper()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := MakeDB(t.Context(), &config.CLI{DBURL: ":memory:"}, nil, mod)
	require.NoError(t, err)
	uris := make([]string, n)
	for i := range uris {
		view, err := state.CreateMultistreamTarget(placestream.MultistreamCreateTarget_Input{MultistreamTarget: placestream.MultistreamTarget{
			Url: "rtmp://localhost/live/key", CreatedAt: "2026-11-11T00:00:00Z",
		}}, moveDID)
		require.NoError(t, err)
		uris[i] = view.Uri
	}
	return state, uris
}

func rowCount(t *testing.T, state *StatefulDB) int {
	t.Helper()
	rows, err := state.ListMultistreamTargets(moveDID, 100, 0, nil)
	require.NoError(t, err)
	return len(rows)
}

func TestMoveMultistreamTargets(t *testing.T) {
	t.Run("moves every row under its own rkey", func(t *testing.T) {
		state, uris := moveTestTargets(t, 2)
		space := &spacestest.Fake{}
		require.NoError(t, state.MoveMultistreamTargets(t.Context(), space, moveDID))
		require.Zero(t, rowCount(t, state))
		require.Len(t, space.Records, 2)
		for _, uri := range uris {
			require.Contains(t, space.Records, uri[strings.LastIndex(uri, "/")+1:])
		}
		// Nothing is left to move, so the PDS is not asked again.
		calls := len(space.Calls)
		require.NoError(t, state.MoveMultistreamTargets(t.Context(), space, moveDID))
		require.Len(t, space.Calls, calls)
	})

	t.Run("keeps every row when the PDS fails", func(t *testing.T) {
		state, _ := moveTestTargets(t, 2)
		failure := errors.New("PDS unavailable")
		require.ErrorIs(t, state.MoveMultistreamTargets(t.Context(), &spacestest.Fake{Err: failure}, moveDID), failure)
		require.Equal(t, 2, rowCount(t, state))
	})

	t.Run("keeps a row edited while it moved", func(t *testing.T) {
		state, uris := moveTestTargets(t, 1)
		space := &editOnPut{edit: func() {
			_, err := state.UpdateMultistreamTarget(uris[0], placestream.MultistreamPutTarget_Input{MultistreamTarget: placestream.MultistreamTarget{
				Url: "rtmp://localhost/live/edited", CreatedAt: "2026-11-11T00:00:00Z",
			}})
			require.NoError(t, err)
		}}
		require.NoError(t, state.MoveMultistreamTargets(t.Context(), space, moveDID))
		require.Equal(t, 1, rowCount(t, state))
		// The next move carries the edit over.
		require.NoError(t, state.MoveMultistreamTargets(t.Context(), space, moveDID))
		require.Zero(t, rowCount(t, state))
		require.Contains(t, string(space.Records[uris[0][strings.LastIndex(uris[0], "/")+1:]]), "edited")
	})
}
