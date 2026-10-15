package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

// WithAllDatabases runs the inner testing function against all databases we
// support: an in-process SQLite, a Postgres server, and a SlateDB database in
// an object store. The last two run when the environment names one (see
// harness_test.go), which is how a laptop runs the first and CI runs all
// three.
func WithAllDatabases(t *testing.T, f func(*StatefulDB)) {
	t.Run("sqlite", func(t *testing.T) {
		f(makeTestDB(t, ":memory:"))
	})
	t.Run("postgres", func(t *testing.T) {
		if postgresURL == "" {
			t.Skip("no postgres url, skipping postgres tests")
		}
		f(makeTestDB(t, makePostgresURL(t)))
	})
	t.Run("slatedb", func(t *testing.T) {
		stateURL := makeSlateDBURL(t)
		if stateURL == "" {
			t.Skip("no object store, skipping slatedb tests")
		}
		f(makeTestDB(t, stateURL))
	})
}

// TestRedactDBURL: database URLs are logged as they are opened and refused, so
// the credentials a URL carries have to be stripped first. Postgres keeps them
// in the userinfo, and an object store's run in the query string.
func TestRedactDBURL(t *testing.T) {
	require.Equal(t,
		"postgresql://user:redacted@localhost:5432/state",
		redactDBURL("postgresql://user:hunter2@localhost:5432/state"))

	redacted := redactDBURL("slatedb+s3://bucket/state?endpoint=http://localhost:9000&access_key_id=key&secret_access_key=hunter2&session_token=toksecret")
	require.NotContains(t, redacted, "hunter2")
	require.NotContains(t, redacted, "toksecret")
	require.Contains(t, redacted, "access_key_id=key")
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
