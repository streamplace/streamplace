package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bluesky-social/indigo/carstore"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	urfavecli "github.com/urfave/cli/v3"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/director"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/replication"
	"stream.place/streamplace/pkg/replication/iroh_replicator"
	"stream.place/streamplace/pkg/replication/websocketrep"
	"stream.place/streamplace/pkg/rtmps"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/storage"

	_ "github.com/go-gst/go-glib/glib"
	_ "github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/api"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

// Additional jobs that can be injected by platforms
type jobFunc func(ctx context.Context, cli *config.CLI) error

// parse the CLI and fire up an streamplace node!
func start(build *config.BuildFlags, platformJobs []jobFunc) error {
	iroh_streamplace.InitLogging()

	cli := config.CLI{Build: build}
	app := cli.NewCommand("streamplace")
	app.Usage = "decentralized live streaming platform"
	app.Version = build.Version
	app.Commands = []*urfavecli.Command{
		makeSelfTestCommand(build),
		makeStreamCommand(build),
		makeLiveCommand(build),
		makeSignCommand(build),
		makeWhepCommand(build),
		makeWhipCommand(build),
		makeCombineCommand(build),
		makeSplitCommand(build),
		makeLivepeerCommand(build),
		makeMigrateCommand(build),
	}
	// Add the verbosity flag
	// app.Flags = append(app.Flags, &urfavecli.StringFlag{
	// 	Name:  "v",
	// 	Usage: "log verbosity level",
	// 	Value: "3",
	// })
	app.Before = func(ctx context.Context, cmd *urfavecli.Command) (context.Context, error) {
		// Run self-test before starting
		selfTest := cmd.Name == "self-test"
		err := media.RunSelfTest(ctx)
		if err != nil {
			if selfTest {
				fmt.Println(err.Error())
				os.Exit(1)
			} else {
				retryCount, _ := strconv.Atoi(os.Getenv("STREAMPLACE_SELFTEST_RETRY"))
				if retryCount >= 3 {
					log.Error(ctx, "gstreamer self-test failed 3 times, giving up", "error", err)
					return ctx, err
				}
				log.Log(ctx, "error in gstreamer self-test, attempting recovery", "error", err, "retry", retryCount+1)
				os.Setenv("STREAMPLACE_SELFTEST_RETRY", strconv.Itoa(retryCount+1))
				err := syscall.Exec(os.Args[0], os.Args[1:], os.Environ())
				if err != nil {
					log.Error(ctx, "error in gstreamer self-test, could not restart", "error", err)
					return ctx, err
				}
				panic("invalid code path: exec succeeded but we're still here???")
			}
		}
		return ctx, nil
	}
	app.Action = func(ctx context.Context, cmd *urfavecli.Command) error {
		return runMain(ctx, build, platformJobs, cmd, &cli)
	}

	return app.Run(context.Background(), os.Args)
}

