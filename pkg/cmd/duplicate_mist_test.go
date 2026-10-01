package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/rtmps"
)

// Re-exec the real command in tests, just as the addon re-execs the node.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "duplicate-mist-worker" {
		if err := makeDuplicateMistWorkerCommand().Run(context.Background(), os.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type shadowExitWriter struct {
	io.Writer
	done chan struct{}
	once sync.Once
}

func (w *shadowExitWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("duplicate-mist-test: shadow worker exited")) {
		w.once.Do(func() { close(w.done) })
	}
	return w.Writer.Write(p)
}

// An independently negotiated primary RTMP session receives all media while
// the client-only replay passes the native relay, mp4mux, signer and verifier.
// Run the scenario in its own process so logging and GStreamer state remain
// isolated from the rest of the package suite.
func TestDuplicateMistEndToEnd(t *testing.T) {
	if os.Getenv("DUPLICATE_MIST_E2E_SCENARIO") != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		exe, err := os.Executable()
		require.NoError(t, err)
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestDuplicateMistEndToEnd$", "-test.v")
		cmd.Env = append(os.Environ(), "DUPLICATE_MIST_E2E_SCENARIO=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		// Multiple verified GoPs, not merely successful parser initialization.
		match := regexp.MustCompile(`duplicate-mist-test shadow finished.*segments_verified=(\d+)`).FindSubmatch(out)
		require.Len(t, match, 2, "%s", out)
		verified, err := strconv.Atoi(string(match[1]))
		require.NoError(t, err)
		require.GreaterOrEqual(t, verified, 5, "%s", out)
		require.NotContains(t, string(out), "duplicate-mist-test failed", "%s", out)
		t.Logf("%s", out)
		return
	}
	_ = flag.Set("v", "3")
	shadowExit := &shadowExitWriter{Writer: os.Stderr, done: make(chan struct{})}
	slog.SetDefault(slog.New(slog.NewTextHandler(shadowExit, nil)))
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()

	primary, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer primary.Close()
	primaryResult := make(chan error, 1)
	go func() {
		conn, err := primary.Accept()
		if err != nil {
			primaryResult <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
		sc := &gortmplib.ServerConn{RW: conn}
		if err := sc.Initialize(); err != nil {
			primaryResult <- err
			return
		}
		if err := sc.Accept(); err != nil {
			primaryResult <- err
			return
		}
		r := &gortmplib.Reader{Conn: sc}
		if err := r.Initialize(); err != nil {
			primaryResult <- err
			return
		}
		var videoEnd, audioEnd time.Duration
		for _, track := range r.Tracks() {
			switch track := track.(type) {
			case *format.H264:
				r.OnDataH264(track, func(pts, dts time.Duration, au [][]byte) { videoEnd = dts })
			case *format.MPEG4Audio:
				r.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) { audioEnd = pts })
			}
		}
		for {
			if err := r.Read(); err != nil {
				if videoEnd < 8*time.Second || audioEnd < 8*time.Second {
					primaryResult <- fmt.Errorf("primary media ended early: video=%s audio=%s: %w", videoEnd, audioEnd, err)
				} else {
					primaryResult <- nil
				}
				return
			}
		}
	}()

	// The repository's local TLS fixture is trusted only by this test client.
	cert, err := tls.LoadX509KeyPair("../../localhost.pem", "../../localhost-key.pem")
	require.NoError(t, err)
	port, err := freePort()
	require.NoError(t, err)
	addonAddr := fmt.Sprintf("127.0.0.1:%d", port)
	cli := &config.CLI{RTMPServerAddon: primary.Addr().String(), RTMPSAddonAddr: addonAddr, DuplicateMistTest: true}
	addonDone := make(chan error, 1)
	go func() {
		addonDone <- rtmps.ServeRTMPSAddon(ctx, cli, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	}()

	// rtmp2sink is plaintext-only; a private one-session TLS bridge supplies
	// the encoder's encrypted connection, without introducing a second parser.
	bridge, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer bridge.Close()
	bridgeDone := make(chan error, 1)
	go func() {
		client, err := bridge.Accept()
		if err != nil {
			bridgeDone <- err
			return
		}
		defer client.Close()
		var server *tls.Conn
		for {
			// Self-signed fixture; scoped to this loopback test connection.
			server, err = tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", addonAddr, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			if err == nil || ctx.Err() != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err != nil {
			bridgeDone <- err
			return
		}
		defer server.Close()
		stop := context.AfterFunc(ctx, func() { client.Close(); server.Close() })
		defer stop()
		replies := make(chan struct{})
		go func() { _, _ = io.Copy(client, server); client.Close(); close(replies) }()
		_, err = io.Copy(server, client)
		server.Close()
		<-replies
		bridgeDone <- err
	}()

	gstinit.InitGST()
	pipeline, err := gst.NewPipelineFromString(fmt.Sprintf(
		"flvmux name=mux streamable=true ! rtmp2sink location=rtmp://%s/live/shadow-e2e "+
			"videotestsrc num-buffers=300 ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc key-int-max=30 bframes=0 tune=zerolatency ! h264parse ! queue ! mux.video "+
			"audiotestsrc num-buffers=470 samplesperbuffer=1024 ! audio/x-raw,rate=48000 ! audioconvert ! fdkaacenc ! aacparse ! queue ! mux.audio", bridge.Addr()))
	require.NoError(t, err)
	defer pipeline.SetState(gst.StateNull) //nolint:errcheck
	busDone := make(chan error, 1)
	go func() { busDone <- media.HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	select {
	case err := <-busDone:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("publisher timed out")
	}
	require.NoError(t, pipeline.SetState(gst.StateNull))
	select {
	case err := <-primaryResult:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("primary failed to finish")
	}
	select {
	case err := <-bridgeDone:
		require.True(t, err == nil || errors.Is(err, net.ErrClosed), "bridge: %v", err)
	case <-ctx.Done():
		t.Fatal("TLS bridge failed to finish")
	}
	// Wait for the final GoP and natural worker exit, not a timing assumption
	// that could let node shutdown kill an otherwise healthy slow worker.
	select {
	case <-shadowExit.done:
	case <-ctx.Done():
		t.Fatal("shadow failed to exit after publisher EOF")
	}
	cancel()
	require.NoError(t, <-addonDone)
}
