package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// TestValidateConfigCommand checks the registration, which is the part that
// is easy to get wrong: the command has to carry the server's flags so an
// operator can validate the exact same flag set the node would start with.
func TestValidateConfigCommand(t *testing.T) {
	cmd := makeValidateConfigCommand(&config.BuildFlags{Version: "test"})
	require.Equal(t, "validate-config", cmd.Name)
	require.NotNil(t, cmd.Action)

	names := map[string]bool{}
	for _, flag := range cmd.Flags {
		for _, name := range flag.Names() {
			names[name] = true
		}
	}
	require.True(t, names["data-dir"], "validate-config needs the data dir")
	require.True(t, names["vod-cdn-provider"], "validate-config needs the CDN flags")
	require.True(t, names["bunny-token-auth-key"], "validate-config needs the bunny flags")
}

// TestValidateConfigCommandRuns runs the command for real: a bad flag
// combination fails with the same error a starting node would die on, and a
// clean configuration passes without opening anything.
func TestValidateConfigCommandRuns(t *testing.T) {
	cmd := makeValidateConfigCommand(&config.BuildFlags{Version: "test"})
	err := cmd.Run(t.Context(), []string{"validate-config", "--data-dir", t.TempDir(),
		"--bunny-token-auth-key", "k"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--vod-cdn-provider=bunny")

	err = cmd.Run(t.Context(), []string{"validate-config", "--data-dir", t.TempDir(),
		"--livepeer-gateway"})
	require.NoError(t, err)

	err = cmd.Run(t.Context(), []string{"validate-config", "--data-dir", t.TempDir()})
	require.NoError(t, err)
}
