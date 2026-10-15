package multitest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/util"
	scraper "github.com/starttoaster/prometheus-exporter-scraper"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/cmd"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/test/remote"
)

func TestMultinodeSyndication(t *testing.T) {
	if os.Getenv("GITHUB_ACTION") != "" {
		t.Skip("Skipping multitest in GitHub Actions")
	}
	gstinit.InitGST()
	dev := devenv.WithDevEnv(t)
	acct1 := dev.CreateAccount(t)
	acct2 := dev.CreateAccount(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	node1 := startStreamplaceNode(ctx, "node1", t, dev)
	node2 := startStreamplaceNode(ctx, "node2", t, dev)
	node3 := startStreamplaceNode(ctx, "node3", t, dev)
	node1.StartStream(t, acct1)
	node2.PlayStream(t, acct1)
	node3.PlayStream(t, acct1)
	<-time.After(10 * time.Second)
	node2.Shutdown(t)
	<-time.After(20 * time.Second)
	node4 := startStreamplaceNode(ctx, "node4", t, dev)
	node4.StartStream(t, acct2)
	node4.PlayStream(t, acct1)
	node1.PlayStream(t, acct2)
	node3.PlayStream(t, acct2)
	<-time.After(30 * time.Second)
}

func TestOriginSwap(t *testing.T) {
	if os.Getenv("GITHUB_ACTION") != "" {
		t.Skip("Skipping multitest in GitHub Actions")
	}
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dev := devenv.WithDevEnv(t)
	acct1 := dev.CreateAccount(t)
	acct2 := dev.CreateAccount(t)
	node1 := startStreamplaceNode(ctx, "node1", t, dev)
	node2 := startStreamplaceNode(ctx, "node2", t, dev)
	node3 := startStreamplaceNode(ctx, "node3", t, dev)
	// node4 := startStreamplaceNode(ctx, "node4", t, dev)
	node1.StartStream(t, acct1)
	node2.StartStream(t, acct2)
	node1.PlayStream(t, acct1)
	node2.PlayStream(t, acct1)
	node3.PlayStream(t, acct1)
	node1.PlayStream(t, acct2)
	node2.PlayStream(t, acct2)
	node3.PlayStream(t, acct2)
	// node4.PlayStream(t, acct1)
	<-time.After(30 * time.Second)
	// node1.StopStream(t, acct1)
	// node2.StartStream(t, acct1)
	// <-time.After(20 * time.Second)
	// // node2.StopStream(t, acct1)
	// // node3.StartStream(t, acct1)
	// // node4.Shutdown(t)
	// // <-time.After(10 * time.Second)
}

var currentPort = 10000

func nextPort() int {
	currentPort++
	return currentPort
}

type TestNode struct {
	Env           map[string]string
	Dev           *devenv.DevEnv
	Cmd           *exec.Cmd
	Ctx           context.Context // don't ever do this, it's just a test
	Shutdown      func(t *testing.T)
	ActiveStreams map[string]context.CancelFunc
	Workers       *sync.WaitGroup
	Name          string
}

func startStreamplaceNode(ctx context.Context, name string, t *testing.T, dev *devenv.DevEnv) *TestNode {
	t.Helper()
	nodeCtx, nodeCancel := context.WithCancel(ctx)
	t.Cleanup(nodeCancel)
	dataDir := t.TempDir()
	devAccountCreds := []string{}
	for _, acct := range dev.Accounts {
		devAccountCreds = append(devAccountCreds, fmt.Sprintf("%s=%s", acct.DID, acct.Password))
	}
	apiPort := nextPort()
	// Every isolated node needs its own advertised loopback MoQ port.
	moqPort := nextPort()
	env := map[string]string{
		"SP_HTTP_ADDR":              fmt.Sprintf("127.0.0.1:%d", apiPort),
		"SP_HTTP_INTERNAL_ADDR":     fmt.Sprintf("127.0.0.1:%d", nextPort()),
		"SP_RTMP_ADDR":              fmt.Sprintf("127.0.0.1:%d", nextPort()),
		"SP_MOQ_ADDR":               fmt.Sprintf("127.0.0.1:%d", moqPort),
		"SP_MOQ_URL":                fmt.Sprintf("moqt://127.0.0.1:%d", moqPort),
		"SP_RELAY_HOST":             strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
		"SP_PLC_URL":                dev.PLCURL,
		"SP_DATA_DIR":               dataDir,
		"SP_DEV_ACCOUNT_CREDS":      strings.Join(devAccountCreds, ","),
		"SP_TRUST_PRIVATE_NETWORK":  "true",
		"SP_DEV_PUBLIC_OAUTH":       "true",
		"SP_SYNDICATE":              "*",
		"SP_STREAM_SESSION_TIMEOUT": "3s",
		"SP_COLOR":                  "true",
		"RUST_LOG":                  os.Getenv("RUST_LOG"),
		"SP_BROADCASTER_HOST":       fmt.Sprintf("%s.example.com", name),
		"SP_WEBSOCKET_URL":          fmt.Sprintf("ws://127.0.0.1:%d", apiPort),
	}
	_, file, _, _ := runtime.Caller(0)
	buildDir, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", fmt.Sprintf("build-%s-%s", runtime.GOOS, runtime.GOARCH)))
	require.NoErrorf(t, err, "[%s] failed to resolve absolute build path", name)
	abs := filepath.Join(buildDir, "libstreamplace")
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		abs = filepath.Join(buildDir, "streamplace")
	} else {
		require.NoErrorf(t, err, "[%s] failed to inspect development binary", name)
		// Bypass the development shell launcher: it invokes Go and enables
		// frontend proxies that these isolated nodes do not need.
		env["LD_LIBRARY_PATH"] = filepath.Join(buildDir, "lib")
		if runtime.GOOS == "darwin" {
			env["DYLD_LIBRARY_PATH"] = filepath.Join(buildDir, "lib")
		}
	}
	cmd := exec.CommandContext(nodeCtx, abs)
	cmd.WaitDelay = 5 * time.Second
	// Keep runtime tools and libraries, but never inherit another node's config.
	cmd.Env = []string{}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if _, overridden := env[key]; !strings.HasPrefix(key, "SP_") && !overridden {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdoutPipe, err := cmd.StdoutPipe()
	require.NoErrorf(t, err, "[%s] failed to get stdout pipe", name)
	t.Cleanup(func() { _ = stdoutPipe.Close() })
	stderrPipe, err := cmd.StderrPipe()
	require.NoErrorf(t, err, "[%s] failed to get stderr pipe", name)
	t.Cleanup(func() { _ = stderrPipe.Close() })

	err = cmd.Start()
	require.NoErrorf(t, err, "[%s] failed to start streamplace process", name)

	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	// Goroutine to read stdout and prefix lines
	go func() {
		defer close(stdoutDone)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			fmt.Fprintf(os.Stdout, "[%s STDOUT] %s\n", name, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stdout, "[%s STDOUT] Error reading stdout: %v\n", name, err)
		}
	}()
	// Goroutine to read stderr and prefix lines
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			fmt.Fprintf(os.Stdout, "[%s STDERR] %s\n", name, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stdout, "[%s STDERR] Error reading stderr: %v\n", name, err)
		}
	}()

	processDone := make(chan struct{})
	var processErr error
	var workers sync.WaitGroup
	go func() {
		processErr = cmd.Wait()
		close(processDone)
	}()
	var shutdownOnce sync.Once
	nodeShutdown := func(t *testing.T) {
		shutdownOnce.Do(func() {
			nodeCancel()
			<-processDone
			<-stdoutDone
			<-stderrDone
			workersDone := make(chan struct{})
			go func() {
				workers.Wait()
				close(workersDone)
			}()
			select {
			case <-workersDone:
			case <-time.After(10 * time.Second):
				t.Errorf("[%s] stream workers did not stop after cancellation", name)
			}
		})
	}
	// Register before readiness so startup failures also reap the process.
	t.Cleanup(func() { nodeShutdown(t) })

	// Bound every request and the overall startup, and report an early exit.
	startupCtx, startupCancel := context.WithTimeout(nodeCtx, time.Minute)
	defer startupCancel()
	healthz := fmt.Sprintf("http://%s/api/healthz", env["SP_HTTP_ADDR"])
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		select {
		case <-processDone:
			t.Fatalf("[%s] streamplace exited before readiness: %v", name, processErr)
		case <-startupCtx.Done():
			t.Fatalf("[%s] streamplace startup failed: %v (last health check: %v)", name, startupCtx.Err(), lastErr)
		default:
		}
		req, err := http.NewRequestWithContext(startupCtx, http.MethodGet, healthz, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		lastErr = err
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
			lastErr = fmt.Errorf("GET %s: %s", healthz, resp.Status)
		}
		select {
		case <-processDone:
			t.Fatalf("[%s] streamplace exited before readiness: %v", name, processErr)
		case <-startupCtx.Done():
			t.Fatalf("[%s] streamplace startup failed: %v (last health check: %v)", name, startupCtx.Err(), lastErr)
		case <-ticker.C:
		}
	}
	node := &TestNode{
		Env:           env,
		Dev:           dev,
		Cmd:           cmd,
		Ctx:           nodeCtx,
		ActiveStreams: make(map[string]context.CancelFunc),
		Name:          name,
		Shutdown:      nodeShutdown,
		Workers:       &workers,
	}
	workers.Add(2)
	go func() {
		defer workers.Done()
		select {
		case <-nodeCtx.Done():
		case <-processDone:
			if nodeCtx.Err() == nil {
				t.Errorf("[%s] streamplace exited unexpectedly: %v", name, processErr)
				nodeCancel()
			}
		}
	}()
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			select {
			case <-nodeCtx.Done():
				return
			case <-time.After(1 * time.Second):
				scrp, err := scraper.NewWebScraper(fmt.Sprintf("http://%s/metrics", env["SP_HTTP_INTERNAL_ADDR"]))
				if err != nil {
					t.Errorf("[%s] failed to create scraper: %v", name, err)
					return
				}
				data, err := scrp.ScrapeWeb()
				if nodeCtx.Err() != nil {
					return
				}
				if err != nil {
					t.Errorf("[%s] failed to scrape metrics: %v", name, err)
					return
				}
				found := false
				for _, metric := range data.Gauges {
					if metric.Key == "streamplace_send_segment_calls" {
						// require.Lessf(t, metric.Value, float64(2), "[%s] send segment calls should be < 2, got %f", name, metric.Value)
						log.Log(nodeCtx, fmt.Sprintf("[%s] open send_segment calls", name), "open", metric.Value)
						found = true
						break
					}
				}
				if !found {
					require.Fail(t, fmt.Sprintf("[%s] send segment calls metric not found", name))
				}
			}
		}
	}()
	return node
}