func runMain(ctx context.Context, build *config.BuildFlags, platformJobs []jobFunc, cmd *urfavecli.Command, cli *config.CLI) error {
	_ = flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")

	err := cli.Validate(cmd)
	if err != nil {
		return err
	}

	err = flag.CommandLine.Parse(nil)
	if err != nil {
		return err
	}
	verbosity := cmd.String("v")
	_ = vFlag.Value.Set(verbosity)
	log.SetColorLogger(cli.Color)
	ctx = log.WithDebugValue(ctx, cli.Debug)

	log.Log(ctx,
		"streamplace",
		"version", build.Version,
		"buildTime", build.BuildTimeStr(),
		"uuid", build.UUID,
		"runtime.GOOS", runtime.GOOS,
		"runtime.GOARCH", runtime.GOARCH,
		"runtime.Version", runtime.Version())

	signer, err := createSigner(ctx, cli)
	if err != nil {
		return err
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return statedb.Migrate(cli)
	}

	spmetrics.Version.WithLabelValues(build.Version).Inc()
	if cli.LivepeerHelp {
		lpFlags := flag.NewFlagSet("livepeer", flag.ContinueOnError)
		_ = starter.NewLivepeerConfig(lpFlags)
		lpFlags.VisitAll(func(f *flag.Flag) {
			adapted := config.ToSnakeCase(f.Name)
			fmt.Printf("  -%s\n", fmt.Sprintf("livepeer.%s", adapted))
			usage := fmt.Sprintf("    	%s", f.Usage)
			if f.DefValue != "" {
				usage = fmt.Sprintf("%s (default %s)", usage, f.DefValue)
			}
			fmt.Printf("    	%s\n", usage)
		})
		return nil
	}

	aqhttp.UserAgent = fmt.Sprintf("streamplace/%s", build.Version)

	err = os.MkdirAll(cli.DataDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating streamplace dir at %s:%w", cli.DataDir, err)
	}

	ldb, err := localdb.MakeDB(cli.LocalDBURL)
	if err != nil {
		return err
	}

	mod, err := model.MakeDB(cli.DataFilePath([]string{"index"}))
	if err != nil {
		return err
	}
	var noter notifications.FirebaseNotifier
	if cli.FirebaseServiceAccount != "" {
		noter, err = notifications.MakeFirebaseNotifier(ctx, cli.FirebaseServiceAccount)
		if err != nil {
			return err
		}
	}

	group, ctx := TimeoutGroupWithContext(ctx)

	out := carstore.SQLiteStore{}
	err = out.Open(":memory:")
	if err != nil {
		return err
	}
	state, err := statedb.MakeDB(ctx, cli, noter, mod)
	if err != nil {
		return err
	}
	handle, err := atproto.MakeLexiconRepo(ctx, cli, mod, state)
	if err != nil {
		return err
	}
	defer handle.Close()

	jwk, err := state.EnsureJWK(ctx, "jwk")
	if err != nil {
		return err
	}
	cli.JWK = jwk

	accessJWK, err := state.EnsureJWK(ctx, "access-jwk")
	if err != nil {
		return err
	}
	cli.AccessJWK = accessJWK

	b := bus.NewBus()
	atsync := &atproto.ATProtoSynchronizer{
		CLI:        cli,
		Model:      mod,
		StatefulDB: state,
		Noter:      noter,
		Bus:        b,
	}
	err = atsync.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	mm, err := media.MakeMediaManager(ctx, cli, signer, mod, b, atsync, ldb)
	if err != nil {
		return err
	}

	ms, err := media.MakeMediaSigner(ctx, cli, cli.StreamerName, signer, mod)
	if err != nil {
		return err
	}

	var clientMetadata *oatproxy.OAuthClientMetadata
	var host string
	if cli.PublicOAuth {
		u, err := url.Parse(cli.OwnPublicURL())
		if err != nil {
			return err
		}
		host = u.Host
		clientMetadata = &oatproxy.OAuthClientMetadata{
			Scope:      atproto.OAuthString,
			ClientName: "Streamplace",
			RedirectURIs: []string{
				fmt.Sprintf("%s/login", cli.OwnPublicURL()),
				fmt.Sprintf("%s/api/app-return", cli.OwnPublicURL()),
			},
		}
	} else {
		host = cli.BroadcasterHost
		clientMetadata = &oatproxy.OAuthClientMetadata{
			Scope:      atproto.OAuthString,
			ClientName: "Streamplace",
			RedirectURIs: []string{
				fmt.Sprintf("https://%s/login", cli.BroadcasterHost),
				fmt.Sprintf("https://%s/api/app-return", cli.BroadcasterHost),
			},
		}
	}

	var replicator replication.Replicator = nil
	if slices.Contains(cli.Replicators, config.ReplicatorIroh) {
		exists, err := cli.DataFileExists([]string{"iroh-kv-secret"})
		if err != nil {
			return err
		}
		if !exists {
			secret := make([]byte, 32)
			_, err := rand.Read(secret)
			if err != nil {
				return fmt.Errorf("failed to generate random secret: %w", err)
			}
			err = cli.DataFileWrite([]string{"iroh-kv-secret"}, bytes.NewReader(secret), true)
			if err != nil {
				return err
			}
		}
		buf := bytes.Buffer{}
		err = cli.DataFileRead([]string{"iroh-kv-secret"}, &buf)
		if err != nil {
			return err
		}
		secret := buf.Bytes()
		var topic []byte
		if cli.IrohTopic != "" {
			topic, err = hexutil.Decode("0x" + cli.IrohTopic)
			if err != nil {
				return err
			}
		}
		replicator, err = iroh_replicator.NewSwarm(ctx, cli, secret, topic, mm, b, mod)
		if err != nil {
			return err
		}
	}
	if slices.Contains(cli.Replicators, config.ReplicatorWebsocket) {
		replicator = websocketrep.NewWebsocketReplicator(b, mod, mm)
	}

	op := oatproxy.New(&oatproxy.Config{
		Host:               host,
		CreateOAuthSession: state.CreateOAuthSession,
		UpdateOAuthSession: state.UpdateOAuthSession,
		GetOAuthSession:    state.LoadOAuthSession,
		Lock:               state.GetNamedLock,
		Scope:              atproto.OAuthString,
		UpstreamJWK:        cli.JWK,
		DownstreamJWK:      cli.AccessJWK,
		ClientMetadata:     clientMetadata,
		Public:             cli.PublicOAuth,
		HTTPClient:         &aqhttp.Client,
	})
	d := director.NewDirector(mm, mod, cli, b, op, state, replicator, ldb)
	a, err := api.MakeStreamplaceAPI(cli, mod, state, noter, mm, ms, b, atsync, d, op, ldb)
	if err != nil {
		return err
	}

	ctx = log.WithLogValues(ctx, "version", build.Version)

	group.Go(func() error {
		return handleSignals(ctx)
	})

	group.Go(func() error {
		return state.ProcessQueue(ctx)
	})

	if cli.TracingEndpoint != "" {
		group.Go(func() error {
			return startTelemetry(ctx, cli.TracingEndpoint)
		})
	}

	if cli.Secure {
		group.Go(func() error {
			return a.ServeHTTPS(ctx)
		})
		group.Go(func() error {
			return a.ServeHTTPRedirect(ctx)
		})
		if cli.RTMPServerAddon != "" {
			group.Go(func() error {
				return rtmps.ServeRTMPSAddon(ctx, cli)
			})
		}
		group.Go(func() error {
			return a.ServeRTMPS(ctx, cli)
		})
	} else {
		group.Go(func() error {
			return a.ServeHTTP(ctx)
		})
		group.Go(func() error {
			return a.ServeRTMP(ctx)
		})
	}

	group.Go(func() error {
		return a.ServeInternalHTTP(ctx)
	})

	if !cli.NoFirehose {
		group.Go(func() error {
			return atsync.StartFirehose(ctx)
		})
	}
	for _, labeler := range cli.Labelers {
		group.Go(func() error {
			return atsync.StartLabelerFirehose(ctx, labeler)
		})
	}

	group.Go(func() error {
		return a.ExpireSessions(ctx)
	})

	group.Go(func() error {
		return storage.StartSegmentCleaner(ctx, ldb, cli)
	})

	group.Go(func() error {
		return ldb.StartSegmentCleaner(ctx)
	})

	group.Go(func() error {
		return replicator.Start(ctx, cli)
	})

	if cli.LivepeerGateway {
		// make a file to make sure the directory exists
		fd, err := cli.DataFileCreate([]string{"livepeer", "gateway", "empty"}, true)
		if err != nil {
			return err
		}
		fd.Close()
		if err != nil {
			return err
		}
		group.Go(func() error {
			err = GoLivepeer(ctx, config.LivepeerFlagSet)
			if err != nil {
				return err
			}
			// livepeer returns nil on error, so we need to check if we're responsible
			if ctx.Err() == nil {
				return fmt.Errorf("livepeer exited")
			}
			return nil
		})
	}

	group.Go(func() error {
		return d.Start(ctx)
	})

	if cli.TestStream {
		atkey, err := atproto.ParsePubKey(signer.Public())
		if err != nil {
			return err
		}
		did := atkey.DIDKey()
		testMediaSigner, err := media.MakeMediaSigner(ctx, cli, did, signer, mod)
		if err != nil {
			return err
		}
		err = mod.UpdateIdentity(&model.Identity{
			ID:     testMediaSigner.Pub().String(),
			Handle: "stream-self-tester",
			DID:    "",
		})
		if err != nil {
			return err
		}
		cli.AllowedStreams = append(cli.AllowedStreams, did)
		a.Aliases["self-test"] = did
		group.Go(func() error {
			return mm.TestSource(ctx, testMediaSigner)
		})

		// Start a test stream that will run intermittently
		if err != nil {
			return err
		}
		atkey2, err := atproto.ParsePubKey(signer.Public())
		if err != nil {
			return err
		}
		did2 := atkey2.DIDKey()
		intermittentMediaSigner, err := media.MakeMediaSigner(ctx, cli, did2, signer, mod)
		if err != nil {
			return err
		}
		err = mod.UpdateIdentity(&model.Identity{
			ID:     intermittentMediaSigner.Pub().String(),
			Handle: "stream-intermittent-tester",
			DID:    "",
		})
		if err != nil {
			return err
		}
		cli.AllowedStreams = append(cli.AllowedStreams, did2)
		a.Aliases["intermittent-self-test"] = did2

		group.Go(func() error {
			for {
				// Start intermittent stream
				intermittentCtx, cancel := context.WithCancel(ctx)
				done := make(chan struct{})
				go func() {
					_ = mm.TestSource(intermittentCtx, intermittentMediaSigner)
					close(done)
				}()
				// Stream ON for 15 seconds
				time.Sleep(15 * time.Second)
				// Stop stream
				cancel()
				<-done // Wait for TestSource to exit
				// Stream OFF for 15 seconds
				time.Sleep(15 * time.Second)
			}
		})
	}

	for _, job := range platformJobs {
		group.Go(func() error {
			return job(ctx, cli)
		})
	}

	if cli.WHIPTest != "" {
		group.Go(func() error {
			// Parse WHIPTest string using the whip command's flag parser
			whipCmd := makeWhipCommand(build)
			args := strings.Split(cli.WHIPTest, " ")
			err := whipCmd.Run(ctx, append([]string{"streamplace", "whip"}, args...))
			log.Warn(ctx, "WHIP test complete, sleeping for 3 seconds and shutting down gstreamer")
			time.Sleep(time.Second * 3)
			// gst.Deinit()
			log.Warn(ctx, "gst deinit complete, exiting")
			return err
		})
	}

	return group.Wait()
}

