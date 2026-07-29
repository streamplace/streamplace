package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	urfavecli "github.com/urfave/cli/v3"
)

// TestSweepConcurrencyFlag: the sweep's host-lane cap is settable from the
// command line and the environment, and every command built from NewCommand --
// including `streamplace sync`, which is the one an operator uses to warm an
// index -- gets it.
func TestSweepConcurrencyFlag(t *testing.T) {
	run := func(t *testing.T, args ...string) *CLI {
		t.Helper()
		cli := &CLI{}
		cmd := cli.NewCommand("sync")
		cmd.Action = func(context.Context, *urfavecli.Command) error { return nil }
		require.NoError(t, cmd.Run(context.Background(), append([]string{"sync"}, args...)))
		return cli
	}

	require.Equal(t, DefaultSweepConcurrency, run(t).SweepConcurrency, "unset is the default")
	require.Equal(t, 8, run(t, "--sweep-concurrency", "8").SweepConcurrency)
	require.Equal(t, 8, run(t, "--sweep-concurrency=8").SweepConcurrency)

	t.Setenv("SP_SWEEP_CONCURRENCY", "12")
	require.Equal(t, 12, run(t).SweepConcurrency)
}
