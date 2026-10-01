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

	"github.com/stretchr/testify/require"
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

func TestStalledShadowDoesNotAffectPrimary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, out := helperOpts(t, "stall")
	opts.ringSize = 256 << 10
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)

	require.Eventually(t, func() bool { _, err := os.Stat(out); return err == nil }, 10*time.Second, 10*time.Millisecond)
	payload := randomBytes(4 << 20)
	mistGot, clientGot := runProxy(t, ctx, sh, payload)
	waitDone(t, sh)

	require.Equal(t, payload, mistGot)
	require.Equal(t, mistReply, string(clientGot))
	require.True(t, sh.aborted.Load(), "overflow must abort the shadow")
	pid := readPid(t, out)
	require.ErrorIs(t, syscall.Kill(pid, 0), syscall.ESRCH, "stalled worker must be killed and reaped")
}

func TestCrashedShadowDoesNotAffectPrimary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, _ := helperOpts(t, "crash")
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)

	payload := randomBytes(4 << 20)
	mistGot, clientGot := runProxy(t, ctx, sh, payload)
	waitDone(t, sh)

	require.Equal(t, payload, mistGot)
	require.Equal(t, mistReply, string(clientGot))
	require.True(t, sh.aborted.Load())
}

func TestShadowHungAtStreamEndIsKilled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, _ := helperOpts(t, "hang-on-eof")
	opts.exitGrace = 300 * time.Millisecond
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)

	payload := randomBytes(100 << 10)
	mistGot, _ := runProxy(t, ctx, sh, payload)
	require.Equal(t, payload, mistGot)
	waitDone(t, sh)
	require.True(t, sh.aborted.Load(), "a worker that outlives the grace period must be killed")
}

func TestServerCancelReapsShadow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts, out := helperOpts(t, "stall")
	var released atomic.Int32
	opts.release = func() { released.Add(1) }
	sh := startShadow(ctx, opts, 1, "127.0.0.1:1")
	require.NotNil(t, sh)
	sh.tee(ctx, []byte("some client bytes"))
	require.Eventually(t, func() bool { _, err := os.Stat(out); return err == nil }, 10*time.Second, 10*time.Millisecond)

	cancel()
	waitDone(t, sh)
	require.Eventually(t, func() bool { return released.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
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
	var released atomic.Int32
	sh := startShadow(ctx, shadowOptions{exe: "/nonexistent/streamplace", release: func() { released.Add(1) }}, 1, "127.0.0.1:1")
	require.Nil(t, sh)
	require.Equal(t, int32(1), released.Load())

	payload := randomBytes(64 << 10)
	mistGot, clientGot := runProxy(t, ctx, sh, payload)
	require.Equal(t, payload, mistGot)
	require.Equal(t, mistReply, string(clientGot))
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
