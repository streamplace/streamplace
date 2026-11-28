package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
)

func e2eStationS3Env() ([]string, error) {
	for _, entry := range os.Environ() {
		// These are the fixture wrapper's explicit local-only AWS settings.
		switch entry {
		case "AWS_CONFIG_FILE=/dev/null", "AWS_SHARED_CREDENTIALS_FILE=/dev/null",
			"AWS_EC2_METADATA_DISABLED=true", "AWS_REGION=us-east-1", "AWS_DEFAULT_REGION=us-east-1":
			continue
		}
		key, value, _ := strings.Cut(entry, "=")
		if value != "" && (strings.HasPrefix(key, "AWS_") || strings.HasPrefix(key, "S3_") || strings.HasPrefix(key, "SP_S3_")) {
			return nil, fmt.Errorf("station cluster refuses ambient storage configuration %s; use SP_TEST_S3_* only", key)
		}
	}

	suffixes := [...]string{"ENDPOINT", "BUCKET", "ACCESS_KEY_ID", "SECRET_ACCESS_KEY"}
	env := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		key := "SP_TEST_S3_" + suffix
		value := os.Getenv(key)
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("station cluster requires %s for its local S3 fixture", key)
		}
		if suffix == "ENDPOINT" {
			port, ok := strings.CutPrefix(value, "http://127.0.0.1:")
			n, err := strconv.ParseUint(port, 10, 16)
			if !ok || err != nil || n == 0 {
				return nil, fmt.Errorf("%s must be http://127.0.0.1:<port>", key)
			}
		}
		if suffix == "BUCKET" && value != "station-test" {
			return nil, fmt.Errorf("%s must be station-test", key)
		}
		env = append(env, "SP_S3_"+suffix+"="+value)
	}
	return env, nil
}

func runE2EStationCluster(ctx context.Context, tlsEnv *e2eHTTPS, env e2eDevEnv, s3Env []string, creator *comatproto.ServerCreateAccount_Output, password, appBundleID, fixture, stationAddr, edgeAddr, runID string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	tmpDir, err := filepath.Abs(os.TempDir())
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp(tmpDir, "streamplace-e2e-station-cluster-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	var stateKey [32]byte
	if _, err := rand.Read(stateKey[:]); err != nil {
		return fmt.Errorf("generate station state key: %w", err)
	}
	keyPath := filepath.Join(dir, "station-state.key")
	if err := os.WriteFile(keyPath, stateKey[:], 0o600); err != nil {
		return fmt.Errorf("write station state key: %w", err)
	}
	common := append(s3Env,
		"SP_BROADCASTER_HOST="+tlsEnv.stationHost,
		"SP_STATION_DID="+env.StationDID,
		"SP_DB_URL=station://e2e-"+runID,
		"SP_STATION_STATE_KEY_FILE="+keyPath,
		"SP_ADMIN_DIDS="+creator.Did,
		"SP_DISABLE_SYNDICATION=true",
		// The fixture's HTTP endpoint must never go through an ambient proxy.
		"HTTP_PROXY=", "http_proxy=", "https_proxy=", "ALL_PROXY=", "all_proxy=", "no_proxy=",
		"NO_PROXY=127.0.0.1,localhost",
	)
	if appBundleID != "" {
		common = append(common, "SP_APP_BUNDLE_ID="+appBundleID)
	}
	common = append(common, tlsEnv.NodeEnv()...)

	start := func(name, addr, host, peerHost string, publisher bool) (*e2eLocalNode, error) {
		internalPort, err := freePort()
		if err != nil {
			return nil, err
		}
		rtmpPort, err := freePort()
		if err != nil {
			return nil, err
		}
		moqPort, err := freeUDPPort()
		if err != nil {
			return nil, err
		}
		n := &e2eLocalNode{
			addr:         addr,
			internalAddr: fmt.Sprintf("127.0.0.1:%d", internalPort),
			dataDir:      filepath.Join(dir, name),
		}
		nodeEnv := append([]string{
			"SP_HTTP_ADDR=" + n.addr,
			"SP_HTTP_INTERNAL_ADDR=" + n.internalAddr,
			fmt.Sprintf("SP_RTMP_ADDR=127.0.0.1:%d", rtmpPort),
			fmt.Sprintf("SP_MOQ_ADDR=127.0.0.1:%d", moqPort),
			"SP_SERVER_HOST=" + host,
			"SP_STATION_PEERS=did:web:" + peerHost + "=https://" + peerHost,
			"SP_WEBSOCKET_URL=ws://" + n.addr,
			"SP_STATION_ORIGIN_PUBLISHER=" + strconv.FormatBool(publisher),
		}, common...)
		n.cmd, err = startNode(ctx, self, env, n.dataDir, nodeEnv)
		if err != nil {
			return nil, err
		}
		if err := waitE2EHealth(ctx, "http://"+n.addr+"/api/healthz"); err != nil {
			n.Close(ctx, "")
			return nil, err
		}
		return n, nil
	}

	station, err := start("station", stationAddr, tlsEnv.stationHost, tlsEnv.edgeHost, true)
	if err != nil {
		return fmt.Errorf("station cluster origin: %w", err)
	}
	defer station.Close(ctx, "")
	edge, err := start("edge", edgeAddr, tlsEnv.edgeHost, tlsEnv.stationHost, false)
	if err != nil {
		return fmt.Errorf("station cluster edge: %w", err)
	}
	defer edge.Close(ctx, "")
	edge.cleanOnRestart = true

	ackPath := filepath.Join(dir, "edge-restart.json")
	var vars strings.Builder
	for _, item := range [...]struct{ key, value string }{
		{"SERVER_HTTPS_URL", tlsEnv.StationURL()},
		{"SERVER_INTERNAL_URL", "http://" + station.internalAddr},
		{"SERVER2_URL", "http://" + edge.addr},
		{"SERVER2_HTTPS_URL", "https://" + tlsEnv.edgeHost},
		{"SERVER2_INTERNAL_URL", "http://" + edge.internalAddr},
		{"ACCOUNT_HANDLE", creator.Handle},
		{"ACCOUNT_PASSWORD", password},
		{"ACCOUNT_DID", creator.Did},
		{"E2E_FIXTURE_MP4", fixture},
		{"PDS_HTTP_URL", env.PDSURL},
		{"PDS_HTTPS_URL", tlsEnv.PDSURL()},
		{"PDS_KIND", e2eStationPDSKind},
		{"E2E_PROXY_URL", tlsEnv.ProxyURL()},
		{"E2E_TLS_SPKI", tlsEnv.spki},
		{"E2E_TLS_CA", tlsEnv.caPath},
		{"E2E_STATION_CLUSTER", "true"},
		{"E2E_STATION_DID", env.StationDID},
		{"E2E_STATION_HANDLE", env.StationHandle},
		{"E2E_STATION_PASSWORD", env.StationPassword},
		{"E2E_STATION_EDGE_RESTART", ackPath},
		// The shell polls for this line before sourcing the whole file.
		{"SERVER_URL", "http://" + station.addr},
	} {
		fmt.Fprintf(&vars, "%s='%s'\n", item.key, strings.ReplaceAll(item.value, "'", "'\\''"))
	}
	if _, err := io.WriteString(os.Stdout, vars.String()); err != nil {
		return fmt.Errorf("write station cluster environment: %w", err)
	}
	go edge.Control(ctx, "", "restart-station-edge", ackPath)
	<-ctx.Done()
	return nil
}
