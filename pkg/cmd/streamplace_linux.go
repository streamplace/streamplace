//go:build linux

package cmd

import (
	"context"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/proc"
)

func runMist(ctx context.Context, cli *config.CLI) error {
	if cli.NoMist {
		<-ctx.Done()
		return nil
	}
	return proc.RunMistServer(ctx, cli)
}

func Start(build *config.BuildFlags) error {
	return start(build, []jobFunc{runMist})
}
