package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	urfavecli "github.com/urfave/cli/v3"
)

// flagRun builds a command the way `streamplace sync` is built -- the command
// an operator uses to warm an index -- and runs it with the given arguments,
// handing back the CLI the flags landed in.
func flagRun(t *testing.T, args ...string) *CLI {
	t.Helper()
	cli := &CLI{}
	cmd := cli.NewCommand("sync")
	cmd.Action = func(context.Context, *urfavecli.Command) error { return nil }
	require.NoError(t, cmd.Run(context.Background(), append([]string{"sync"}, args...)))
	return cli
}

// TestSweepIntervalFlag: how often a node re-checks every repo it indexes is an
// operator's decision -- and setting it to zero, which turns the periodic sweep
// off entirely, has to be expressible.
func TestSweepIntervalFlag(t *testing.T) {
	require.Equal(t, DefaultSweepInterval, flagRun(t).SweepInterval, "unset is the default")
	require.Equal(t, 90*time.Minute, flagRun(t, "--sweep-interval", "90m").SweepInterval)
	require.Equal(t, time.Duration(0), flagRun(t, "--sweep-interval=0").SweepInterval)

	t.Setenv("SP_SWEEP_INTERVAL", "2h")
	require.Equal(t, 2*time.Hour, flagRun(t).SweepInterval)
}

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
