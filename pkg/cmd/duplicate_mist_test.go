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
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	scraper "github.com/starttoaster/prometheus-exporter-scraper"
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

// shadowMetrics is the parent's view of the shadow, scraped over HTTP.
type shadowMetrics struct {
	sessions map[string]int
	verified int
	workers  float64
}

func scrapeShadowMetrics(t *testing.T, scrp *scraper.WebScraper) shadowMetrics {
	t.Helper()
	data, err := scrp.ScrapeWeb()
	require.NoError(t, err)
	m := shadowMetrics{sessions: map[string]int{}}
	for _, c := range data.Counters {
		switch c.Key {
		case "streamplace_duplicate_mist_sessions_total":
			m.sessions[c.Labels["result"]] = c.Value
		case "streamplace_duplicate_mist_verified_segments_total":
			m.verified = c.Value
		}
	}
	for _, g := range data.Gauges {
		if g.Key == "streamplace_duplicate_mist_workers" {
			m.workers = g.Value
		}
	}
	return m
}

func waitForVerifiedSegment(ctx context.Context, t *testing.T, scrp *scraper.WebScraper) shadowMetrics {
	t.Helper()
	for {
		m := scrapeShadowMetrics(t, scrp)
		if m.verified > 0 {
			return m
		}
		select {
		case <-ctx.Done():
			t.Fatalf("shadow never reported a verified segment: %+v", m)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Every outcome series must exist, so a zero is a zero and not a missing one.
func sessionCounts(success, failure int) map[string]int {
	return map[string]int{"success": success, "failure": failure, "canceled": 0, "skipped": 0}
}

// An independently negotiated primary RTMP session receives all media while
// the client-only replay passes the native relay, mp4mux, signer and verifier.
// Run the scenario in its own process so logging, GStreamer state and the
// global metrics remain isolated from the rest of the package suite.
//
// paced streams in real time, burst sends everything at once to exercise the
// downstream drain, non-live uses a publisher path outside the native server's
// /live/ namespace, and corrupt injects invalid RTMP after verified media.
// The primary must receive additional traffic after the shadow has failed.
func TestDuplicateMistEndToEnd(t *testing.T) {
	if os.Getenv("DUPLICATE_MIST_E2E_SCENARIO") != "1" {
		for _, mode := range []string{"paced", "burst", "non-live", "corrupt"} {
			t.Run(mode, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
				defer cancel()
				exe, err := os.Executable()
				require.NoError(t, err)
				cmd := exec.CommandContext(ctx, exe, "-test.run=^TestDuplicateMistEndToEnd$", "-test.v")
				cmd.Env = append(os.Environ(), "DUPLICATE_MIST_E2E_SCENARIO=1", "DUPLICATE_MIST_E2E_MODE="+mode)
				out, err := cmd.CombinedOutput()
				require.NoError(t, err, "%s", out)
				t.Logf("%s", out)
			})
		}
		return
	}
	mode := os.Getenv("DUPLICATE_MIST_E2E_MODE")
	corrupt := mode == "corrupt"
	paced := mode != "burst"
	publisherPath := "/live/shadow-e2e"
	if mode == "non-live" {
		publisherPath = "/app/shadow-e2e"
	}
	_ = flag.Set("v", "3")
	shadowExit := &shadowExitWriter{Writer: os.Stderr, done: make(chan struct{})}
	slog.SetDefault(slog.New(slog.NewTextHandler(shadowExit, nil)))
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()

	// The parent's collectors are in this process's default registry; scrape
	// them the way production is scraped, through promhttp over HTTP.
	metricsSrv := httptest.NewServer(promhttp.Handler())
	defer metricsSrv.Close()
	scrp, err := scraper.NewWebScraper(metricsSrv.URL)
	require.NoError(t, err)
	require.Equal(t, shadowMetrics{sessions: sessionCounts(0, 0)}, scrapeShadowMetrics(t, scrp))

	primary, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer primary.Close()
	primaryResult := make(chan error, 1)
	var primaryBytes atomic.Int64
	go func() {
		conn, err := primary.Accept()
		if err != nil {
			primaryResult <- err
			return
		}
		defer conn.Close()
		deadline, _ := ctx.Deadline()
		_ = conn.SetDeadline(deadline)
		sc := &gortmplib.ServerConn{RW: conn}
		if err := sc.Initialize(); err != nil {
			primaryResult <- err
			return
		}
		if err := sc.Accept(); err != nil {
			primaryResult <- err
			return
		}
		if corrupt {
			// Mist is a byte sink here: it must keep receiving traffic
			// after the shadow parser rejects the injected corruption.
			n, err := io.Copy(io.Discard, conn)
			primaryBytes.Store(n)
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
	publisherEOF := make(chan struct{})
	afterFailure := make(chan int64, 1)
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
		forwarded, err := io.Copy(server, client)
		if err == nil {
			// Keep the actual TLS publisher connected until the parent has
			// observed live verification, even if encoding already reached EOS.
			select {
			case <-publisherEOF:
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
		if corrupt && err == nil {
			// Invalid RTMP after verified media must produce a failure.
			var garbage [64 << 10]byte
			var n int
			n, err = server.Write(garbage[:])
			forwarded += int64(n)
			if err == nil {
				select {
				case <-shadowExit.done:
				case <-ctx.Done():
					err = ctx.Err()
				}
			}
			if err == nil {
				// Prove forwarding still works after the worker is reaped.
				afterFailure <- forwarded
				_, err = server.Write(garbage[:])
			}
		}
		server.Close()
		<-replies
		bridgeDone <- err
	}()

	gstinit.InitGST()
	pipeline, err := gst.NewPipelineFromString(fmt.Sprintf(
		"flvmux name=mux streamable=true ! rtmp2sink sync=%t location=rtmp://%s%s "+
			"videotestsrc num-buffers=300 ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc key-int-max=30 bframes=0 tune=zerolatency ! h264parse ! queue ! mux.video "+
			"audiotestsrc num-buffers=470 samplesperbuffer=1024 ! audio/x-raw,rate=48000 ! audioconvert ! fdkaacenc ! aacparse ! queue ! mux.audio", paced, bridge.Addr(), publisherPath))
	require.NoError(t, err)
	defer pipeline.SetState(gst.StateNull) //nolint:errcheck
	busDone := make(chan error, 1)
	go func() { busDone <- media.HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	if paced {
		// The TLS publisher has not sent EOF; progress cannot be deferred
		// until worker completion. Encoding speed is irrelevant.
		live := waitForVerifiedSegment(ctx, t, scrp)
		require.Equal(t, float64(1), live.workers)
		require.Equal(t, sessionCounts(0, 0), live.sessions)
	}
	close(publisherEOF)
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
	// The session's one terminal outcome is recorded before the exit log.
	final := scrapeShadowMetrics(t, scrp)
	require.Equal(t, float64(0), final.workers)
	if corrupt {
		require.Equal(t, sessionCounts(0, 1), final.sessions)
		require.Positive(t, final.verified)
		// The primary's count excludes handshake/publish bytes. Receiving
		// more than every byte sent before worker exit proves it also
		// received the final write made after the shadow failed.
		require.Greater(t, primaryBytes.Load(), <-afterFailure, "primary stopped receiving once the shadow failed")
	} else {
		require.Equal(t, sessionCounts(1, 0), final.sessions)
		require.Equal(t, 10, final.verified)
	}
	cancel()
	require.NoError(t, <-addonDone)
}
