//go:build !windows

package rtmps

import (
	"context"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/spmetrics"
)

const mistReply = "REPLY-FROM-MIST"

// TestShadowHelperProcess is the shadow worker stand-in; it only does
// anything when re-executed by startShadow with GO_SHADOW_HELPER set.
func TestShadowHelperProcess(t *testing.T) {
	mode := os.Getenv("GO_SHADOW_HELPER")
	if mode == "" {
		return
	}
	out := os.Getenv("GO_SHADOW_OUT")
	switch mode {
	case "stall":
		_ = os.WriteFile(out, []byte(strconv.Itoa(os.Getpid())), 0o644)
		time.Sleep(time.Hour)
	case "crash":
		_, _ = io.CopyN(io.Discard, os.Stdin, 1024)
		os.Stderr.WriteString("boom\n")
		os.Exit(3)
	case "hang-on-eof":
		_, _ = io.Copy(io.Discard, os.Stdin)
		time.Sleep(time.Hour)
	case "env":
		_ = os.WriteFile(out, []byte(strings.Join(os.Environ(), "\n")), 0o644)
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	case "silent":
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	case "report":
		// Stdout is the progress channel: one 0x01 per validated segment, the
		// first while the client is still streaming.
		_, _ = os.Stdout.Write([]byte{1})
		_, _ = io.Copy(io.Discard, os.Stdin)
		_, _ = os.Stdout.Write([]byte{1, 1})
		os.Exit(0)
	case "garbage":
		_, _ = os.Stdout.Write([]byte{1, 1, 'x', 1})
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	os.Exit(2)
}

func helperOpts(t *testing.T, mode string) (shadowOptions, string) {
	out := filepath.Join(t.TempDir(), "out")
	t.Setenv("GO_SHADOW_HELPER", mode)
	t.Setenv("GO_SHADOW_OUT", out)
	return shadowOptions{exe: os.Args[0], args: []string{"-test.run=^TestShadowHelperProcess$"}}, out
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(1)).Read(b)
	return b
}

// The duplicate-mist collectors are process-global and shared with every
// other test, so assertions are deltas against a baseline.
type shadowMetrics struct {
	success, failure, canceled, skipped, verified, workers float64
}

func readShadowMetrics(t *testing.T) shadowMetrics {
	t.Helper()
	val := func(m prometheus.Metric) float64 {
		var d dto.Metric
		require.NoError(t, m.Write(&d))
		return d.GetCounter().GetValue() + d.GetGauge().GetValue()
	}
	sessions := spmetrics.DuplicateMistSessionsTotal
	return shadowMetrics{
		success:  val(sessions.WithLabelValues("success")),
		failure:  val(sessions.WithLabelValues("failure")),
		canceled: val(sessions.WithLabelValues("canceled")),
		skipped:  val(sessions.WithLabelValues("skipped")),
		verified: val(spmetrics.DuplicateMistVerifiedSegmentsTotal),
		workers:  val(spmetrics.DuplicateMistWorkers),
	}
}

func (m shadowMetrics) since(base shadowMetrics) shadowMetrics {
	return shadowMetrics{
		success:  m.success - base.success,
		failure:  m.failure - base.failure,
		canceled: m.canceled - base.canceled,
		skipped:  m.skipped - base.skipped,
		verified: m.verified - base.verified,
		workers:  m.workers - base.workers,
	}
}

func waitDone(t *testing.T, sh *shadow) {
	t.Helper()
	select {
	case <-sh.done:
	case <-time.After(15 * time.Second):
		t.Fatal("shadow worker was not reaped")
	}
}

// runProxy pushes payload through proxyConn in odd-sized writes against a
// fake Mist that greets first, and returns what Mist received and what the
// client received.
func runProxy(t *testing.T, ctx context.Context, sh *shadow, payload []byte) (mistGot, clientGot []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	mistDone := make(chan struct{})
	go func() {
		defer close(mistDone)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = c.Write([]byte(mistReply))
		mistGot, _ = io.ReadAll(c)
	}()

	client, proxySide := net.Pipe()
	proxyDone := make(chan struct{})
	go func() {
		defer close(proxyDone)
		defer proxySide.Close()
		proxyConn(ctx, proxySide, ln.Addr().String(), sh)
	}()

	clientGot = make([]byte, len(mistReply))
	_, err = io.ReadFull(client, clientGot)
	require.NoError(t, err)
	rng := rand.New(rand.NewSource(2))
	for rest := payload; len(rest) > 0; {
		n := min(len(rest), 1+rng.Intn(70000))
		_, err := client.Write(rest[:n])
		require.NoError(t, err)
		rest = rest[n:]
	}
	client.Close()
	for _, ch := range []chan struct{}{proxyDone, mistDone} {
		select {
		case <-ch:
		case <-time.After(15 * time.Second):
			t.Fatal("primary connection did not finish")
		}
	}
	return mistGot, clientGot
}