func (node *TestNode) StartStream(t *testing.T, acct *devenv.DevEnvAccount) {
	streamCtx, streamCancel := context.WithCancel(node.Ctx)
	node.ActiveStreams[acct.DID] = streamCancel
	priv, pub, err := spkey.GenerateStreamKeyForDID(acct.DID)
	require.NoErrorf(t, err, "[%s] failed to generate stream key for DID %s", node.Name, acct.DID)
	createdBy := "multitest"
	streamKey := placestream.Key{
		SigningKey: pub.DIDKey(),
		CreatedAt:  time.Now().Format(util.ISO8601),
		CreatedBy:  &createdBy,
	}
	_, err = comatproto.RepoCreateRecord(context.TODO(), acct.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.key",
		Repo:       acct.DID,
		Record:     &glex.LexiconTypeDecoder{Val: &streamKey},
	})
	require.NoErrorf(t, err, "[%s] failed to create Repo record for DID %s", node.Name, acct.DID)
	log.Log(context.Background(), "created stream key", "did", acct.DID, "pub", pub.DIDKey())
	now := time.Now().UTC().Format(util.ISO8601)
	livestream := placestream.Livestream{LexiconTypeID: "place.stream.livestream", CreatedAt: now, LastSeenAt: &now, Title: "multinode transport test"}
	_, err = comatproto.RepoCreateRecord(streamCtx, acct.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.livestream",
		Repo:       acct.DID,
		Record:     &glex.LexiconTypeDecoder{Val: &livestream},
	})
	require.NoErrorf(t, err, "[%s] failed to publish livestream for DID %s", node.Name, acct.DID)
	time.Sleep(1 * time.Second)
	whip := &cmd.WHIPClient{
		StreamKey: priv,
		File:      remote.RemoteFixture("3188c071b354f2e548d7f2d332699758e8e3ab1600280e5b07cb67eedc64f274/BigBuckBunny_1sGOP_240p30_NoBframes.mp4"),
		Endpoint:  fmt.Sprintf("http://%s", node.Env["SP_HTTP_ADDR"]),
		Count:     1,
	}

	node.Workers.Add(1)
	go func() {
		defer node.Workers.Done()
		err := whip.WHIP(streamCtx)
		if err != nil && (streamCtx.Err() == nil || !errors.Is(err, context.Canceled)) {
			t.Errorf("[%s] stream ingest failed did=%s: %v", node.Name, acct.DID, err)
		} else if streamCtx.Err() == nil {
			t.Errorf("[%s] stream ingest ended unexpectedly did=%s", node.Name, acct.DID)
		}
	}()
}

