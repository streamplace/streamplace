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

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/carstore"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	urfavecli "github.com/urfave/cli/v3"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/director"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/replication"
	"stream.place/streamplace/pkg/replication/iroh_replicator"
	"stream.place/streamplace/pkg/replication/websocketrep"
	"stream.place/streamplace/pkg/rtmps"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/storage"
	"stream.place/streamplace/pkg/upload"
	"stream.place/streamplace/pkg/viewlog"
	"stream.place/streamplace/pkg/vod"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

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
		makeVODTestCommand(build),
		makeStreamCommand(build),
		makeLiveCommand(build),
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

	muxl.Configure(
		uint64(cli.MuxlInitialMemoryMB)*1024*1024,
		uint64(cli.MuxlMaxMemoryMB)*1024*1024,
	)

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

	serverHandle, err := atproto.MakeServerRepo(ctx, cli, state)
	if err != nil {
		return err
	}
	defer serverHandle.Close()

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

	serviceAuthKey, err := state.EnsureServiceAuthKey(ctx)
	if err != nil {
		return err
	}
	cli.ServiceAuthKey = serviceAuthKey

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
	})
	state.OATProxy = op

	err = atsync.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
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

	d := director.NewDirector(mm, mod, cli, b, op, state, replicator, ldb, atsync)
	um, err := upload.New(ctx, cli, state)
	if err != nil {
		return err
	}
	vodStore, err := makeVODStore(ctx, cli)
	if err != nil {
		return fmt.Errorf("make vod store: %w", err)
	}
	viewLog, err := makeViewLog(ctx, cli, vodStore, ldb)
	if err != nil {
		return fmt.Errorf("make view log: %w", err)
	}
	if viewLog != nil {
		group.Go(func() error {
			viewLog.Run(ctx)
			return nil
		})
		defer func() {
			if err := viewLog.Close(); err != nil {
				log.Error(ctx, "view log close", "error", err)
			}
		}()
	}
	state.SetVODProcessor(func(ctx context.Context, t statedb.VODProcessTask) (string, error) {
		// Labeler enforcement: an account banned after starting an
		// upload (but before processing) doesn't get a video published.
		// The playback gates would hide it regardless, but skipping here
		// avoids the wasted transcode and a dead record.
		labels, err := mod.GetActiveLabels(t.RepoDID)
		if err != nil {
			return "", fmt.Errorf("vod-process: check account labels: %w", err)
		}
		if atproto.IsBanned(labels...) {
			return "", fmt.Errorf("vod-process: account %s is banned; skipping upload %s", t.RepoDID, t.UploadID)
		}
		return vod.ProcessVOD(ctx, cli, state, vodStore, vod.Input{
			UploadID: t.UploadID,
			RepoDID:  t.RepoDID,
			MimeType: t.MimeType,
			Filename: t.Filename,
			Size:     t.Size,
			Backend:  t.Backend,
			Location: t.Location,
		})
	})
	// View-count aggregator runs the log → record pipeline for one
	// window. Same function-pointer pattern as the VOD processor so
	// statedb stays free of viewlog's transitive deps. The scheduler
	// goroutine fires per --view-count-aggregate-interval; statedb's
	// unique TaskKey makes the cross-node race a no-op for losers.
	if vodStore != nil && cli.ViewCountAggregateInterval > 0 {
		// Resolver: for a blob CID, return strongRefs of every
		// place.stream.media.track record whose muxlTrack lives in
		// that blob, keyed by in-container trackId. Used by the
		// aggregator to attribute bytes/duration to the right track
		// record (the streamer's original track records or, later,
		// user-contributed transcript/transcode tracks).
		fetchTrackRefs := func(ctx context.Context, cid string) (map[string]*comatproto.RepoStrongRef, error) {
			rows, err := mod.GetMediaTracksByBlob(ctx, cid)
			if err != nil {
				return nil, err
			}
			out := make(map[string]*comatproto.RepoStrongRef, len(rows))
			for _, row := range rows {
				rec, err := row.ToRecord()
				if err != nil {
					log.Warn(ctx, "viewlog refs: decode track record",
						"uri", row.URI, "error", err)
					continue
				}
				if rec.Track == nil || rec.Track.MediaDefs_MuxlTrack == nil {
					continue
				}
				tid := rec.Track.MediaDefs_MuxlTrack.TrackId
				if tid == "" {
					continue
				}
				out[tid] = &comatproto.RepoStrongRef{
					LexiconTypeID: "com.atproto.repo.strongRef",
					Uri:           row.URI,
					Cid:           row.CID,
				}
			}
			return out, nil
		}
		state.SetViewCountAggregator(func(ctx context.Context, t statedb.ViewCountAggregateTask) error {
			return viewlog.RunAggregation(ctx, viewlog.RunAggregationInput{
				Store:          vodStore,
				CLI:            cli,
				WindowStart:    t.WindowStart,
				WindowEnd:      t.WindowEnd,
				ReadMargin:     2 * cli.ViewLogFlushInterval,
				FetchTrackRefs: fetchTrackRefs,
			})
		})
		group.Go(func() error {
			return viewlog.ScheduleAggregations(ctx, state, viewlog.ScheduleConfig{
				Interval: cli.ViewCountAggregateInterval,
				Lag:      cli.ViewCountAggregateLag,
			})
		})
	}
	a, err := api.MakeStreamplaceAPI(cli, mod, state, noter, mm, ms, b, atsync, d, op, ldb, um, vodStore, viewLog)
	if err != nil {
		return err
	}

	ctx = log.WithLogValues(ctx, "version", build.Version)

	group.Go(func() error {
		return handleSignals(ctx)
	})

	group.Go(func() error {
		return state.ProcessQueue(ctx, cli.VODConcurrency)
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

	// Make sure the beta-invite issuer's repo is registered so the
	// firehose path indexes its place.stream.beta.invite records. The
	// first call also backfills any invites that were published before
	// we came online; subsequent runs are cached and no-op.
	if cli.BetaInviteDID != "" {
		go func() {
			if _, err := atsync.SyncBlueskyRepoCached(ctx, cli.BetaInviteDID); err != nil {
				log.Error(ctx, "failed to sync beta-invite issuer repo; gating will rely on the firehose alone",
					"did", cli.BetaInviteDID, "err", err)
			}
		}()
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

	if cli.LegacySegmentCleaner {
		group.Go(func() error {
			return ldb.StartSegmentCleaner(ctx)
		})
	}

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

// makeVODStore picks the blob.Store backing VOD output for this
// process. S3 if it's configured (production / multi-node); otherwise
// the local DataDir. Either way the same Store is used to read the
// user upload AND write the content-addressed VOD output — uploads
// land under "uploads/" and content blobs land under "blobs/<cid>.mp4"
// / "blobs/<cid>.json" (content-agnostic, since the blob doesn't know
// what kind of video it's for).
//
// FileStore is rooted at DataDir so it can see the upload manager's
// "uploads/<id>" tree alongside the content-addressed "blobs/" tree;
// S3Store is rooted at the configured bucket for the same reason.
//
// Mirrors upload.New's multi-node-requires-S3 invariant: in single-node
// file mode the upload and the produced VOD share local disk; in
// multi-node S3 mode any station can pick up a queued VOD task and
// land output in shared storage.
func makeVODStore(ctx context.Context, cli *config.CLI) (blob.Store, error) {
	if cli.S3Configured() {
		s3client := awss3.New(awss3.Options{
			Region: cli.S3Region,
			Credentials: credentials.NewStaticCredentialsProvider(
				cli.S3AccessKeyID,
				cli.S3SecretAccessKey,
				"",
			),
			BaseEndpoint: aws.String(cli.S3Endpoint),
			UsePathStyle: true,
		})
		log.Log(ctx, "VOD store: S3", "bucket", cli.S3Bucket)
		return blob.NewS3Store(s3client, cli.S3Bucket), nil
	}
	root := cli.DataFilePath(nil)
	log.Log(ctx, "VOD store: file", "root", root)
	return blob.NewFileStore(root)
}

// makeViewLog returns the configured view-event log writer, or nil if
// the operator has disabled it (--view-log-flush-interval=0) or there's
// no place to write logs to (no VOD store). The writer reuses the VOD
// blob.Store under a `view-logs/<server-did>/` prefix; if the operator
// runs S3-backed VOD, view logs land in the same bucket alongside the
// content blobs and pick up the bucket's lifecycle policy for free.
func makeViewLog(ctx context.Context, cli *config.CLI, vodStore blob.Store, ldb localdb.LocalDB) (*viewlog.Writer, error) {
	if cli.ViewLogFlushInterval <= 0 {
		log.Log(ctx, "view log: disabled (view-log-flush-interval=0)")
		return nil, nil
	}
	if vodStore == nil {
		log.Log(ctx, "view log: no VOD store wired; skipping")
		return nil, nil
	}
	w, err := viewlog.NewWriter(viewlog.Config{
		Store:      vodStore,
		NodeDID:    cli.ServerDID(),
		FlushAfter: cli.ViewLogFlushInterval,
		Salts:      viewlog.NewSaltManager(ldb),
	})
	if err != nil {
		return nil, err
	}
	log.Log(ctx, "view log: enabled", "flush_interval", cli.ViewLogFlushInterval, "node_did", cli.ServerDID())
	return w, nil
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

// makeVODTestCommand runs the VOD gstreamer pipeline on a local file
// and prints probe results. Useful for reproducing gstreamer-side
// crashes against the static binary without needing the full server
// scaffolding (DB, blob store, signer). The pipeline output is
// discarded — this is a "did it crash or not" smoke test.
func makeVODTestCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "vod-test",
		Usage:     "run the VOD gstreamer pipeline on a local file",
		ArgsUsage: "[file]",
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			args := cmd.Args()
			if args.Len() != 1 {
				return fmt.Errorf("usage: streamplace vod-test [file]")
			}
			return runVODTest(ctx, args.First())
		},
	}
}

func runVODTest(ctx context.Context, path string) error {
	gstinit.InitGST()

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	fmt.Printf("vod-test: processing %s (%d bytes)\n", path, info.Size())

	start := time.Now()
	result, outBytes, err := vod.ProcessToDiscard(ctx, f, info.Size())
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("vod-test: pipeline FAILED after %s: %v\n", elapsed, err)
		return err
	}
	fmt.Printf("vod-test: pipeline OK in %s, duration=%dms, output=%d bytes\n", elapsed, result.DurationMS, outBytes)
	if result.Video != nil {
		fmt.Printf("  video: codec=%s %dx%d fps=%d/%d\n",
			result.Video.Codec, result.Video.Width, result.Video.Height,
			result.Video.FPSNum, result.Video.FPSDen)
	}
	if result.Audio != nil {
		fmt.Printf("  audio: codec=%s rate=%d channels=%d\n",
			result.Audio.Codec, result.Audio.Rate, result.Audio.Channels)
	}
	return nil
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
	cli := config.CLI{Build: build}
	combineCmd := cli.NewCommand("combine")
	combineCmd.Usage = "combine segments"
	combineCmd.ArgsUsage = "[output] [input1] [input2...]"
	combineCmd.Flags = []urfavecli.Flag{
		&urfavecli.StringFlag{
			Name:  "debug-dir",
			Usage: "directory to write debug output",
		},
	}
	combineCmd.Action = func(ctx context.Context, cmd *urfavecli.Command) error {
		args := cmd.Args()
		if args.Len() < 2 {
			return fmt.Errorf("usage: streamplace combine [--debug-dir dir] [output] [input1] [input2...]")
		}
		ctx = log.WithDebugValue(ctx, cli.Debug)
		return Combine(
			ctx,
			&cli,
			cmd.String("debug-dir"),
			args.Get(0),
			args.Slice()[1:],
		)
	}
	return combineCmd
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
