package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	urfavecli "github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
	"stream.place/streamplace/test/remote"
)

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
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return runE2E(ctx, cmd.String("dev-env"))
		},
	}
}

type e2eDevEnv struct {
	PDSURL string `json:"pds-url"`
	PLCURL string `json:"plc-url"`
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func runE2E(ctx context.Context, devEnvPath string) error {
	// Start the Node.js PDS/PLC dev environment.
	devEnvCmd := exec.CommandContext(ctx, "node", devEnvPath)
	devEnvCmd.Stderr = os.Stderr
	devEnvStdout, err := devEnvCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("dev-env stdout pipe: %w", err)
	}
	if err := devEnvCmd.Start(); err != nil {
		return fmt.Errorf("start dev-env: %w", err)
	}
	defer devEnvCmd.Process.Kill() //nolint:errcheck

	var env e2eDevEnv
	scanner := bufio.NewScanner(devEnvStdout)
	scanner.Scan()
	if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
		return fmt.Errorf("parse dev-env output: %w", err)
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
	handle := fmt.Sprintf("sp-%s.test", uu.String()[:8])
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
		fmt.Sprintf("SP_DEV_ACCOUNT_CREDS=%s=%s", out.Did, password),
		fmt.Sprintf("SP_BROADCASTER_HOST=127.0.0.1:%d", httpPort),
		fmt.Sprintf("SP_WEBSOCKET_URL=ws://%s", httpAddr),
		"SP_STREAM_SESSION_TIMEOUT=30s",
		"SP_TRUST_PRIVATE_NETWORK=true",
	)
	nodeCmd.Stdout = os.Stderr
	nodeCmd.Stderr = os.Stderr
	if err := nodeCmd.Start(); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	defer nodeCmd.Process.Kill() //nolint:errcheck

	// Wait for the node to be ready.
	healthURL := fmt.Sprintf("http://%s/api/healthz", httpAddr)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := httpClient.Get(healthURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Register a stream key for the test account.
	priv, pub, err := spkey.GenerateStreamKeyForDID(out.Did)
	if err != nil {
		return fmt.Errorf("generate stream key: %w", err)
	}
	createdBy := "e2e"
	streamKey := streamplace.Key{
		SigningKey: pub.DIDKey(),
		CreatedAt:  time.Now().Format(util.ISO8601),
		CreatedBy:  &createdBy,
	}
	if _, err := comatproto.RepoCreateRecord(ctx, xrpcc, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.key",
		Repo:       out.Did,
		Record:     &lexutil.LexiconTypeDecoder{Val: &streamKey},
	}); err != nil {
		return fmt.Errorf("register stream key: %w", err)
	}
	// Create a livestream record so the stream shows up in feeds; normally the
	// app does this via place.stream.live.startLivestream when a user goes
	// live. The node keeps lastSeenAt fresh on our behalf via
	// SP_DEV_ACCOUNT_CREDS while segments are ingesting.
	now := time.Now().UTC().Format(util.ISO8601)
	livestream := streamplace.Livestream{
		LexiconTypeID: "place.stream.livestream",
		CreatedAt:     now,
		LastSeenAt:    &now,
		Title:         "e2e test stream",
	}
	if _, err := comatproto.RepoCreateRecord(ctx, xrpcc, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.livestream",
		Repo:       out.Did,
		Record:     &lexutil.LexiconTypeDecoder{Val: &livestream},
	}); err != nil {
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

	// Print the env vars for the workflow to consume.
	fmt.Printf("SERVER_URL=http://%s\n", httpAddr)
	fmt.Printf("ACCOUNT_HANDLE=%s\n", out.Handle)
	fmt.Printf("ACCOUNT_DID=%s\n", out.Did)

	<-ctx.Done()
	return g.Wait()
}
