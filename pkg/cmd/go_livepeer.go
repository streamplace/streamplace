package cmd

import (
	"context"
	"flag"
	"strings"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"stream.place/streamplace/pkg/config"
)

func GoLivepeer(ctx context.Context, fs *flag.FlagSet) error {
	lpfs := flag.NewFlagSet("livepeer", flag.ExitOnError)
	cfg := starter.NewLivepeerConfig(lpfs)
	fs.VisitAll(func(f *flag.Flag) {
		if !strings.HasPrefix(f.Name, "livepeer.") {
			return
		}
		name := strings.TrimPrefix(f.Name, "livepeer.")
		adapted := config.LivepeerFlags.SnakeToCamel[name]

		if adapted == "" {
			panic("unknown livepeer flag: " + name)
		}
		err := lpfs.Set(adapted, f.Value.String())
		if err != nil {
			panic(err)
		}
	})

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
	// err = ff.Parse(fs, args,
	// 	ff.WithConfigFileFlag("config"),
	// 	ff.WithEnvVarPrefix("SP_LIVEPEER"),
	// )
	if err != nil {
		glog.Exit("Error parsing config: ", err)
	}

	cfg = starter.UpdateNilsForUnsetFlags(cfg)

	starter.StartLivepeer(ctx, cfg)

	return nil
}
