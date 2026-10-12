package cmd

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	atmoq "github.com/streamplace/atmoq/go"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// Every child gets a process group, including the PDS and the real MoQ bridge.
// Reap it after teardown; exec.CommandContext alone only kills the direct child.
func stopE2EProcess(ctx context.Context, cmd *exec.Cmd, did string) {
	killNode(context.WithoutCancel(ctx), cmd, did)
	_ = cmd.Wait() //nolint:errcheck // killed during normal harness teardown
}

func e2eNodeEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "SP_") {
			env = append(env, kv)
		}
	}
	return env
}

func waitE2EHealth(ctx context.Context, healthURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(nodeReadyTimeout)
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		resp, err := client.Get(healthURL)
		if err != nil {
			lastErr = err
		} else {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("GET %s: %s", healthURL, resp.Status)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service was not ready after %s: %w", nodeReadyTimeout, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func startE2EBridge(ctx context.Context, tlsEnv *e2eHTTPS, pdsURL, binary string) (*exec.Cmd, string, error) {
	// QUIC binds UDP, not TCP. Reserve an unused UDP port before handing it to
	// the released bridge, rather than guessing from the node's TCP ports.
	if err := tlsEnv.BridgeTLS(); err != nil {
		return nil, "", fmt.Errorf("write bridge TLS: %w", err)
	}
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	addr := ln.LocalAddr().String()
	ln.Close() //nolint:errcheck
	wsURL := strings.Replace(pdsURL, "http://", "ws://", 1)
	cmd := exec.CommandContext(ctx, binary, "serve",
		"--relay-host", wsURL,
		"--server-bind", addr,
		"--tls-cert", tlsEnv.leafPath,
		"--tls-key", tlsEnv.keyPath,
		"--web-bind", "", "--web-tls-bind", "",
		"--group-store-dir", filepath.Join(tlsEnv.dir, "atmoq-store"),
		"--cursor-file", filepath.Join(tlsEnv.dir, "atmoq-cursor"),
		"--group-size", "1")
	bridgeLog, err := os.OpenFile(filepath.Join(tlsEnv.dir, "atmoq.log"), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, "", err
	}
	go func() {
		<-ctx.Done()
		bridgeLog.Close() //nolint:errcheck
	}()
	cmd.Env = append(os.Environ(), "RUST_LOG=info")
	cmd.Stdout = io.MultiWriter(os.Stderr, bridgeLog)
	cmd.Stderr = cmd.Stdout
	setNodeProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start released atmoq bridge (%s): %w", binary, err)
	}
	// Raw MoQ on the released Rust server requires SNI; Go correctly omits
	// SNI for an IP literal. Dial the verified fixture DNS name instead.
	_, port, _ := net.SplitHostPort(addr)
	relayURL := "moqt://" + net.JoinHostPort(tlsEnv.stationHost, port)
	caPEM, err := os.ReadFile(tlsEnv.caPath)
	if err != nil {
		stopE2EProcess(ctx, cmd, "")
		return nil, "", err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		stopE2EProcess(ctx, cmd, "")
		return nil, "", fmt.Errorf("parse bridge CA")
	}
	deadline := time.Now().Add(nodeReadyTimeout)
	for {
		dialCtx, cancel := context.WithTimeout(ctx, time.Second)
		session, err := atmoq.Dial(dialCtx, relayURL, &atmoq.Options{TLSConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}})
		cancel()
		if err == nil {
			session.Close()
			return cmd, relayURL, nil
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			stopE2EProcess(ctx, cmd, "")
			return nil, "", fmt.Errorf("dial real atmoq bridge: %w", err)
		}
	}
}

type e2eLocalNode struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	addr    string
	dataDir string
}

func (n *e2eLocalNode) Close(ctx context.Context, did string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	stopE2EProcess(ctx, n.cmd, did)
	os.RemoveAll(n.dataDir) //nolint:errcheck
}

