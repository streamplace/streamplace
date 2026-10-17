package statedb

import (
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

// TestSQLiteBusyTimeout: the state database is the one two streamplace
// processes share -- the server, and a `streamplace sync` warming a new index
// revision -- so a writer that meets the other's lock has to wait rather than
// fail. The pragma is per-connection, so the only proof it took is asking the
// connection.
func TestSQLiteBusyTimeout(t *testing.T) {
	cli := config.CLI{DBURL: ":memory:"}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := MakeDB(t.Context(), &cli, nil, mod)
	require.NoError(t, err)

	var timeout int
	require.NoError(t, state.DB.Raw("PRAGMA busy_timeout").Scan(&timeout).Error)
	require.Equal(t, int(model.SQLiteBusyTimeout.Milliseconds()), timeout)
}

// TestPostgresConnectRetry pins the two halves of the backoff: a server that
// is still starting up (the CI flake this guards against) is waited out, while
// an error no amount of waiting can fix fails immediately instead of hanging
// the caller for the whole 30s budget.
func TestPostgresConnectRetry(t *testing.T) {
	startingUp := &pgconn.PgError{Code: "57P03", Message: "the database system is starting up"}

	t.Run("retries while starting up", func(t *testing.T) {
		attempts := 0
		err := retryPostgresConnect(t.Context(), time.Second, func() error {
			attempts++
			if attempts < 3 {
				return startingUp
			}
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 3, attempts)
	})

	t.Run("retries while the socket is closed", func(t *testing.T) {
		attempts := 0
		err := retryPostgresConnect(t.Context(), time.Second, func() error {
			attempts++
			if attempts < 2 {
				return fmt.Errorf("failed to connect: %w", syscall.ECONNREFUSED)
			}
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 2, attempts)
	})

	t.Run("fails fast on permanent errors", func(t *testing.T) {
		attempts := 0
		err := retryPostgresConnect(t.Context(), time.Second, func() error {
			attempts++
			return &pgconn.PgError{Code: "28P01", Message: "password authentication failed"}
		})
		require.Error(t, err)
		require.Equal(t, 1, attempts)
	})
}