func TestShadowAdmissionLimitLeavesMistRunningAndReusesSlot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := readShadowMetrics(t)
	opts, _ := helperOpts(t, "stall")
	opts.slots = make(chan struct{}, 1)
	opts.ringSize = 1024
	first := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, first)
	rejected := startShadow(ctx, opts, 2, "127.0.0.1:2")
	require.Nil(t, rejected, "a saturated addon must not spawn another worker")
	require.Equal(t, shadowMetrics{skipped: 1, workers: 1}, readShadowMetrics(t).since(base))
	payload := randomBytes(64 << 10)
	mistGot, clientGot := runProxy(t, ctx, rejected, payload)
	require.Equal(t, payload, mistGot)
	require.Equal(t, mistReply, string(clientGot))

	cancel()
	waitDone(t, first)
	nextCtx, nextCancel := context.WithCancel(context.Background())
	defer nextCancel()
	next := startShadow(nextCtx, opts, 3, "127.0.0.1:3")
	require.NotNil(t, next, "a reaped worker must release admission")
	nextCancel()
	waitDone(t, next)
	require.Equal(t, shadowMetrics{canceled: 2, skipped: 1}, readShadowMetrics(t).since(base))
}

func TestStalledShadowDoesNotAffectPrimary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, out := helperOpts(t, "stall")
	opts.ringSize = 256 << 10
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)

	require.Eventually(t, func() bool { _, err := os.Stat(out); return err == nil }, 10*time.Second, 10*time.Millisecond)
	payload := randomBytes(4 << 20)
	mistGot, clientGot := runProxy(t, ctx, sh, payload)
	waitDone(t, sh)

	require.Equal(t, payload, mistGot)
	require.Equal(t, mistReply, string(clientGot))
	require.Equal(t, shadowMetrics{failure: 1}, readShadowMetrics(t).since(base))
	pid := readPid(t, out)
	require.ErrorIs(t, syscall.Kill(pid, 0), syscall.ESRCH, "stalled worker must be killed and reaped")
}

func TestCrashedShadowDoesNotAffectPrimary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, _ := helperOpts(t, "crash")
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)

	payload := randomBytes(4 << 20)
	mistGot, clientGot := runProxy(t, ctx, sh, payload)
	waitDone(t, sh)

	require.Equal(t, payload, mistGot)
	require.Equal(t, mistReply, string(clientGot))
	require.Equal(t, shadowMetrics{failure: 1}, readShadowMetrics(t).since(base), "crash, broken stdin pipe and bad exit are one failure")
}

func TestShadowHungAtStreamEndIsKilled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, _ := helperOpts(t, "hang-on-eof")
	opts.exitGrace = 300 * time.Millisecond
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)

	payload := randomBytes(100 << 10)
	mistGot, _ := runProxy(t, ctx, sh, payload)
	require.Equal(t, payload, mistGot)
	waitDone(t, sh)
	require.Equal(t, shadowMetrics{failure: 1}, readShadowMetrics(t).since(base))
}

func TestServerCancelReapsShadow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, out := helperOpts(t, "stall")
	var released atomic.Int32
	opts.release = func() { released.Add(1) }
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)
	sh.tee(ctx, []byte("some client bytes"))
	require.Eventually(t, func() bool { _, err := os.Stat(out); return err == nil }, 10*time.Second, 10*time.Millisecond)

	cancel()
	waitDone(t, sh)
	require.Eventually(t, func() bool { return released.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, shadowMetrics{canceled: 1}, readShadowMetrics(t).since(base), "server shutdown is neither a success nor a failure")
	require.ErrorIs(t, syscall.Kill(readPid(t, out), 0), syscall.ESRCH)
}

func TestShadowWorkerEnvStripsSPVars(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, out := helperOpts(t, "env")
	t.Setenv("SP_SECRET_KEY", "hunter2")
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)
	sh.finish(ctx)
	waitDone(t, sh)

	env, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Contains(t, string(env), "GO_SHADOW_HELPER=env")
	require.NotContains(t, string(env), "SP_SECRET_KEY")
	require.NotContains(t, string(env), "hunter2")
}