var ErrCaughtSignal = errors.New("caught signal")

func handleSignals(ctx context.Context) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT, syscall.SIGABRT)
	for {
		select {
		case s := <-c:
			if s == syscall.SIGABRT {
				if err := pprof.Lookup("goroutine").WriteTo(os.Stderr, 2); err != nil {
					log.Error(ctx, "failed to create pprof", "error", err)
				}
			}
			log.Log(ctx, "caught signal, attempting clean shutdown", "signal", s)
			return fmt.Errorf("%w signal=%v", ErrCaughtSignal, s)
		case <-ctx.Done():
			return nil
		}
	}
}

func makeSelfTestCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "self-test",
		Usage: "run gstreamer self-test",
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			err := media.RunSelfTest(ctx)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			runtime.GC()
			if err := pprof.Lookup("goroutine").WriteTo(os.Stderr, 2); err != nil {
				log.Error(ctx, "error creating pprof", "error", err)
			}
			fmt.Println("self-test successful!")
			return nil
		},
	}
}

func makeStreamCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "stream",
		Usage:     "stream command",
		ArgsUsage: "[user]",
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			args := cmd.Args()
			if args.Len() != 1 {
				return fmt.Errorf("usage: streamplace stream [user]")
			}
			return Stream(args.First())
		},
	}
}

