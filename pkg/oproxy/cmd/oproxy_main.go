package main

import (
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/peterbourgon/ff/v3"
	"stream.place/streamplace/pkg/oproxy"
)

func main() {
	err := Run()
	if err != nil {
		slog.Error("exited uncleanly", "error", err)
		os.Exit(1)
	}
}

func Run() error {
	flag.Set("logtostderr", "true")
	fs := flag.NewFlagSet("oproxy", flag.ExitOnError)
	noColor := fs.Bool("no-color", false, "disable colorized logging")
	host := fs.String("host", "", "public HTTPS address where this OAuth provider is hosted (ex example.com, no https:// prefix)")
	dbPath := fs.String("db", "", "path to the database file or postgres connection string")
	verbose := fs.Bool("v", false, "enable verbose logging")
	scope := fs.String("scope", "atproto transition:generic", "scope to use for the OAuth provider")
	clientMetadata := fs.String("client-metadata", "", "JSON client metadata or path to JSON file containing client metadata")
	// version := fs.Bool("version", false, "print version and exit")

	err := ff.Parse(
		fs, os.Args[1:],
		ff.WithEnvVarPrefix("OPROXY"),
	)
	if err != nil {
		panic(err)
	}
	err = flag.CommandLine.Parse(nil)
	if err != nil {
		panic(err)
	}

	opts := &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.RFC3339,
		NoColor:    *noColor,
	}
	if *verbose {
		opts.Level = slog.LevelDebug
	}
	logger := slog.New(
		tint.NewHandler(os.Stderr, opts),
	)

	slog.SetDefault(logger)

	store, err := NewStore(*dbPath)
	if err != nil {
		return err
	}

	var meta *oproxy.OAuthClientMetadata
	if (*clientMetadata)[0] != '{' {
		// path
		bs, err := os.ReadFile(*clientMetadata)
		if err != nil {
			return err
		}
		meta = &oproxy.OAuthClientMetadata{}
		err = json.Unmarshal(bs, meta)
		if err != nil {
			return err
		}
	} else {
		// JSON
		err = json.Unmarshal([]byte(*clientMetadata), meta)
		if err != nil {
			return err
		}
	}

	_ = oproxy.New(&oproxy.Config{
		Host:               *host,
		CreateOAuthSession: store.CreateOAuthSession,
		UpdateOAuthSession: store.UpdateOAuthSession,
		GetOAuthSession:    store.GetOAuthSession,
		Scope:              *scope,
		ClientMetadata:     meta,
		// UpstreamJWK:        cli.JWK,
		// DownstreamJWK:      cli.AccessJWK,
	})

	return nil
}