// Control reads the harness's own stdin, never the node's HTTP listener.
// Restart retains ports, data and genuine OAuth credentials, then acknowledges
// readiness in a run-private file for the browser lifecycle scenario.
func (n *e2eLocalNode) Control(ctx context.Context, did string) {
	scanner := bufio.NewScanner(os.Stdin)
	generation := 0
	for scanner.Scan() {
		if scanner.Text() != "restart-local" {
			continue
		}
		n.mu.Lock()
		old := n.cmd
		stopE2EProcess(ctx, old, did)
		n.cmd = exec.CommandContext(ctx, old.Path, old.Args[1:]...)
		n.cmd.Env = old.Env
		n.cmd.Stdout = old.Stdout
		n.cmd.Stderr = old.Stderr
		setNodeProcessGroup(n.cmd)
		err := n.cmd.Start()
		if err == nil {
			err = waitE2EHealth(ctx, "http://"+n.addr+"/api/healthz")
		}
		n.mu.Unlock()
		generation++
		result := struct {
			Generation int    `json:"generation"`
			Error      string `json:"error,omitempty"`
		}{Generation: generation}
		if err != nil {
			result.Error = err.Error()
		}
		data, err := json.Marshal(result)
		if err == nil {
			tmp := filepath.Join(n.dataDir, "restart.json.tmp")
			err = os.WriteFile(tmp, data, 0o600)
			if err == nil {
				err = os.Rename(tmp, filepath.Join(n.dataDir, "restart.json"))
			}
		}
		if err != nil {
			log.Error(ctx, "write harness restart acknowledgment", "err", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func startE2ELocalNode(ctx context.Context, self string, tlsEnv *e2eHTTPS, env e2eDevEnv, did, appBundleID string) (*e2eLocalNode, error) {
	ports := make([]int, 3)
	for i := range ports {
		port, err := freePort()
		if err != nil {
			return nil, err
		}
		ports[i] = port
	}
	proxyURL, err := tlsEnv.StartLocalProxy(ctx, env.PDSURL)
	if err != nil {
		return nil, fmt.Errorf("start local PDS observer: %w", err)
	}
	dir, err := os.MkdirTemp("", "streamplace-e2e-local-*")
	if err != nil {
		return nil, err
	}
	n := &e2eLocalNode{addr: fmt.Sprintf("127.0.0.1:%d", ports[0]), dataDir: dir}
	n.cmd = exec.CommandContext(ctx, self)
	n.cmd.Env = append(e2eNodeEnv(),
		"SP_HTTP_ADDR="+n.addr,
		fmt.Sprintf("SP_HTTP_INTERNAL_ADDR=127.0.0.1:%d", ports[1]),
		fmt.Sprintf("SP_RTMP_ADDR=127.0.0.1:%d", ports[2]),
		"SP_DATA_DIR="+dir,
		"SP_BROADCASTER_HOST="+n.addr,
		"SP_WEBSOCKET_URL=ws://"+n.addr,
		"SP_PLC_URL="+env.PLCURL,
		"SP_UPSTREAM_STATION=did:web:"+tlsEnv.stationHost,
		"SP_TRUST_PRIVATE_NETWORK=true",
		"SP_ADMIN_DIDS="+did,
		"SP_STREAM_SESSION_TIMEOUT=30s",
		"SSL_CERT_DIR="+tlsEnv.trustDir,
		"HTTPS_PROXY="+proxyURL)
	if appBundleID != "" {
		n.cmd.Env = append(n.cmd.Env, "SP_APP_BUNDLE_ID="+appBundleID)
	}
	n.cmd.Stdout = os.Stderr
	n.cmd.Stderr = os.Stderr
	setNodeProcessGroup(n.cmd)
	if err := n.cmd.Start(); err != nil {
		os.RemoveAll(dir) //nolint:errcheck
		return nil, fmt.Errorf("start loopback upstream client node: %w", err)
	}
	if err := waitE2EHealth(ctx, "http://"+n.addr+"/api/healthz"); err != nil {
		n.Close(ctx, did)
		return nil, err
	}
	return n, nil
}

type e2eUnrelatedAccount struct {
	did       string
	handle    string
	accessJWT string
}

func createE2EUnrelatedAccount(ctx context.Context, pdsURL, handleDomain string) (*e2eUnrelatedAccount, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	handle := "unrelated-" + id.String()[:8] + "." + handleDomain
	password := "test"
	email := handle + "@example.com"
	client := &xrpc.Client{Host: pdsURL, Client: &aqhttp.TrustedClient}
	account, err := comatproto.ServerCreateAccount(ctx, client, &comatproto.ServerCreateAccount_Input{Handle: handle, Email: &email, Password: &password})
	if err != nil {
		return nil, fmt.Errorf("create unrelated account: %w", err)
	}
	session, err := comatproto.ServerCreateSession(ctx, client, &comatproto.ServerCreateSession_Input{Identifier: handle, Password: password})
	if err != nil {
		return nil, fmt.Errorf("create unrelated session: %w", err)
	}
	client.Auth = &xrpc.AuthInfo{Did: account.Did, AccessJwt: session.AccessJwt}
	// This produces a real unrelated firehose commit before local OAuth login.
	// The browser spec emits another after login using this account's PDS JWT.
	if _, err := createRecord(ctx, client, "place.stream.livestream", account.Did, &placestream.Livestream{
		LexiconTypeID: "place.stream.livestream",
		CreatedAt:     time.Now().UTC().Format(util.ISO8601),
		Title:         "unrelated e2e stream must not be indexed locally",
	}); err != nil {
		return nil, fmt.Errorf("seed unrelated record: %w", err)
	}
	return &e2eUnrelatedAccount{did: account.Did, handle: handle, accessJWT: session.AccessJwt}, nil
}
