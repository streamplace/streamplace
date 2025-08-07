package cmd

import (
	"context"
	"flag"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/peterbourgon/ff/v3"
)

func GoLivepeer(ctx context.Context) error {
	fs := flag.NewFlagSet("streamplace", flag.ExitOnError)
	cfg := starter.NewLivepeerConfig(fs)

	// Config file
	_ = flag.String("config", "", "Config file in the format 'key value', flags and env vars take precedence over the config file")
	err := ff.Parse(fs, []string{},
		ff.WithConfigFileFlag("config"),
		ff.WithEnvVarPrefix("LP"),
		ff.WithConfigFileParser(ff.PlainParser),
	)
	if err != nil {
		glog.Exit("Error parsing config: ", err)
	}

	cfg = starter.UpdateNilsForUnsetFlags(cfg)

	starter.StartLivepeer(ctx, cfg)

	return nil
}
