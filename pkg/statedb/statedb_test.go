package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

// run the inner testing function against all databases we support
func WithAllDatabases(t *testing.T, f func(*StatefulDB)) {
	t.Run("sqlite", func(t *testing.T) {
		cli := config.CLI{
			DBURL: ":memory:",
		}
		mod, err := model.MakeDB(":memory:")
		require.NoError(t, err)
		state, err := MakeDB(t.Context(), &cli, nil, mod)
		require.NoError(t, err)
		f(state)
	})
	if postgresURL == "" {
		t.Log("no postgres url, skipping postgres tests")
		return
	} else {
		t.Run("postgres", func(t *testing.T) {
			dburl := makePostgresURL(t)
			cli := config.CLI{
				DBURL: dburl,
			}
			mod, err := model.MakeDB(":memory:")
			require.NoError(t, err)
			state, err := MakeDB(t.Context(), &cli, nil, mod)
			require.NoError(t, err)
			f(state)
			sqlDB, err := state.DB.DB()
			require.NoError(t, err)

			// Close
			sqlDB.Close()
		})
	}
}
