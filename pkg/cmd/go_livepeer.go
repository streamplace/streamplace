package cmd

import (
	"context"
	"flag"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/peterbourgon/ff/v3"
)

func GoLivepeer(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("streamplace", flag.ExitOnError)
	cfg := starter.NewLivepeerConfig(fs)

	err := flag.Set("logtostderr", "true")
	if err != nil {
		return err
	}
	vFlag := flag.Lookup("v")
	err = vFlag.Value.Set("3")
	if err != nil {
		return err
	}

	// Config file
	err = ff.Parse(fs, args,
		ff.WithConfigFileFlag("config"),
		ff.WithEnvVarPrefix("LP"),
	)
	if err != nil {
		glog.Exit("Error parsing config: ", err)
	}

	cfg = starter.UpdateNilsForUnsetFlags(cfg)

	starter.StartLivepeer(ctx, cfg)

	return nil
}
