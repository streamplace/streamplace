package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/labstack/echo/v4"
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

const UPSTREAM_KEY = "upstream"
const DOWNSTREAM_KEY = "downstream"

func Run() error {
	flag.Set("logtostderr", "true")
	fs := flag.NewFlagSet("oproxy", flag.ExitOnError)
	noColor := fs.Bool("no-color", false, "disable colorized logging")
	host := fs.String("host", "", "public HTTPS address where this OAuth provider is hosted (ex example.com, no https:// prefix)")
	dbPath := fs.String("db", "oproxy.sqlite3", "path to the database file or postgres connection string")
	verbose := fs.Bool("v", false, "enable verbose logging")
	scope := fs.String("scope", "atproto transition:generic", "scope to use for the OAuth provider")
	clientMetadata := fs.String("client-metadata", "", "JSON client metadata or path to JSON file containing client metadata")
	httpAddr := fs.String("http-addr", ":8080", "HTTP address to listen on")
	proxyHost := fs.String("proxy-host", "", "location of backend reverse proxy for all non-oauth requests (ex http://localhost:8081)")
	// version := fs.Bool("version", false, "print version and exit")

	err := ff.Parse(
		fs, os.Args[1:],
		ff.WithEnvVarPrefix("OPROXY"),
	)
	if err != nil {
		return err
	}
	err = flag.CommandLine.Parse(nil)
	if err != nil {
		return err
	}

	if *proxyHost == "" {
		return fmt.Errorf("proxy-host is required")
	}
	if *host == "" {
		return fmt.Errorf("host is required")
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

	store, err := NewStore(*dbPath, logger, *verbose)
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

	upstreamKey, err := store.GetKey(UPSTREAM_KEY)
	if err != nil {
		return err
	}
	downstreamKey, err := store.GetKey(DOWNSTREAM_KEY)
	if err != nil {
		return err
	}
	o := oproxy.New(&oproxy.Config{
		Host:               *host,
		CreateOAuthSession: store.CreateOAuthSession,
		UpdateOAuthSession: store.UpdateOAuthSession,
		GetOAuthSession:    store.GetOAuthSession,
		Scope:              *scope,
		ClientMetadata:     meta,
		UpstreamJWK:        upstreamKey,
		DownstreamJWK:      downstreamKey,
	})

	reverse := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			u, err := url.Parse(*proxyHost)
			if err != nil {
				logger.Error("failed to parse proxy host", "error", err)
				return
			}
			u.RawPath = r.In.URL.RawPath
			u.RawQuery = r.In.URL.RawQuery
			logger.Info("proxying request", "url", u)
			r.SetURL(u)
		},
	}

	reverseEcho := func(c echo.Context) error {
		reverse.ServeHTTP(c.Response().Writer, c.Request())
		c.Response().Committed = true
		return nil
	}

	o.Echo.Any("/*", reverseEcho)

	server := &http.Server{
		Addr:    *httpAddr,
		Handler: o.Echo,
	}

	logger.Info("starting server", "addr", *httpAddr)
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
