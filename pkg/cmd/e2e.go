package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	glex "github.com/streamplace/glex/runtime"
	urfavecli "github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	spcomatproto "stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/test/remote"
)

// createRecord writes a glex record into the given repo via the public
// com.atproto.repo.createRecord XRPC. The repo's own generated comatproto
// types are used (rather than indigo's) so the record's $type survives
// marshalling: indigo's LexiconTypeDecoder only accepts structs with a const
// $type field, which glex-generated records do not have.
func createRecord(ctx context.Context, client *xrpc.Client, collection, repo string, record glex.Record) (string, error) {
	inp := spcomatproto.RepoCreateRecord_Input{
		Collection: collection,
		Repo:       repo,
		Record:     &glex.LexiconTypeDecoder{Val: record},
	}
	out := spcomatproto.RepoCreateRecord_Output{}
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, inp, &out); err != nil {
		return "", err
	}
	return out.Uri, nil
}

func makeE2eCommand(build *config.BuildFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "e2e",
		Usage: "start a self-contained e2e test environment with a test account and live stream",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:    "dev-env",
				Usage:   "path to js/dev-env/run.mjs",
				Value:   "js/dev-env/run.mjs",
				Sources: urfavecli.EnvVars("SP_DEV_ENV_MJS"),
			},
			// Together these turn on locally trusted HTTPS, so atproto OAuth
			// works end to end; see e2e_https.go.
			&urfavecli.StringFlag{
				Name:    "https-pds-hostname",
				Usage:   "serve the PDS at https://<this>, with account handles under it; it and *.<it> must resolve to 127.0.0.1 (binds 127.0.0.1:443)",
				Sources: urfavecli.EnvVars("SP_E2E_HTTPS_PDS_HOSTNAME"),
			},
			&urfavecli.StringFlag{
				Name:    "https-station-hostname",
				Usage:   "the node's broadcaster host, served at https://<this>, which must resolve to 127.0.0.1",
				Sources: urfavecli.EnvVars("SP_E2E_HTTPS_STATION_HOSTNAME"),
			},
			&urfavecli.IntFlag{
				Name:    "https-port",
				Usage:   "port the HTTPS front end listens on; the URLs stay portless, so anything but 443 only works where loopback 443 is redirected to it",
				Value:   443,
				Sources: urfavecli.EnvVars("SP_E2E_HTTPS_PORT"),
			},
			&urfavecli.StringFlag{
				Name:    "app-bundle-id",
				Usage:   "bundle id of the mobile app under test, which the node hands OAuth logins back to (its --app-bundle-id)",
				Sources: urfavecli.EnvVars("SP_E2E_APP_BUNDLE_ID"),
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			// Canonical form, so they compare equal to the SNI names the
			// TLS front end routes on.
			canon := func(h string) string { return strings.ToLower(strings.TrimSuffix(h, ".")) }
			pdsHost, stationHost := canon(cmd.String("https-pds-hostname")), canon(cmd.String("https-station-hostname"))
			if (pdsHost == "") != (stationHost == "") {
				return errors.New("--https-pds-hostname and --https-station-hostname go together")
			}
			return runE2E(ctx, cmd.String("dev-env"), pdsHost, stationHost, int(cmd.Int("https-port")), cmd.String("app-bundle-id"))
		},
	}
}

// e2eDeprecatedIngestHost is the hostname the harness node treats as a
// deprecated ingest host (--deprecated-ingest-hosts). It has to resolve to the
// node's loopback listener, since the RTMP client dials the host it names in
// its tcUrl.
const e2eDeprecatedIngestHost = "localhost"

// nodeReadyTimeout bounds how long runE2E waits for the forked node to answer
// /api/healthz. A cold dev node has to run a GStreamer self-test and open its
// databases first, so this is generous — but it is a bound, not forever.
const nodeReadyTimeout = 90 * time.Second

