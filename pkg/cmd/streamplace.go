package cmd

import (
	"context"
	"crypto"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/crypto/signers/eip712"
	"stream.place/streamplace/pkg/director"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/replication"
	"stream.place/streamplace/pkg/replication/boring"
	v0 "stream.place/streamplace/pkg/schema/v0"
	"stream.place/streamplace/pkg/spmetrics"

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
		pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
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

	if len(os.Args) > 1 && os.Args[1] == "sign" {
		return Sign(context.Background())
	}

	if len(os.Args) > 1 && os.Args[1] == "whep" {
		return WHEP(os.Args[2:])
	}
	if len(os.Args) > 1 && os.Args[1] == "whip" {
		return WHIP(os.Args[2:])
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
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet("streamplace", flag.ExitOnError)
	cli := config.CLI{Build: build}
	fs.StringVar(&cli.DataDir, "data-dir", config.DefaultDataDir(), "directory for keeping all streamplace data")
	fs.StringVar(&cli.HttpAddr, "http-addr", ":38080", "Public HTTP address")
	fs.StringVar(&cli.HttpInternalAddr, "http-internal-addr", "127.0.0.1:39090", "Private, admin-only HTTP address")
	fs.StringVar(&cli.HttpsAddr, "https-addr", ":38443", "Public HTTPS address")
	fs.BoolVar(&cli.Secure, "secure", false, "Run with HTTPS. Required for WebRTC output")
	cli.DataDirFlag(fs, &cli.TLSCertPath, "tls-cert", filepath.Join("tls", "tls.crt"), "Path to TLS certificate")
	cli.DataDirFlag(fs, &cli.TLSKeyPath, "tls-key", filepath.Join("tls", "tls.key"), "Path to TLS key")
	fs.StringVar(&cli.SigningKeyPath, "signing-key", "", "Path to signing key for pushing OTA updates to the app")
	cli.DataDirFlag(fs, &cli.DBPath, "db-path", "db.sqlite", "path to sqlite database file")
	fs.StringVar(&cli.AdminAccount, "admin-account", "", "ethereum account that administrates this streamplace node")
	fs.StringVar(&cli.FirebaseServiceAccount, "firebase-service-account", "", "JSON string of a firebase service account key")
	fs.StringVar(&cli.GitLabURL, "gitlab-url", "https://git.stream.place/api/v4/projects/1", "gitlab url for generating download links")
	cli.DataDirFlag(fs, &cli.EthKeystorePath, "eth-keystore-path", "keystore", "path to ethereum keystore")
	fs.StringVar(&cli.EthAccountAddr, "eth-account-addr", "", "ethereum account address to use (if keystore contains more than one)")
	fs.StringVar(&cli.EthPassword, "eth-password", "", "password for encrypting keystore")
	fs.StringVar(&cli.TAURL, "ta-url", "http://timestamp.digicert.com", "timestamp authority server for signing")
	fs.StringVar(&cli.PKCS11ModulePath, "pkcs11-module-path", "", "path to a PKCS11 module for HSM signing, for example /usr/lib/x86_64-linux-gnu/opensc-pkcs11.so")
	fs.StringVar(&cli.PKCS11Pin, "pkcs11-pin", "", "PIN for logging into PKCS11 token. if not provided, will be prompted interactively")
	fs.StringVar(&cli.PKCS11TokenSlot, "pkcs11-token-slot", "", "slot number of PKCS11 token (only use one of slot, label, or serial)")
	fs.StringVar(&cli.PKCS11TokenLabel, "pkcs11-token-label", "", "label of PKCS11 token (only use one of slot, label, or serial)")
	fs.StringVar(&cli.PKCS11TokenSerial, "pkcs11-token-serial", "", "serial number of PKCS11 token (only use one of slot, label, or serial)")
	fs.StringVar(&cli.PKCS11KeypairLabel, "pkcs11-keypair-label", "", "label of signing keypair on PKCS11 token")
	fs.StringVar(&cli.PKCS11KeypairID, "pkcs11-keypair-id", "", "id of signing keypair on PKCS11 token")
	fs.StringVar(&cli.AppBundleID, "app-bundle-id", "", "bundle id of an app that we facilitate oauth login for")
	fs.StringVar(&cli.StreamerName, "streamer-name", "", "name of the person streaming from this streamplace node")
	fs.StringVar(&cli.FrontendProxy, "dev-frontend-proxy", "", "(FOR DEVELOPMENT ONLY) proxy frontend requests to this address instead of using the bundled frontend")
	fs.StringVar(&cli.LivepeerGatewayURL, "livepeer-gateway-url", "", "URL of the Livepeer Gateway to use for transcoding")
	fs.BoolVar(&cli.WideOpen, "wide-open", false, "allow ALL streams to be uploaded to this node (not recommended for production)")
	cli.StringSliceFlag(fs, &cli.AllowedStreams, "allowed-streams", "", "if set, only allow these addresses or atproto DIDs to upload to this node")
	cli.StringSliceFlag(fs, &cli.Peers, "peers", "", "other streamplace nodes to replicate to")
	cli.StringSliceFlag(fs, &cli.Redirects, "redirects", "", "http 302s /path/one:/path/two,/path/three:/path/four")
	cli.DebugFlag(fs, &cli.Debug, "debug", "", "modified log verbosity for specific functions or files in form func=ToHLS:3,file=gstreamer.go:4")
	fs.BoolVar(&cli.TestStream, "test-stream", false, "run a built-in test stream on boot")
	fs.BoolVar(&cli.NoFirehose, "no-firehose", false, "disable the bluesky firehose")
	fs.BoolVar(&cli.PrintChat, "print-chat", false, "print chat messages to stdout")
	fs.StringVar(&cli.WHIPTest, "whip-test", "", "run a WHIP self-test with the given parameters")
	verbosity := fs.String("v", "3", "log verbosity level")
	fs.StringVar(&cli.RelayHost, "relay-host", "wss://bsky.network", "websocket url for relay firehose")
	fs.Bool("insecure", false, "DEPRECATED, does nothing.")
	fs.StringVar(&cli.Color, "color", "", "'true' to enable colorized logging, 'false' to disable")
	fs.BoolVar(&cli.Thumbnail, "thumbnail", true, "enable thumbnail generation")
	fs.BoolVar(&cli.SmearAudio, "smear-audio", false, "enable audio smearing to create 'perfect' segment timestamps")
	fs.BoolVar(&cli.ExternalSigning, "external-signing", false, "enable external signing via exec (prevents potential memory leak)")
	fs.StringVar(&cli.TracingEndpoint, "tracing-endpoint", "", "gRPC endpoint to send traces to")
	version := fs.Bool("version", false, "print version and exit")

	if runtime.GOOS == "linux" {
		fs.BoolVar(&cli.NoMist, "no-mist", true, "Disable MistServer")
		fs.IntVar(&cli.MistAdminPort, "mist-admin-port", 14242, "MistServer admin port (internal use only)")
		fs.IntVar(&cli.MistRTMPPort, "mist-rtmp-port", 11935, "MistServer RTMP port (internal use only)")
		fs.IntVar(&cli.MistHTTPPort, "mist-http-port", 18080, "MistServer HTTP port (internal use only)")
	}

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
	vFlag.Value.Set(*verbosity)
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
	spmetrics.Version.WithLabelValues(build.Version).Inc()

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
	var rep replication.Replicator = &boring.BoringReplicator{Peers: cli.Peers}
	mod, err := model.MakeDB(cli.DBPath)
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
	b := bus.NewBus()
	atsync := &atproto.ATProtoSynchronizer{
		CLI:   &cli,
		Model: mod,
		Noter: noter,
		Bus:   b,
	}
	mm, err := media.MakeMediaManager(ctx, &cli, signer, rep, mod, b, atsync)
	if err != nil {
		return err
	}

	ms, err := media.MakeMediaSigner(ctx, &cli, cli.StreamerName, signer)
	if err != nil {
		return err
	}

	d := director.NewDirector(mm, mod, &cli, b)

	a, err := api.MakeStreamplaceAPI(&cli, mod, eip712signer, noter, mm, ms, b, atsync, d)
	if err != nil {
		return err
	}

	group, ctx := TimeoutGroupWithContext(ctx)
	ctx = log.WithLogValues(ctx, "version", build.Version)

	group.Go(func() error {
		return handleSignals(ctx)
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

	group.Go(func() error {
		return spmetrics.ExpireSessions(ctx)
	})

	group.Go(func() error {
		return mod.StartSegmentCleaner(ctx)
	})

	group.Go(func() error {
		return d.Start(ctx)
	})

	if cli.TestStream {
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
		testMediaSigner, err := media.MakeMediaSigner(ctx, &cli, did, testSigner)
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
				pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
			}
			log.Log(ctx, "caught signal, attempting clean shutdown", "signal", s)
			return fmt.Errorf("%w signal=%v", ErrCaughtSignal, s)
		case <-ctx.Done():
			return nil
		}
	}
}
