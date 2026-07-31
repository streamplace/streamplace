package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/indexdb"
)

// run the inner testing function against all databases we support
func WithAllDatabases(t *testing.T, f func(*StatefulDB)) {
	WithAllDatabasesAndModel(t, func(state *StatefulDB, _ indexdb.Model) {
		f(state)
	})
}

// WithAllDatabasesAndModel is WithAllDatabases for tests that also seed
// the index database: the callback gets the concrete indexdb.Model that
// the StatefulDB was built with (state.model is deliberately narrower).
func WithAllDatabasesAndModel(t *testing.T, f func(*StatefulDB, indexdb.Model)) {
	t.Run("sqlite", func(t *testing.T) {
		cli := config.CLI{
			DBURL: ":memory:",
		}
		mod, err := indexdb.MakeDB(":memory:")
		require.NoError(t, err)
		state, err := MakeDB(t.Context(), &cli, nil, mod)
		require.NoError(t, err)
		f(state, mod)
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
			mod, err := indexdb.MakeDB(":memory:")
			require.NoError(t, err)
			state, err := MakeDB(t.Context(), &cli, nil, mod)
			require.NoError(t, err)
			f(state, mod)
			sqlDB, err := state.DB.DB()
			require.NoError(t, err)

			// Close
			sqlDB.Close()
		})
	}
}