type e2eDevEnv struct {
	PDSURL string `json:"pds-url"`
	PLCURL string `json:"plc-url"`
	// Only in HTTPS mode: the account the PDS resolves lexicons from.
	LexiconDID      string `json:"lexicon-did"`
	LexiconPassword string `json:"lexicon-password"`
}

// e2eAccount is a test account on the dev-env PDS, and a client logged in as
// it.
type e2eAccount struct {
	DID      string
	Handle   string
	Password string
	client   *xrpc.Client
}

func createE2EAccount(ctx context.Context, pdsURL, handleDomain string) (*e2eAccount, error) {
	xrpcc := &xrpc.Client{Host: pdsURL, Client: &aqhttp.TrustedClient}
	uu, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	handle := fmt.Sprintf("sp-%s.%s", uu.String()[:8], handleDomain)
	email := fmt.Sprintf("%s@example.com", handle)
	password := "test"
	out, err := comatproto.ServerCreateAccount(ctx, xrpcc, &comatproto.ServerCreateAccount_Input{
		Handle:   handle,
		Email:    &email,
		Password: &password,
	})
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	session, err := comatproto.ServerCreateSession(ctx, xrpcc, &comatproto.ServerCreateSession_Input{
		Identifier: out.Handle,
		Password:   password,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &e2eAccount{
		DID:      out.Did,
		Handle:   out.Handle,
		Password: password,
		client: &xrpc.Client{
			Host:   pdsURL,
			Client: &aqhttp.TrustedClient,
			Auth: &xrpc.AuthInfo{
				Did:        out.Did,
				AccessJwt:  session.AccessJwt,
				RefreshJwt: session.RefreshJwt,
				Handle:     out.Handle,
			},
		},
	}, nil
}

// registerStreamKey makes a stream key for the account and publishes it, and
// returns the private key an encoder streams with.
func registerStreamKey(ctx context.Context, acct *e2eAccount) (string, error) {
	priv, pub, err := spkey.GenerateStreamKeyForDID(acct.DID)
	if err != nil {
		return "", fmt.Errorf("generate stream key: %w", err)
	}
	createdBy := "e2e"
	streamKey := placestream.Key{
		SigningKey: pub.DIDKey(),
		CreatedAt:  time.Now().Format(util.ISO8601),
		CreatedBy:  &createdBy,
	}
	if _, err := createRecord(ctx, acct.client, "place.stream.key", acct.DID, &streamKey); err != nil {
		return "", fmt.Errorf("register stream key: %w", err)
	}
	return priv, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func runE2E(ctx context.Context, devEnvPath, httpsPDSHost, httpsStationHost string, httpsPort int, appBundleID string) error {
	// Ctrl-C / SIGTERM must unwind through the normal path, or none of the
	// teardown below runs: Go's default handling exits immediately, which left
	// the dev-env node, the forked node and the temp data dir behind.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Claim the HTTPS listeners first: failing to bind 443 should stop us
	// before anything else has started.
	var tlsEnv *e2eHTTPS
	handleDomain := "test"
	if httpsPDSHost != "" {
		var err error
		tlsEnv, err = newE2EHTTPS(httpsPDSHost, httpsStationHost, httpsPort)
		if err != nil {
			return err
		}
		defer tlsEnv.Close()
		handleDomain = httpsPDSHost
	}

	// Start the Node.js PDS/PLC dev environment.
	devEnvCmd := exec.CommandContext(ctx, "node", devEnvPath)
	if tlsEnv != nil {
		devEnvCmd.Env = append(os.Environ(), tlsEnv.DevEnvEnv()...)
	}
	devEnvCmd.Stderr = os.Stderr
	devEnvStdout, err := devEnvCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("dev-env stdout pipe: %w", err)
	}
	if err := devEnvCmd.Start(); err != nil {
		return fmt.Errorf("start dev-env: %w", err)
	}
	defer devEnvCmd.Process.Kill() //nolint:errcheck

	// The PDS logs JSON to stdout too when LOG_ENABLED is set, so skip lines
	// until the one that carries the URLs.
	var env e2eDevEnv
	scanner := bufio.NewScanner(devEnvStdout)
	for env.PDSURL == "" {
		if !scanner.Scan() {
			return fmt.Errorf("dev-env exited without printing its URLs: %w", scanner.Err())
		}
		log.Log(ctx, "dev-env: "+scanner.Text())
		_ = json.Unmarshal(scanner.Bytes(), &env) //nolint:errcheck // not every line is ours
	}
	go func() {
		for scanner.Scan() {
			log.Log(ctx, "dev-env: "+scanner.Text())
		}
	}()

	// Create the test accounts on the local PDS: the one whose stream flows
	// watch, and one that streams through a deprecated ingest hostname.
	acct, err := createE2EAccount(ctx, env.PDSURL, handleDomain)
	if err != nil {
		return err
	}
	deprecatedHostAcct, err := createE2EAccount(ctx, env.PDSURL, handleDomain)
	if err != nil {
		return err
	}

	if env.LexiconDID != "" {
		if err := publishLexicons(ctx, env); err != nil {
			return fmt.Errorf("publish lexicons: %w", err)
		}
	}

	// Pick free ports for the node.
	httpPort, err := freePort()
	if err != nil {
		return err
	}
	internalPort, err := freePort()
	if err != nil {
		return err
	}
	rtmpPort, err := freePort()
	if err != nil {
		return err
	}
	httpAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	broadcasterHost := httpAddr
	if tlsEnv != nil {
		broadcasterHost = httpsStationHost
		pdsURL, err := url.Parse(env.PDSURL)
		if err != nil {
			return fmt.Errorf("parse dev-env pds url: %w", err)
		}
		plcURL, err := url.Parse(env.PLCURL)
		if err != nil {
			return fmt.Errorf("parse dev-env plc url: %w", err)
		}
		tlsEnv.Serve(ctx, pdsURL.Host, plcURL.Host, httpAddr)
	}

	// Fork ourselves as a Streamplace server node.
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	dataDir, err := os.MkdirTemp("", "streamplace-e2e-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir) //nolint:errcheck

	nodeCmd := exec.CommandContext(ctx, self)
	// Inherit the parent environment (dev builds need LD_LIBRARY_PATH etc.)
	// but strip any SP_ vars so the node only gets our config.
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "SP_") {
			nodeCmd.Env = append(nodeCmd.Env, kv)
		}
	}
	nodeCmd.Env = append(nodeCmd.Env,
		fmt.Sprintf("SP_HTTP_ADDR=%s", httpAddr),
		fmt.Sprintf("SP_HTTP_INTERNAL_ADDR=127.0.0.1:%d", internalPort),
		fmt.Sprintf("SP_RTMP_ADDR=127.0.0.1:%d", rtmpPort),
		fmt.Sprintf("SP_RELAY_HOST=%s", strings.ReplaceAll(env.PDSURL, "http://", "ws://")),
		fmt.Sprintf("SP_PLC_URL=%s", env.PLCURL),
		fmt.Sprintf("SP_DATA_DIR=%s", dataDir),
		fmt.Sprintf("SP_DEV_ACCOUNT_CREDS=%s=%s", acct.DID, acct.Password),
		fmt.Sprintf("SP_BROADCASTER_HOST=%s", broadcasterHost),
		fmt.Sprintf("SP_WEBSOCKET_URL=ws://%s", httpAddr),
		"SP_STREAM_SESSION_TIMEOUT=30s",
		"SP_TRUST_PRIVATE_NETWORK=true",
		// This node's old name, as stream.place is for rtmp.stream.place:
		// the second account's encoder streams to it.
		"SP_DEPRECATED_INGEST_HOSTS="+e2eDeprecatedIngestHost,
	)
	if tlsEnv != nil {
		nodeCmd.Env = append(nodeCmd.Env, tlsEnv.NodeEnv()...)
	}
	if appBundleID != "" {
		nodeCmd.Env = append(nodeCmd.Env, "SP_APP_BUNDLE_ID="+appBundleID)
	}
	nodeCmd.Stdout = os.Stderr
	nodeCmd.Stderr = os.Stderr
	// Own process group, so cleanup can take the whole tree down at once.
	setNodeProcessGroup(nodeCmd)
	if err := nodeCmd.Start(); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	// Kill the node *and* the ingest workers it detaches; the plain
	// Process.Kill that used to be here left those behind.
	defer killNode(ctx, nodeCmd, acct.DID, deprecatedHostAcct.DID)

	// Wait for the node to be ready. Bounded, so a node that never comes up
	// reports why instead of hanging the harness (and `make provision`) with
	// no output forever.
	healthURL := fmt.Sprintf("http://%s/api/healthz", httpAddr)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(nodeReadyTimeout)
	ready := false
	var lastErr error
	for !ready {
		if err := ctx.Err(); err != nil {
			return err
		}
		resp, err := httpClient.Get(healthURL)
		switch {
		case err != nil:
			lastErr = err
		case resp.StatusCode == http.StatusOK:
			// Close every response, not just the happy one: a body left open
			// holds its connection out of the pool, so a node answering
			// non-200 exhausts them.
			resp.Body.Close()
			ready = true
		default:
			lastErr = fmt.Errorf("GET %s: %s", healthURL, resp.Status)
			resp.Body.Close()
		}
		if !ready {
			if time.Now().After(deadline) {
				if lastErr == nil {
					lastErr = errors.New("no response")
				}
				return fmt.Errorf("node was not ready after %s: %w", nodeReadyTimeout, lastErr)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Register stream keys for the test accounts.
	priv, err := registerStreamKey(ctx, acct)
	if err != nil {
		return err
	}
	deprecatedHostPriv, err := registerStreamKey(ctx, deprecatedHostAcct)
	if err != nil {
		return err
	}
	// Create a livestream record so the stream shows up in feeds; normally the
	// app does this via place.stream.live.startLivestream when a user goes
	// live. The node keeps lastSeenAt fresh on our behalf via
	// SP_DEV_ACCOUNT_CREDS while segments are ingesting.
	now := time.Now().UTC().Format(util.ISO8601)
	livestream := placestream.Livestream{
		LexiconTypeID: "place.stream.livestream",
		CreatedAt:     now,
		LastSeenAt:    &now,
		Title:         "e2e test stream",
	}
	if _, err := createRecord(ctx, acct.client, "place.stream.livestream", acct.DID, &livestream); err != nil {
		return fmt.Errorf("create livestream record: %w", err)
	}
	// And a VOD, so flows have a video page to open. It has no source tracks:
	// its metadata (title, author) loads, but there is nothing to play.
	video := placestream.Video{
		LexiconTypeID: "place.stream.video",
		CreatedAt:     now,
		Title:         "e2e test video",
		DurationMs:    10_000,
		Source: placestream.Video_Source{
			MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        []spcomatproto.RepoStrongRef{},
			},
		},
	}
	videoURI, err := createRecord(ctx, acct.client, "place.stream.video", acct.DID, &video)
	if err != nil {
		return fmt.Errorf("create video record: %w", err)
	}

	// Give the node a moment to index the key before we start streaming.
	time.Sleep(1 * time.Second)

	// Stream a test fixture in a loop so it outlasts any Maestro test run.
	fixture := remote.RemoteFixture("3188c071b354f2e548d7f2d332699758e8e3ab1600280e5b07cb67eedc64f274/BigBuckBunny_1sGOP_240p30_NoBframes.mp4")
	g, streamCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			select {
			case <-streamCtx.Done():
				return nil
			default:
			}
			whip := &WHIPClient{
				StreamKey: priv,
				File:      fixture,
				Endpoint:  fmt.Sprintf("http://%s", httpAddr),
				Count:     1,
			}
			if err := whip.WHIP(streamCtx); err != nil && streamCtx.Err() == nil {
				log.Log(streamCtx, "whip stream ended, restarting", "err", err)
			}
			// Brief pause between restarts so we don't spin on errors.
			select {
			case <-streamCtx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
			}
		}
	})
	// The second account streams RTMP, as OBS does, through the deprecated
	// hostname, so its dashboard warns about it. It has no livestream record,
	// so it stays out of the feeds the other flows open streams from.
	deprecatedHostURL := fmt.Sprintf("rtmp://%s/live/%s", net.JoinHostPort(e2eDeprecatedIngestHost, strconv.Itoa(rtmpPort)), deprecatedHostPriv)
	g.Go(func() error {
		for {
			if err := rtmpPublishFile(streamCtx, fixture, deprecatedHostURL); err != nil && streamCtx.Err() == nil {
				log.Log(streamCtx, "rtmp stream ended, restarting", "err", err)
			}
			select {
			case <-streamCtx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
			}
		}
	})

	// Print the env vars for the workflow to consume, in one write: callers
	// poll for SERVER_URL and then read the whole file.
	vars := fmt.Sprintf("SERVER_URL=http://%s\nACCOUNT_HANDLE=%s\nACCOUNT_DID=%s\nACCOUNT_PASSWORD=%s\nVIDEO_URI=%s\n",
		httpAddr, acct.Handle, acct.DID, acct.Password, videoURI)
	vars += fmt.Sprintf("DEPRECATED_HOST_ACCOUNT_HANDLE=%s\nDEPRECATED_HOST_ACCOUNT_DID=%s\nDEPRECATED_HOST_ACCOUNT_PASSWORD=%s\n",
		deprecatedHostAcct.Handle, deprecatedHostAcct.DID, deprecatedHostAcct.Password)
	if tlsEnv != nil {
		// The same node over HTTPS at its public name, plus what clients
		// need to reach and trust it (see e2e_https.go): a browser pins the
		// leaf by SPKI, a device installs the CA.
		vars += fmt.Sprintf("SERVER_HTTPS_URL=%s\nPDS_HTTPS_URL=%s\nE2E_PROXY_URL=%s\nE2E_TLS_SPKI=%s\nE2E_TLS_CA=%s\n",
			tlsEnv.StationURL(), tlsEnv.PDSURL(), tlsEnv.ProxyURL(), tlsEnv.spki, tlsEnv.caPath)
	}
	fmt.Print(vars)

	<-ctx.Done()
	return g.Wait()
}

