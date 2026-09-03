package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"slices"
	"strconv"
	"stream.place/streamplace/pkg/accessctl"
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
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/director"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/ingestframe"
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
		makeIngestWorkerCommand(build),
		makeRTMPPushWorkerCommand(build),
		makeLiveCommand(build),
		makeWhepCommand(build),
		makeWhipCommand(build),
		makeCombineCommand(build),
		makeSplitCommand(build),
		makeLivepeerCommand(build),
		makeMigrateCommand(build),
		makeSyncCommand(build),
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

	mod, err := model.MakeDBConns(cli.DataFilePath([]string{"index"}), cli.IndexDBConnections)
	if err != nil {
		return err
	}
	var fbNotifier notifications.FirebaseNotifier
	if cli.FirebaseServiceAccount != "" {
		fbNotifier, err = notifications.MakeFirebaseNotifier(ctx, cli.FirebaseServiceAccount)
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
	// The notifier is assembled after the DB exists, because the Web Push
	// notifier needs VAPID keys that are persisted in the Config table. The
	// queue processor nil-checks the notifier, so the brief window is safe.
	state, err := statedb.MakeDB(ctx, cli, nil, mod)
	if err != nil {
		return err
	}

	// Build the Web Push notifier from VAPID keys generated/stored in the DB.
	vapidKeys, err := state.EnsureVAPIDKeys(ctx)
	if err != nil {
		return err
	}
	webNotifier := notifications.NewWebPushNotifier(vapidKeys, "")
	noter := notifications.NewMultiNotifier(fbNotifier, webNotifier)
	state.SetNotifier(noter)
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

	accessCtl, err := accessctl.New(ctx, cli, state, mod)
	if err != nil {
		return fmt.Errorf("access control: %w", err)
	}
	cli.Access = accessCtl

	b := bus.NewBus()
	atsync := &atproto.ATProtoSynchronizer{
		CLI:        cli,
		Model:      mod,
		StatefulDB: state,
		Noter:      noter,
		Bus:        b,
	}
	mm, err := media.MakeMediaManager(ctx, cli, signer, mod, b, atsync, ldb)
	if err != nil {
		return err
	}
	if cli.IsolatedIngest && !media.IngestIsolationSupported() {
		// The worker transport needs Unix fd-passing + Setsid (Linux today); fall
		// back to in-process ingest elsewhere rather than break.
		log.Log(ctx, "isolated ingest not supported on this platform; using in-process ingest", "goos", runtime.GOOS)
		cli.IsolatedIngest = false
	}
	if cli.IsolatedIngest {
		// Reconnect to any ingest workers still running from before this restart
		// and drain whatever they buffered while we were down (zero-downtime).
		mm.ResumeDetachedWorkers(ctx)
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
	// The synchronizer uses stored sessions to act as a streamer (e.g. ending
	// the source livestream when a teleport fires); without this it would
	// nil-deref the first time a teleport arrives for a logged-in streamer.
	atsync.OATProxy = op

	// Sync every repo we know about, at boot and every --sweep-interval after:
	// a repair pass for repos left half-indexed by a previous run, and a head
	// check that finds the ones that drifted while we were not listening.
	// Nothing below depends on it, so it runs in the background off the serve
	// context -- shutdown cancels it -- and the node is up and serving in the
	// meantime. Started only now that atsync is fully wired (OATProxy above):
	// the sweep can index records and schedule callbacks that use every field.
	go atsync.SweepForever(ctx)
	// History is fetched by its own loop rather than by those passes: it takes
	// days where reconciliation takes minutes, nothing on the node waits for
	// it, and at full speed it is what buries a boot. It trickles in at
	// --deepen-rate windows a minute for as long as this node runs.
	go atsync.DeepenForever(ctx)

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
	// Live-to-VOD finalize. Resolves the streamer's live signing key here
	// (where the model is in scope) so pkg/vod stays free of pkg/model, then
	// concatenates the recorded MUXL objects into a VOD.
	state.SetLivestreamVODFinalizer(func(ctx context.Context, t statedb.FinalizeLivestreamVODTask) (string, error) {
		signingKey, err := resolveLiveSigningKey(mod, t.RepoDID)
		if err != nil {
			return "", fmt.Errorf("finalize-livestream-vod: resolve signing key: %w", err)
		}
		return vod.FinalizeLivestreamVOD(ctx, cli, state, vodStore, vod.FinalizeInput{
			UploadID:      t.UploadID,
			RepoDID:       t.RepoDID,
			LivestreamURI: t.LivestreamURI,
			SigningKey:    signingKey,
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
		fetchTrackRefs := func(ctx context.Context, cid string) (map[string]comatproto.RepoStrongRef, error) {
			rows, err := mod.GetMediaTracksByBlob(ctx, cid)
			if err != nil {
				return nil, err
			}
			out := make(map[string]comatproto.RepoStrongRef, len(rows))
			for _, row := range rows {
				rec, err := row.ToRecord()
				if err != nil {
					log.Warn(ctx, "viewlog refs: decode track record",
						"uri", row.URI, "error", err)
					continue
				}
				if rec.Track.MediaDefs_MuxlTrack == nil {
					continue
				}
				tid := rec.Track.MediaDefs_MuxlTrack.TrackId
				if tid == "" {
					continue
				}
				out[tid] = comatproto.RepoStrongRef{
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

// makeIngestWorkerCommand is the per-stream isolated ingest worker (Stage 1:
// fMP4 / Mist pull). The node spawns it; it is not meant for direct use. It reads
// the config handshake from fd 3, the fragmented-MP4 media from stdin, runs the mux + sign
// pipeline, and writes signed canonical .m4s frames to fd 4 — dedicated fds so
// stray stdout/stderr can't corrupt the frame stream. A clean run ends with an
// End frame; a fatal error emits an Error frame before exiting non-zero.
func makeIngestWorkerCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "ingest-worker",
		Usage:     "internal: per-stream isolated ingest worker (spawned by the node)",
		ArgsUsage: "[streamer-did]",
		Hidden:    true,
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			// The streamer DID is passed on argv purely so the worker is
			// identifiable in a process listing (ps); the authoritative copy
			// still arrives in the fd-3 config. Thread it into the logger for
			// log correlation.
			if did := cmd.Args().First(); did != "" {
				ctx = log.WithLogValues(ctx, "streamer", did)
				log.Log(ctx, "ingest-worker starting")
			}
			cfgFile := os.NewFile(3, "ingest-config")
			if cfgFile == nil {
				return fmt.Errorf("ingest-worker: missing config fd 3")
			}
			cfgBytes, err := io.ReadAll(cfgFile)
			cfgFile.Close()
			if err != nil {
				return fmt.Errorf("ingest-worker: read config: %w", err)
			}
			var cfg media.IngestWorkerConfig
			if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
				return fmt.Errorf("ingest-worker: parse config: %w", err)
			}

			// WHIP transport: the worker owns the PeerConnection (built from the
			// offer in the config) and serves frames over the socket — no media fd.
			if cfg.Transport == media.IngestTransportWHIP {
				return media.ServeWHIPIngestWorkerSocket(ctx, cfg)
			}

			// Detach/reattach transport: serve frames over a unix socket with
			// buffered reconnect (survives a main restart) instead of the fd-4 pipe.
			// Media comes from the fd-passed ingest connection (InputFD) when main
			// handed one off, else stdin.
			if cfg.SocketPath != "" {
				raw := io.Reader(os.Stdin)
				if cfg.InputFD > 0 {
					f := os.NewFile(uintptr(cfg.InputFD), "ingest-input")
					if f == nil {
						return fmt.Errorf("ingest-worker: bad input fd %d", cfg.InputFD)
					}
					defer f.Close()
					raw = f
				}
				return media.ServeMP4IngestWorkerSocket(ctx, cfg, media.WorkerInput(cfg, raw))
			}

			framesFile := os.NewFile(4, "ingest-frames")
			if framesFile == nil {
				return fmt.Errorf("ingest-worker: missing frames fd 4")
			}
			defer framesFile.Close()
			frames := ingestframe.NewWriter(framesFile)

			// This fd-4 path has no back-channel for manifest updates, so the
			// manifest stays whatever main built at spawn. It's not used in prod
			// (api requires a hijackable connection); kept for the worker self-test.
			if err := media.RunMP4IngestWorker(ctx, cfg, os.Stdin, frames, func() []byte { return cfg.Manifest }); err != nil {
				_ = frames.Error(err.Error())
				return err
			}
			return frames.End()
		},
	}
}

// makeRTMPPushWorkerCommand is the per-target isolated multistream egress
// worker. The node spawns it; it is not meant for direct use. It reads the
// config handshake (incl. the target URL + stream key) from fd 3, the assembled
// fMP4 source stream from stdin, runs the native RTMP push pipeline, and writes
// status Event frames to fd 4 — dedicated fds so stray stdout/stderr can't
// corrupt the frame stream. A clean run ends with an End frame; a fatal error
// emits an Error frame before exiting non-zero.
func makeRTMPPushWorkerCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "rtmp-push-worker",
		Usage:     "internal: per-target isolated RTMP push worker (spawned by the node)",
		ArgsUsage: "[streamer-did]",
		Hidden:    true,
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			// The streamer DID is passed on argv purely so the worker is
			// identifiable in a process listing (ps); the target URL stays on fd 3.
			if did := cmd.Args().First(); did != "" {
				ctx = log.WithLogValues(ctx, "streamer", did)
				log.Log(ctx, "rtmp-push-worker starting")
			}
			cfgFile := os.NewFile(3, "push-config")
			if cfgFile == nil {
				return fmt.Errorf("rtmp-push-worker: missing config fd 3")
			}
			cfgBytes, err := io.ReadAll(cfgFile)
			cfgFile.Close()
			if err != nil {
				return fmt.Errorf("rtmp-push-worker: read config: %w", err)
			}
			var cfg media.RTMPPushWorkerConfig
			if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
				return fmt.Errorf("rtmp-push-worker: parse config: %w", err)
			}

			eventsFile := os.NewFile(4, "push-events")
			if eventsFile == nil {
				return fmt.Errorf("rtmp-push-worker: missing events fd 4")
			}
			defer eventsFile.Close()
			events := ingestframe.NewWriter(eventsFile)

			if err := media.RunRTMPPushWorker(ctx, cfg, os.Stdin, events); err != nil {
				_ = events.Error(err.Error())
				return err
			}
			return events.End()
		},
	}
}

func makeLiveCommand(build *config.BuildFlags) *urfavecli.Command {
	cli := config.CLI{Build: build}
	liveCmd := cli.NewCommand("live")
	liveCmd.Usage = "start live stream (pipe fragmented MP4 to stdin)"
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

// makeSyncCommand runs the backfill sweep to completion and exits, without
// starting a node. It is for the case where a new index revision has to be warm
// before traffic reaches it: run this, wait for it to finish, then start the
// server -- rather than starting the server and serving from an index that is
// still filling in behind it.
func makeSyncCommand(build *config.BuildFlags) *urfavecli.Command {
	cli := config.CLI{Build: build}
	syncCmd := cli.NewCommand("sync")
	syncCmd.Usage = "index every repo this node knows about, then exit"
	syncCmd.Action = func(ctx context.Context, cmd *urfavecli.Command) error {
		return runSync(ctx, build, cmd, &cli)
	}
	return syncCmd
}

// runSync builds the smallest stack a sweep needs -- the index, the state
// database, an identity resolver -- and nothing else. No HTTP servers, no media
// manager, no firehose: this process talks to other people's PDSes and to the
// two databases, and then it is done.
func runSync(ctx context.Context, build *config.BuildFlags, cmd *urfavecli.Command, cli *config.CLI) error {
	if err := cli.Validate(cmd); err != nil {
		return err
	}
	log.SetColorLogger(cli.Color)
	ctx = log.WithDebugValue(ctx, cli.Debug)
	log.Log(ctx, "streamplace sync", "version", build.Version, "dataDir", cli.DataDir)

	if err := os.MkdirAll(cli.DataDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating streamplace dir at %s: %w", cli.DataDir, err)
	}
	mod, err := model.MakeDBConns(cli.DataFilePath([]string{"index"}), cli.IndexDBConnections)
	if err != nil {
		return err
	}
	state, err := statedb.MakeDB(ctx, cli, nil, mod)
	if err != nil {
		return err
	}
	atsync := &atproto.ATProtoSynchronizer{
		CLI:        cli,
		Model:      mod,
		StatefulDB: state,
		Bus:        bus.NewBus(),
	}
	// A sweep that could not sync a single repo is a broken node and exits
	// nonzero; anything less than that heals on the next run, so it is logged
	// and forgiven.
	return atsync.Sweep(ctx)
}

// resolveLiveSigningKey returns the did:key whose private half signed a
// streamer's live segments, for stamping on live-to-VOD place.stream.media.track
// records so playback can verify them. It picks the most recently created
// non-revoked place.stream.key for the repo; streamers normally have exactly
// one. Errors if the repo has no active signing key.
func resolveLiveSigningKey(mod model.Model, repoDID string) (string, error) {
	keys, err := mod.GetSigningKeysForRepo(repoDID)
	if err != nil {
		return "", err
	}
	var best *model.SigningKey
	for i := range keys {
		k := &keys[i]
		if k.RevokedAt != nil {
			continue
		}
		if best == nil || k.CreatedAt.After(best.CreatedAt) {
			best = k
		}
	}
	if best == nil {
		return "", fmt.Errorf("no active signing key for repo %s", repoDID)
	}
	return best.DID, nil
}
