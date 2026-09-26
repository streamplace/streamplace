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
	"path/filepath"
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
	"stream.place/streamplace/pkg/branding"
	spcomatproto "stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/test/remote"
)

// createRecord writes a glex record into the given repo via the public
// com.atproto.repo.createRecord XRPC. The repo's own generated comatproto
// types are used (rather than indigo's) so the record's $type survives
// marshalling: indigo's LexiconTypeDecoder only accepts structs with a const
// $type field, which glex-generated records do not have.
func createRecord(ctx context.Context, client *xrpc.Client, collection, repo string, record glex.Record) error {
	inp := spcomatproto.RepoCreateRecord_Input{
		Collection: collection,
		Repo:       repo,
		Record:     &glex.LexiconTypeDecoder{Val: record},
	}
	out := spcomatproto.RepoCreateRecord_Output{}
	return client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, inp, &out)
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
			// What an admin granting the premium feature would do, done
			// before the node starts so no flow has to log in as an admin.
			&urfavecli.StringSliceFlag{
				Name:    "custom-domain",
				Usage:   "grant this hostname to the test account as a custom domain (repeatable); requests to the node under it get the account's place.stream.branding.brand record for it",
				Sources: urfavecli.EnvVars("SP_E2E_CUSTOM_DOMAINS"),
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
			return runE2E(ctx, cmd.String("dev-env"), pdsHost, stationHost, cmd.StringSlice("custom-domain"))
		},
	}
}

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

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func runE2E(ctx context.Context, devEnvPath, httpsPDSHost, httpsStationHost string, customDomains []string) error {
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
		tlsEnv, err = newE2EHTTPS(httpsPDSHost, httpsStationHost)
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

	// Create a test account on the local PDS.
	xrpcc := &xrpc.Client{Host: env.PDSURL, Client: &aqhttp.TrustedClient}
	uu, err := uuid.NewRandom()
	if err != nil {
		return err
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
		return fmt.Errorf("create account: %w", err)
	}
	session, err := comatproto.ServerCreateSession(ctx, xrpcc, &comatproto.ServerCreateSession_Input{
		Identifier: out.Handle,
		Password:   password,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	xrpcc = &xrpc.Client{
		Host:   env.PDSURL,
		Client: &aqhttp.TrustedClient,
		Auth: &xrpc.AuthInfo{
			Did:        out.Did,
			AccessJwt:  session.AccessJwt,
			RefreshJwt: session.RefreshJwt,
			Handle:     out.Handle,
		},
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
	if err := seedCustomDomains(ctx, dataDir, out.Did, customDomains); err != nil {
		return fmt.Errorf("seed custom domains: %w", err)
	}

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
		fmt.Sprintf("SP_DEV_ACCOUNT_CREDS=%s=%s", out.Did, password),
		// The test account runs the node, so flows can reach admin screens
		// (branding, custom domains) once logged in.
		fmt.Sprintf("SP_ADMIN_DIDS=%s", out.Did),
		fmt.Sprintf("SP_BROADCASTER_HOST=%s", broadcasterHost),
		fmt.Sprintf("SP_WEBSOCKET_URL=ws://%s", httpAddr),
		"SP_STREAM_SESSION_TIMEOUT=30s",
		"SP_TRUST_PRIVATE_NETWORK=true",
	)
	if tlsEnv != nil {
		nodeCmd.Env = append(nodeCmd.Env, tlsEnv.NodeEnv()...)
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
	defer killNode(ctx, nodeCmd, out.Did)

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

	// Register a stream key for the test account.
	priv, pub, err := spkey.GenerateStreamKeyForDID(out.Did)
	if err != nil {
		return fmt.Errorf("generate stream key: %w", err)
	}
	createdBy := "e2e"
	streamKey := placestream.Key{
		SigningKey: pub.DIDKey(),
		CreatedAt:  time.Now().Format(util.ISO8601),
		CreatedBy:  &createdBy,
	}
	if err := createRecord(ctx, xrpcc, "place.stream.key", out.Did, &streamKey); err != nil {
		return fmt.Errorf("register stream key: %w", err)
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
	if err := createRecord(ctx, xrpcc, "place.stream.livestream", out.Did, &livestream); err != nil {
		return fmt.Errorf("create livestream record: %w", err)
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

	// Print the env vars for the workflow to consume, in one write: callers
	// poll for SERVER_URL and then read the whole file.
	vars := fmt.Sprintf("SERVER_URL=http://%s\nACCOUNT_HANDLE=%s\nACCOUNT_DID=%s\nACCOUNT_PASSWORD=%s\nPDS_URL=%s\n",
		httpAddr, out.Handle, out.Did, password, env.PDSURL)
	if len(customDomains) > 0 {
		vars += fmt.Sprintf("CUSTOM_DOMAINS=%s\n", strings.Join(customDomains, ","))
	}
	if tlsEnv != nil {
		// The same node over HTTPS at its public name, plus what a browser
		// needs to reach and trust it (see e2e_https.go).
		vars += fmt.Sprintf("SERVER_HTTPS_URL=%s\nPDS_HTTPS_URL=%s\nE2E_PROXY_URL=%s\nE2E_TLS_SPKI=%s\n",
			tlsEnv.StationURL(), tlsEnv.PDSURL(), tlsEnv.ProxyURL(), tlsEnv.spki)
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

// seedCustomDomains grants hostnames to the test account in the node's
// state database before the node first opens it.
func seedCustomDomains(ctx context.Context, dataDir, did string, hosts []string) error {
	if len(hosts) == 0 {
		return nil
	}
	cli := &config.CLI{DataDir: dataDir, DBURL: "sqlite://" + filepath.Join(dataDir, "state.sqlite")}
	state, err := statedb.MakeDB(ctx, cli, nil, nil)
	if err != nil {
		return err
	}
	for _, h := range hosts {
		host := branding.NormalizeHostname(h)
		if host == "" {
			return fmt.Errorf("%q is not a hostname", h)
		}
		if _, err := state.PutBrandingDomain(host, did); err != nil {
			return err
		}
	}
	sqlDB, err := state.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