func (node *TestNode) StopStream(t *testing.T, acct *devenv.DevEnvAccount) {
	cancel := node.ActiveStreams[acct.DID]
	if cancel == nil {
		require.FailNow(t, fmt.Sprintf("[%s] stream not active for did %s", node.Name, acct.DID))
	}
	cancel()
	delete(node.ActiveStreams, acct.DID)
}

func (node *TestNode) PlayStream(t *testing.T, acct *devenv.DevEnvAccount) {
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	require.Eventually(t, func() bool {
		resp, err := client.Get(fmt.Sprintf("http://%s/xrpc/place.stream.live.getLiveUsers?limit=50", node.Env["SP_HTTP_ADDR"]))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		var body placestream.LiveGetLiveUsers_Output
		if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&body) != nil {
			return false
		}
		for _, stream := range body.Streams {
			if stream.Author.Did == acct.DID {
				return true
			}
		}
		return false
	}, time.Minute, 200*time.Millisecond, "target node must receive the stream before playback")
	whep := &cmd.WHEPClient{
		Endpoint: fmt.Sprintf("http://%s/api/playback/%s/webrtc", node.Env["SP_HTTP_ADDR"], acct.DID),
		Count:    1,
	}
	g, ctx := errgroup.WithContext(node.Ctx)
	g.Go(func() error {
		return whep.WHEP(ctx)
	})
	start := time.Now()
	// start at -1 to give them an extra go-round to boot
	prevVideoTotal := -1
	prevAudioTotal := -1
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				stats := whep.Stats[0]
				videoStats := stats["video"]
				audioStats := stats["audio"]
				if videoStats.Total == prevVideoTotal || audioStats.Total == prevAudioTotal {
					return fmt.Errorf("stream playback stalled did=%s, video=%d, audio=%d, elapsed=%s", acct.DID, videoStats.Total, audioStats.Total, time.Since(start))
				}
				prevVideoTotal = videoStats.Total
				prevAudioTotal = audioStats.Total
				log.Log(ctx, fmt.Sprintf("[%s] stream playback", node.Name), "did", acct.DID, "video", videoStats.Total, "audio", audioStats.Total, "elapsed", time.Since(start))
			}
		}
	})
	node.Workers.Add(1)
	go func() {
		defer node.Workers.Done()
		err := g.Wait()
		if err != nil && (node.Ctx.Err() == nil || !errors.Is(err, context.Canceled)) {
			t.Errorf("[%s] stream playback failed did=%s: %v", node.Name, acct.DID, err)
		} else if node.Ctx.Err() == nil {
			t.Errorf("[%s] stream playback ended unexpectedly did=%s", node.Name, acct.DID)
		}
	}()
}