func makeLiveCommand(build *config.BuildFlags) *urfavecli.Command {
	cli := config.CLI{Build: build}
	liveCmd := cli.NewCommand("live")
	liveCmd.Usage = "start live stream"
	liveCmd.ArgsUsage = "[stream-key]"
	liveCmd.Action = func(ctx context.Context, cmd *urfavecli.Command) error {
		args := cmd.Args()
		if args.Len() != 1 {
			return fmt.Errorf("usage: streamplace live [flags] [stream-key]")
		}
		return Live(args.First(), cli.HTTPInternalAddr)
	}
	return liveCmd
}

func makeSignCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "sign",
		Usage: "sign command",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:  "cert",
				Usage: "path to the certificate file",
			},
			&urfavecli.StringFlag{
				Name:  "key",
				Usage: "base58-encoded secp256k1 private key",
			},
			&urfavecli.StringFlag{
				Name:  "streamer",
				Usage: "streamer name",
			},
			&urfavecli.StringFlag{
				Name:  "ta-url",
				Usage: "timestamp authority server for signing",
				Value: "http://timestamp.digicert.com",
			},
			&urfavecli.IntFlag{
				Name:  "start-time",
				Usage: "start time of the stream",
			},
			&urfavecli.StringFlag{
				Name:  "manifest",
				Usage: "JSON manifest to use for signing",
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return Sign(
				ctx,
				cmd.String("cert"),
				cmd.String("key"),
				cmd.String("streamer"),
				cmd.String("ta-url"),
				int64(cmd.Int("start-time")),
				cmd.String("manifest"),
			)
		},
	}
}

func makeWhepCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "whep",
		Usage: "WHEP client",
		Flags: []urfavecli.Flag{
			&urfavecli.IntFlag{
				Name:  "count",
				Usage: "number of concurrent streams (for load testing)",
				Value: 1,
			},
			&urfavecli.DurationFlag{
				Name:  "duration",
				Usage: "stop after this long",
			},
			&urfavecli.StringFlag{
				Name:  "endpoint",
				Usage: "endpoint to send the WHEP request to",
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return WHEP(
				ctx,
				cmd.Int("count"),
				cmd.Duration("duration"),
				cmd.String("endpoint"),
			)
		},
	}
}

func makeWhipCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "whip",
		Usage: "WHIP client",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:  "stream-key",
				Usage: "stream key",
			},
			&urfavecli.IntFlag{
				Name:  "count",
				Usage: "number of concurrent streams (for load testing)",
				Value: 1,
			},
			&urfavecli.IntFlag{
				Name:  "viewers",
				Usage: "number of viewers to simulate per stream",
			},
			&urfavecli.DurationFlag{
				Name:  "duration",
				Usage: "duration of the stream",
			},
			&urfavecli.StringFlag{
				Name:     "file",
				Usage:    "file to stream (needs to be an MP4 containing H264 video and Opus audio)",
				Required: true,
			},
			&urfavecli.StringFlag{
				Name:  "endpoint",
				Usage: "endpoint to send the WHIP request to",
				Value: "http://127.0.0.1:38080",
			},
			&urfavecli.DurationFlag{
				Name:  "freeze-after",
				Usage: "freeze the stream after the given duration",
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return WHIP(
				ctx,
				cmd.String("stream-key"),
				cmd.Int("count"),
				cmd.Int("viewers"),
				cmd.Duration("duration"),
				cmd.String("file"),
				cmd.String("endpoint"),
				cmd.Duration("freeze-after"),
			)
		},
	}
}

func makeCombineCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "combine",
		Usage: "combine segments",
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return Combine(ctx, build, cmd.Args().Slice())
		},
	}
}

func makeSplitCommand(build *config.BuildFlags) *urfavecli.Command {
	cli := config.CLI{Build: build}
	splitCmd := cli.NewCommand("split")
	splitCmd.Usage = "split video file"
	splitCmd.ArgsUsage = "[input file] [output directory]"
	splitCmd.Action = func(ctx context.Context, cmd *urfavecli.Command) error {
		args := cmd.Args()
		if args.Len() != 2 {
			return fmt.Errorf("usage: streamplace split [flags] [input file] [output directory]")
		}
		ctx = log.WithDebugValue(ctx, cli.Debug)
		gstinit.InitGST()
		return Split(ctx, args.Get(0), args.Get(1))
	}
	return splitCmd
}

func makeLivepeerCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "livepeer",
		Usage: "run livepeer gateway",
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return GoLivepeer(ctx, config.LivepeerFlagSet)
		},
	}
}

func makeMigrateCommand(build *config.BuildFlags) *urfavecli.Command {
	cli := config.CLI{Build: build}
	return &urfavecli.Command{
		Name:  "migrate",
		Usage: "run database migrations",
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return statedb.Migrate(&cli)
		},
	}
}
