package cmd

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/peterbourgon/ff/v3"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/term"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/crypto/signers/eip712"
	"stream.place/streamplace/pkg/director"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/replication"
	"stream.place/streamplace/pkg/replication/iroh_replicator"
	"stream.place/streamplace/pkg/replication/websocketrep"
	"stream.place/streamplace/pkg/rtmps"
	v0 "stream.place/streamplace/pkg/schema/v0"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/storage"

	"github.com/ThalesGroup/crypto11"
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
	selfTest := len(os.Args) > 1 && os.Args[1] == "self-test"
	err := media.RunSelfTest(context.Background())
	if err != nil {
		if selfTest {
			fmt.Println(err.Error())
			os.Exit(1)
		} else {
			retryCount, _ := strconv.Atoi(os.Getenv("STREAMPLACE_SELFTEST_RETRY"))
			if retryCount >= 3 {
				log.Error(context.Background(), "gstreamer self-test failed 3 times, giving up", "error", err)
				return err
			}
			log.Log(context.Background(), "error in gstreamer self-test, attempting recovery", "error", err, "retry", retryCount+1)
			os.Setenv("STREAMPLACE_SELFTEST_RETRY", strconv.Itoa(retryCount+1))
			err := syscall.Exec(os.Args[0], os.Args[1:], os.Environ())
			if err != nil {
				log.Error(context.Background(), "error in gstreamer self-test, could not restart", "error", err)
				return err
			}
			panic("invalid code path: exec succeeded but we're still here???")
		}
	}
	if selfTest {
		runtime.GC()
		if err := pprof.Lookup("goroutine").WriteTo(os.Stderr, 2); err != nil {
			log.Error(context.Background(), "error creating pprof", "error", err)
		}
		fmt.Println("self-test successful!")
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "stream" {
		if len(os.Args) != 3 {
			fmt.Println("usage: streamplace stream [user]")
			os.Exit(1)
		}
		return Stream(os.Args[2])
	}

	if len(os.Args) > 1 && os.Args[1] == "live" {
		cli := config.CLI{Build: build}
		fs := cli.NewFlagSet("streamplace live")

		err := cli.Parse(fs, os.Args[2:])
		if err != nil {
			return err
		}

		args := fs.Args()
		if len(args) != 1 {
			fmt.Println("usage: streamplace live [flags] [stream-key]")
			os.Exit(1)
		}

		return Live(args[0], cli.HTTPInternalAddr)
	}

	if len(os.Args) > 1 && os.Args[1] == "sign" {
		return Sign(context.Background())
	}

	if len(os.Args) > 1 && os.Args[1] == "whep" {
		return WHEP(os.Args[2:])
	}
	if len(os.Args) > 1 && os.Args[1] == "whip" {
		return WHIP(os.Args[2:])
	}

	if len(os.Args) > 1 && os.Args[1] == "clip" {
		cli := config.CLI{Build: build}
		fs := cli.NewFlagSet("streamplace clip")
		out := fs.String("out", "", "output file")

		err := cli.Parse(fs, os.Args[2:])
		if err != nil {
			return err
		}
		ctx := context.Background()
		ctx = log.WithDebugValue(ctx, cli.Debug)
		return Clip(ctx, fs.Args(), *out)
	}

	if len(os.Args) > 1 && os.Args[1] == "segment" {
		cli := config.CLI{Build: build}
		fs := cli.NewFlagSet("streamplace clip")
		out := fs.String("out-dir", "", "output directory")

		err := cli.Parse(fs, os.Args[2:])
		if err != nil {
			return err
		}
		ctx := context.Background()
		ctx = log.WithDebugValue(ctx, cli.Debug)
		return media.SegmentFile(ctx, fs.Args()[0], *out)
	}

	if len(os.Args) > 1 && os.Args[1] == "self-test" {
		err := media.RunSelfTest(context.Background())
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		fmt.Println("self-test successful!")
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "livepeer" {
		lpfs := flag.NewFlagSet("livepeer", flag.ExitOnError)
		_ = starter.NewLivepeerConfig(lpfs)
		err = ff.Parse(lpfs, os.Args[2:],
			ff.WithConfigFileFlag("config"),
			ff.WithEnvVarPrefix("LP"),
		)
		if err != nil {
			return err
		}
		err = GoLivepeer(context.Background(), lpfs)
		if err != nil {
			log.Error(context.Background(), "error in livepeer", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	_ = flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	cli := config.CLI{Build: build}
	fs := cli.NewFlagSet("streamplace")
	verbosity := fs.String("v", "3", "log verbosity level")
	version := fs.Bool("version", false, "print version and exit")

	err = cli.Parse(
		fs, os.Args[1:],
	)
	if err != nil {
		return err
	}

	err = flag.CommandLine.Parse(nil)
	if err != nil {
		return err
	}
	_ = vFlag.Value.Set(*verbosity)
	log.SetColorLogger(cli.Color)
	ctx := context.Background()
	ctx = log.WithDebugValue(ctx, cli.Debug)

	log.Log(ctx,
		"streamplace",
		"version", build.Version,
		"buildTime", build.BuildTimeStr(),
		"uuid", build.UUID,
		"runtime.GOOS", runtime.GOOS,
		"runtime.GOARCH", runtime.GOARCH,
		"runtime.Version", runtime.Version())
	if *version {
		return nil
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return statedb.Migrate(&cli)
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
	schema, err := v0.MakeV0Schema()
	if err != nil {
		return err
	}
	eip712signer, err := eip712.MakeEIP712Signer(ctx, &eip712.EIP712SignerOptions{
		Schema:              schema,
		EthKeystorePath:     cli.EthKeystorePath,
		EthAccountAddr:      cli.EthAccountAddr,
		EthKeystorePassword: cli.EthPassword,
	})
	if err != nil {
		return err
	}
	var signer crypto.Signer = eip712signer
	if cli.PKCS11ModulePath != "" {
		conf := &crypto11.Config{
			Path: cli.PKCS11ModulePath,
		}
		count := 0
		for _, val := range []string{cli.PKCS11TokenSlot, cli.PKCS11TokenLabel, cli.PKCS11TokenSerial} {
			if val != "" {
				count += 1
			}
		}
		if count != 1 {
			return fmt.Errorf("need exactly one of pkcs11-token-slot, pkcs11-token-label, or pkcs11-token-serial (got %d)", count)
		}
		if cli.PKCS11TokenSlot != "" {
			num, err := strconv.ParseInt(cli.PKCS11TokenSlot, 10, 16)
			if err != nil {
				return fmt.Errorf("error parsing pkcs11-slot: %w", err)
			}
			numint := int(num)
			// why does crypto11 want this as a reference? odd.
			conf.SlotNumber = &numint
		}
		if cli.PKCS11TokenLabel != "" {
			conf.TokenLabel = cli.PKCS11TokenLabel
		}
		if cli.PKCS11TokenSerial != "" {
			conf.TokenSerial = cli.PKCS11TokenSerial
		}
		pin := cli.PKCS11Pin
		if pin == "" {
			fmt.Printf("Please enter PKCS11 PIN: ")
			password, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println("")
			if err != nil {
				return fmt.Errorf("error reading PKCS11 password: %w", err)
			}
			pin = string(password)
		}
		conf.Pin = pin

		sc, err := crypto11.Configure(conf)
		if err != nil {
			return fmt.Errorf("error initalizing PKCS11 HSM: %w", err)
		}
		var id []byte = nil
		var label []byte = nil
		if cli.PKCS11KeypairID != "" {
			num, err := strconv.ParseInt(cli.PKCS11KeypairID, 10, 8)
			if err != nil {
				return fmt.Errorf("error parsing pkcs11-keypair-id: %w", err)
			}
			id = []byte{byte(num)}
		}
		if cli.PKCS11KeypairLabel != "" {
			label = []byte(cli.PKCS11KeypairLabel)
		}
		hwsigner, err := sc.FindKeyPair(id, label)
		if err != nil {
			return fmt.Errorf("error finding keypair on PKCS11 token: %w", err)
		}
		if hwsigner == nil {
			return fmt.Errorf("keypair on token not found (tried id='%s' label='%s')", cli.PKCS11KeypairID, cli.PKCS11KeypairLabel)
		}
		addr, err := signers.HexAddrFromSigner(hwsigner)
		if err != nil {
			return fmt.Errorf("error getting ethereum address for hardware keypair: %w", err)
		}
		log.Log(ctx, "successfully initialized hardware signer", "address", addr)
		signer = hwsigner
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
	state, err := statedb.MakeDB(ctx, &cli, noter, mod)
	if err != nil {
		return err
	}
	handle, err := atproto.MakeLexiconRepo(ctx, &cli, mod, state)
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
		CLI:        &cli,
		Model:      mod,
		StatefulDB: state,
		Noter:      noter,
		Bus:        b,
	}
	err = atsync.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	mm, err := media.MakeMediaManager(ctx, &cli, signer, mod, b, atsync)
	if err != nil {
		return err
	}

	ms, err := media.MakeMediaSigner(ctx, &cli, cli.StreamerName, signer, mod)
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
			Scope:      "atproto transition:generic",
			ClientName: "Streamplace",
			RedirectURIs: []string{
				fmt.Sprintf("%s/login", cli.OwnPublicURL()),
				fmt.Sprintf("%s/api/app-return", cli.OwnPublicURL()),
			},
		}
	} else {
		host = cli.BroadcasterHost
		clientMetadata = &oatproxy.OAuthClientMetadata{
			Scope:      "atproto transition:generic",
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
		replicator, err = iroh_replicator.NewSwarm(ctx, &cli, secret, topic, mm, b, mod)
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
		Scope:              "atproto transition:generic",
		UpstreamJWK:        cli.JWK,
		DownstreamJWK:      cli.AccessJWK,
		ClientMetadata:     clientMetadata,
		Public:             cli.PublicOAuth,
	})
	d := director.NewDirector(mm, mod, &cli, b, op, state, replicator)
	a, err := api.MakeStreamplaceAPI(&cli, mod, state, eip712signer, noter, mm, ms, b, atsync, d, op)
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
				return rtmps.ServeRTMPS(ctx, &cli)
			})
		}
	} else {
		group.Go(func() error {
			return a.ServeHTTP(ctx)
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
		return storage.StartSegmentCleaner(ctx, mod, &cli)
	})

	group.Go(func() error {
		return mod.StartSegmentCleaner(ctx)
	})

	group.Go(func() error {
		return replicator.Start(ctx, &cli)
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
			err := GoLivepeer(ctx, fs)
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
		// regular stream self-test
		testSigner, err := eip712.MakeEIP712Signer(ctx, &eip712.EIP712SignerOptions{
			Schema:          schema,
			EthKeystorePath: filepath.Join(cli.DataDir, "test-signer"),
		})
		if err != nil {
			return err
		}
		atkey, err := atproto.ParsePubKey(signer.Public())
		if err != nil {
			return err
		}
		did := atkey.DIDKey()
		testMediaSigner, err := media.MakeMediaSigner(ctx, &cli, did, testSigner, mod)
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
		intermittentSigner, err := eip712.MakeEIP712Signer(ctx, &eip712.EIP712SignerOptions{
			Schema:          schema,
			EthKeystorePath: filepath.Join(cli.DataDir, "intermittent-signer"),
		})
		if err != nil {
			return err
		}
		atkey2, err := atproto.ParsePubKey(intermittentSigner.Public())
		if err != nil {
			return err
		}
		did2 := atkey2.DIDKey()
		intermittentMediaSigner, err := media.MakeMediaSigner(ctx, &cli, did2, intermittentSigner, mod)
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
			return job(ctx, &cli)
		})
	}

	if cli.WHIPTest != "" {
		group.Go(func() error {
			err := WHIP(strings.Split(cli.WHIPTest, " "))
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
