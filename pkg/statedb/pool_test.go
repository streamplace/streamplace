package statedb

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// Needs a Postgres: SP_TEST_POSTGRES_URL=postgres://user:pass@host/ (the
// database named in the URL is created if missing).
func TestPostgresPoolBounded(t *testing.T) {
	dbURL := os.Getenv("SP_TEST_POSTGRES_URL")
	if dbURL == "" {
		t.Skip("SP_TEST_POSTGRES_URL not set")
	}
	state, err := MakeDB(context.Background(), &config.CLI{DBURL: dbURL, DBMaxOpenConns: 12}, nil, nil)
	require.NoError(t, err)
	sqlDB, err := state.DB.DB()
	require.NoError(t, err)
	stats := sqlDB.Stats()
	require.Equal(t, 12, stats.MaxOpenConnections)
	require.LessOrEqual(t, stats.OpenConnections, 12)

	// The default applies when the flag is unset, and a silly value is
	// raised to leave room beside the lock connection.
	state, err = MakeDB(context.Background(), &config.CLI{DBURL: dbURL}, nil, nil)
	require.NoError(t, err)
	sqlDB, _ = state.DB.DB()
	require.Equal(t, 30, sqlDB.Stats().MaxOpenConnections)
	state, err = MakeDB(context.Background(), &config.CLI{DBURL: dbURL, DBMaxOpenConns: 1}, nil, nil)
	require.NoError(t, err)
	sqlDB, _ = state.DB.DB()
	require.Equal(t, 4, sqlDB.Stats().MaxOpenConnections)
}
