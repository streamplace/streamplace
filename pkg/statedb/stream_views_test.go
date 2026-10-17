package statedb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Counts are kept per livestream record and survive the streamer moving to
// a new record; the streamer's "current" total is the last one counted into.
func TestStreamViewTotalsPerLivestream(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		first, second := "at://did:plc:me/place.stream.livestream/first", "at://did:plc:me/place.stream.livestream/second"
		for i := 0; i < 3; i++ {
			_, err := state.AddStreamView(ctx, "did:plc:me", first)
			require.NoError(t, err)
		}
		total, uri, err := state.GetStreamViewTotal(ctx, "did:plc:me")
		require.NoError(t, err)
		require.Equal(t, int64(3), total)
		require.Equal(t, first, uri)

		time.Sleep(5 * time.Millisecond) // updated_at orders the "current" row
		n, err := state.AddStreamView(ctx, "did:plc:me", second)
		require.NoError(t, err)
		require.Equal(t, int64(1), n, "a new record starts its own count")
		total, uri, err = state.GetStreamViewTotal(ctx, "did:plc:me")
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, second, uri)

		totals, err := state.LivestreamViewTotals(ctx, []string{first, second, "at://did:plc:me/place.stream.livestream/never"})
		require.NoError(t, err)
		require.Equal(t, map[string]int64{first: 3, second: 1}, totals, "the first record's count is still there")

		none, err := state.LivestreamViewTotals(ctx, nil)
		require.NoError(t, err)
		require.Empty(t, none)
	})
}

// A node upgrading from the per-streamer row carries it over once, and a
// count that has moved on is not overwritten by the legacy value.
func TestStreamViewTotalsMigration(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		uri := "at://did:plc:me/place.stream.livestream/live"
		require.NoError(t, state.DB.Create(&StreamViewTotal{StreamerDID: "did:plc:me", LivestreamURI: uri, Views: 40, UpdatedAt: time.Now()}).Error)
		require.NoError(t, state.DB.Create(&StreamViewTotal{StreamerDID: "did:plc:none", LivestreamURI: "", Views: 7, UpdatedAt: time.Now()}).Error)

		require.NoError(t, migrateStreamViewTotals(ctx, state.DB))
		totals, err := state.LivestreamViewTotals(ctx, []string{uri})
		require.NoError(t, err)
		require.Equal(t, int64(40), totals[uri])
		total, _, err := state.GetStreamViewTotal(ctx, "did:plc:none")
		require.NoError(t, err)
		require.Equal(t, int64(0), total, "a row for no record is not carried")

		_, err = state.AddStreamView(ctx, "did:plc:me", uri)
		require.NoError(t, err)
		require.NoError(t, migrateStreamViewTotals(ctx, state.DB), "a second startup")
		totals, err = state.LivestreamViewTotals(ctx, []string{uri})
		require.NoError(t, err)
		require.Equal(t, int64(41), totals[uri], "not reset to the legacy 40")
	})
}