func TestStartFailureLeavesPrimaryAlone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := readShadowMetrics(t)
	var released atomic.Int32
	sh := startShadow(ctx, shadowOptions{exe: "/nonexistent/streamplace", release: func() { released.Add(1) }}, 1, "127.0.0.1:1")
	require.Nil(t, sh)
	require.Equal(t, int32(1), released.Load())
	require.Equal(t, shadowMetrics{skipped: 1}, readShadowMetrics(t).since(base))

	payload := randomBytes(64 << 10)
	mistGot, clientGot := runProxy(t, ctx, sh, payload)
	require.Equal(t, payload, mistGot)
	require.Equal(t, mistReply, string(clientGot))
}

// The worker's progress stream is what makes a session a success: the live
// reports must reach the parent while the client is still connected, and the
// session is counted exactly once, after the publisher's EOF.
func TestShadowCountsProgressLiveAndSuccessAtEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, _ := helperOpts(t, "report")
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)

	require.Eventually(t, func() bool {
		return readShadowMetrics(t).since(base) == shadowMetrics{verified: 1, workers: 1}
	}, 10*time.Second, 10*time.Millisecond, "a report must be counted before the publisher ends")

	sh.tee(ctx, randomBytes(1024))
	sh.finish(ctx)
	waitDone(t, sh)
	require.Equal(t, shadowMetrics{success: 1, verified: 3}, readShadowMetrics(t).since(base))
}

func TestMistDisconnectAfterShadowProgressIsFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	opts, _ := helperOpts(t, "report")
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)
	require.Eventually(t, func() bool {
		return readShadowMetrics(t).since(base).verified == 1
	}, 10*time.Second, 10*time.Millisecond)

	primary, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer primary.Close()
	payload := []byte("publisher has not sent EOF")
	primaryDone := make(chan error, 1)
	go func() {
		conn, err := primary.Accept()
		if err != nil {
			primaryDone <- err
			return
		}
		defer conn.Close()
		_, err = io.ReadFull(conn, make([]byte, len(payload)))
		primaryDone <- err
		// Mist disconnects while the publisher is still connected.
	}()
	client, proxySide := net.Pipe()
	defer client.Close()
	defer proxySide.Close()
	proxyDone := make(chan struct{})
	go func() {
		defer close(proxyDone)
		proxyConn(ctx, proxySide, primary.Addr().String(), sh)
	}()
	_, err = client.Write(payload)
	require.NoError(t, err)
	_, err = io.ReadAll(client)
	require.NoError(t, err)
	require.NoError(t, <-primaryDone)
	<-proxyDone
	waitDone(t, sh)
	delta := readShadowMetrics(t).since(base)
	require.Positive(t, delta.verified)
	require.Equal(t, shadowMetrics{failure: 1, verified: delta.verified}, delta,
		"verified progress plus Mist disconnect is not publisher EOF")
}

// A clean exit that never reported a validated segment proved nothing.
func TestShadowCleanExitWithoutProgressIsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, _ := helperOpts(t, "silent")
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)
	sh.finish(ctx)
	waitDone(t, sh)
	require.Equal(t, shadowMetrics{failure: 1}, readShadowMetrics(t).since(base))
}

// Anything but progress bytes on stdout makes the channel untrustworthy; only
// the valid prefix counts and the session can never be a success.
func TestShadowInvalidProgressOutputIsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, _ := helperOpts(t, "garbage")
	base := readShadowMetrics(t)
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)
	sh.finish(ctx)
	waitDone(t, sh)
	require.Equal(t, shadowMetrics{failure: 1, verified: 2}, readShadowMetrics(t).since(base))
}

func TestRingWrapsAndRejectsOverflow(t *testing.T) {
	r := newRing(1000)
	rng := rand.New(rand.NewSource(3))
	var want, got []byte
	seq := byte(0)
	for i := 0; i < 5000; i++ {
		p := make([]byte, 1+rng.Intn(300))
		for j := range p {
			p[j] = seq
			seq++
		}
		if r.write(p) {
			want = append(want, p...)
		} else {
			require.Greater(t, len(p), len(r.buf)-r.n, "only a full ring may reject a write")
		}
		for rng.Intn(2) == 0 {
			b, _ := r.peek(1 + rng.Intn(200))
			if len(b) == 0 {
				break
			}
			got = append(got, b...)
			r.advance(len(b))
		}
	}
	for {
		b, _ := r.peek(1 << 20)
		if len(b) == 0 {
			break
		}
		got = append(got, b...)
		r.advance(len(b))
	}
	require.Equal(t, want, got)
}

func readPid(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	pid, err := strconv.Atoi(string(b))
	require.NoError(t, err)
	return pid
}
