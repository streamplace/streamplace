package multitest

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	scraper "github.com/starttoaster/prometheus-exporter-scraper"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/cmd"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
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
	Name          string
}

func startStreamplaceNode(ctx context.Context, name string, t *testing.T, dev *devenv.DevEnv) *TestNode {
	nodeCtx, nodeCancel := context.WithCancel(ctx)
	dataDir := t.TempDir()
	devAccountCreds := []string{}
	for _, acct := range dev.Accounts {
		devAccountCreds = append(devAccountCreds, fmt.Sprintf("%s=%s", acct.DID, acct.Password))
	}
	apiPort := nextPort()
	env := map[string]string{
		"SP_HTTP_ADDR":              fmt.Sprintf("127.0.0.1:%d", apiPort),
		"SP_HTTP_INTERNAL_ADDR":     fmt.Sprintf("127.0.0.1:%d", nextPort()),
		"SP_RTMP_ADDR":              fmt.Sprintf("127.0.0.1:%d", nextPort()),
		"SP_RELAY_HOST":             strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
		"SP_PLC_URL":                dev.PLCURL,
		"SP_DATA_DIR":               dataDir,
		"SP_DEV_ACCOUNT_CREDS":      strings.Join(devAccountCreds, ","),
		"SP_STREAM_SESSION_TIMEOUT": "3s",
		"SP_COLOR":                  "true",
		"RUST_LOG":                  os.Getenv("RUST_LOG"),
		"SP_BROADCASTER_HOST":       fmt.Sprintf("%s.example.com", name),
		"SP_WEBSOCKET_URL":          fmt.Sprintf("ws://127.0.0.1:%d", apiPort),
	}
	_, file, _, _ := runtime.Caller(0)
	buildDir := fmt.Sprintf("build-%s-%s", runtime.GOOS, runtime.GOARCH)
	abs, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", buildDir, "streamplace"))
	require.NoErrorf(t, err, "[%s] failed to resolve absolute binary path", name)
	// Run the streamplace binary at abs with the environment env
	cmd := exec.Command(abs)
	cmd.Env = []string{}
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdoutPipe, err := cmd.StdoutPipe()
	require.NoErrorf(t, err, "[%s] failed to get stdout pipe", name)
	stderrPipe, err := cmd.StderrPipe()
	require.NoErrorf(t, err, "[%s] failed to get stderr pipe", name)

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

	err = cmd.Start()
	require.NoErrorf(t, err, "[%s] failed to start streamplace process", name)

	// Wait for the streamplace node to be ready by polling the health endpoint
	healthz := fmt.Sprintf("http://%s/api/healthz", env["SP_HTTP_ADDR"])
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		resp, err := client.Get(healthz)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	node := &TestNode{
		Env:           env,
		Dev:           dev,
		Cmd:           cmd,
		Ctx:           nodeCtx,
		ActiveStreams: make(map[string]context.CancelFunc),
		Name:          name,
	}
	go func() {
		<-nodeCtx.Done()
		node.Shutdown(t)
	}()
	go func() {
		for {
			select {
			case <-nodeCtx.Done():
				return
			case <-time.After(1 * time.Second):
				scrp, err := scraper.NewWebScraper(fmt.Sprintf("http://%s/metrics", env["SP_HTTP_INTERNAL_ADDR"]))
				require.NoErrorf(t, err, "[%s] failed to create scraper", name)
				data, err := scrp.ScrapeWeb()
				require.NoErrorf(t, err, "[%s] failed to scrape metrics", name)
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
	shuttingDown := false
	nodeShutdown := func(t *testing.T) {
		if shuttingDown {
			return
		}
		shuttingDown = true
		nodeCancel()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		// Wait for stdout/stderr readers to finish
		<-stdoutDone
		<-stderrDone
	}
	node.Shutdown = nodeShutdown
	t.Cleanup(func() {
		node.Shutdown(t)
	})
	return node
}

func (node *TestNode) StartStream(t *testing.T, acct *devenv.DevEnvAccount) {
	streamCtx, streamCancel := context.WithCancel(node.Ctx)
	node.ActiveStreams[acct.DID] = streamCancel
	priv, pub, err := spkey.GenerateStreamKeyForDID(acct.DID)
	require.NoErrorf(t, err, "[%s] failed to generate stream key for DID %s", node.Name, acct.DID)
	createdBy := "multitest"
	streamKey := streamplace.Key{
		SigningKey: pub.DIDKey(),
		CreatedAt:  time.Now().Format(util.ISO8601),
		CreatedBy:  &createdBy,
	}
	_, err = comatproto.RepoCreateRecord(context.TODO(), acct.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.key",
		Repo:       acct.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: &streamKey},
	})
	require.NoErrorf(t, err, "[%s] failed to create Repo record for DID %s", node.Name, acct.DID)
	log.Log(context.Background(), "created stream key", "did", acct.DID, "pub", pub.DIDKey())
	time.Sleep(1 * time.Second)
	whip := &cmd.WHIPClient{
		StreamKey: priv,
		File:      remote.RemoteFixture("3188c071b354f2e548d7f2d332699758e8e3ab1600280e5b07cb67eedc64f274/BigBuckBunny_1sGOP_240p30_NoBframes.mp4"),
		Endpoint:  fmt.Sprintf("http://%s", node.Env["SP_HTTP_ADDR"]),
		Count:     1,
	}

	g, ctx := errgroup.WithContext(streamCtx)
	g.Go(func() error {
		return whip.WHIP(ctx)
	})
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
					require.FailNow(t, fmt.Sprintf("[%s] stream playback stalled did=%s, video=%d, audio=%d, elapsed=%s", node.Name, acct.DID, videoStats.Total, audioStats.Total, time.Since(start)))
				}
				prevVideoTotal = videoStats.Total
				prevAudioTotal = audioStats.Total
				log.Log(ctx, fmt.Sprintf("[%s] stream playback", node.Name), "did", acct.DID, "video", videoStats.Total, "audio", audioStats.Total, "elapsed", time.Since(start))
			}
		}
	})
}
