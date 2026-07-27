package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// TestSyncCommand checks the registration, which is the part that is easy to
// get wrong: the command has to carry the server's flags (its own --data-dir
// and --db-url, resolved into the same CLI the action reads) rather than being
// a bare subcommand that runs against defaults.
func TestSyncCommand(t *testing.T) {
	cmd := makeSyncCommand(&config.BuildFlags{Version: "test"})
	require.Equal(t, "sync", cmd.Name)
	require.NotNil(t, cmd.Action)

	names := map[string]bool{}
	for _, flag := range cmd.Flags {
		for _, name := range flag.Names() {
			names[name] = true
		}
	}
	require.True(t, names["data-dir"], "sync needs the data dir to find the index")
	require.True(t, names["db-url"], "sync needs the state database")
	require.True(t, names["plc-url"], "sync resolves identities")
}

// TestSyncCommandRuns runs the command for real against empty databases: it
// opens the index and the state database, sweeps the zero repos in them, and
// exits successfully. No HTTP server is started and no network is touched,
// which is the point of the command.
func TestSyncCommandRuns(t *testing.T) {
	cmd := makeSyncCommand(&config.BuildFlags{Version: "test"})
	err := cmd.Run(t.Context(), []string{"sync", "--data-dir", t.TempDir(), "--db-url", ":memory:"})
	require.NoError(t, err)
}
