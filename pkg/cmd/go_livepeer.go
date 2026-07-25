package cmd

import (
	"context"
	"flag"

	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"stream.place/streamplace/pkg/config"
)

func GoLivepeer(ctx context.Context, fs *flag.FlagSet) error {
	err := flag.Set("logtostderr", "true")
	if err != nil {
		return err
	}
	vFlag := flag.Lookup("v")
	err = vFlag.Value.Set("3")
	if err != nil {
		return err
	}

	config.LivepeerConfig = starter.UpdateNilsForUnsetFlags(config.LivepeerConfig)

	starter.StartLivepeer(ctx, config.LivepeerConfig)

	return nil
}
