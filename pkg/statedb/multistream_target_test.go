package statedb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestMoveMultistreamTargets(t *testing.T) {
	target := func(url string) placestream.MultistreamTarget {
		return placestream.MultistreamTarget{Url: url, CreatedAt: "2026-11-11T00:00:00Z"}
	}
	WithAllDatabases(t, func(state *StatefulDB) {
		rows := func() int {
			rows, err := state.ListMultistreamTargets(moveDID, 100, 0, nil)
			require.NoError(t, err)
			return len(rows)
		}
		var rkeys []string
		for range 2 {
			view, err := state.CreateMultistreamTarget(placestream.MultistreamCreateTarget_Input{MultistreamTarget: target("rtmp://localhost/live/key")}, moveDID)
			require.NoError(t, err)
			rkeys = append(rkeys, view.Uri[strings.LastIndex(view.Uri, "/")+1:])
		}

		// A PDS failure moves nothing.
		failure := errors.New("PDS unavailable")
		require.ErrorIs(t, state.MoveMultistreamTargets(t.Context(), &spacestest.Fake{Err: failure}, moveDID), failure)
		require.Equal(t, 2, rows())

		// Both rows are copied under their rkeys; the one edited meanwhile stays.
		space := &editOnPut{edit: func() {
			uri := "at://" + moveDID + "/place.stream.multistream.target/" + rkeys[0]
			_, err := state.UpdateMultistreamTarget(uri, placestream.MultistreamPutTarget_Input{MultistreamTarget: target("rtmp://localhost/live/edited")})
			require.NoError(t, err)
		}}
		require.NoError(t, state.MoveMultistreamTargets(t.Context(), space, moveDID))
		require.Equal(t, 1, rows())
		require.Contains(t, space.Records, rkeys[1])

		// The next move carries the edit over, and the one after has nothing to do.
		require.NoError(t, state.MoveMultistreamTargets(t.Context(), space, moveDID))
		require.Zero(t, rows())
		require.Contains(t, string(space.Records[rkeys[0]]), "edited")
		calls := len(space.Calls)
		require.NoError(t, state.MoveMultistreamTargets(t.Context(), space, moveDID))
		require.Len(t, space.Calls, calls)
	})
}