// publishLexicons writes this build's lexicons, OAuth permission sets
// included, into the dev-env lexicon authority account: the local version of
// what the account behind _lexicon.stream.place holds in production. The PDS
// resolves `include:place.stream.authFull` from it when the app logs in.
func publishLexicons(ctx context.Context, env e2eDevEnv) error {
	xrpcc := &xrpc.Client{Host: env.PDSURL, Client: &aqhttp.TrustedClient}
	session, err := comatproto.ServerCreateSession(ctx, xrpcc, &comatproto.ServerCreateSession_Input{
		Identifier: env.LexiconDID,
		Password:   env.LexiconPassword,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	xrpcc.Auth = &xrpc.AuthInfo{Did: session.Did, AccessJwt: session.AccessJwt, RefreshJwt: session.RefreshJwt}
	lexs, err := atproto.PublishedLexicons(ctx)
	if err != nil {
		return err
	}
	for _, lex := range lexs {
		rkey := lex.ID
		inp := spcomatproto.RepoCreateRecord_Input{
			Collection: "com.atproto.lexicon.schema",
			Repo:       session.Did,
			Rkey:       &rkey,
			Record:     &glex.LexiconTypeDecoder{Val: &atproto.SchemaFileWrapper{SchemaFile: *lex}},
		}
		out := spcomatproto.RepoCreateRecord_Output{}
		if err := xrpcc.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, inp, &out); err != nil {
			return fmt.Errorf("%s: %w", lex.ID, err)
		}
	}
	return nil
}
