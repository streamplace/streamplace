package muxl

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSha256Bench drives muxl-wasm's `bench-sha256` subcommand once for
// each backend and prints the per-iter timings + throughput. This is the
// upper bound on what a host-SHA256 path could save us on the c2pa-rs
// hashing hot loop — the in-wasm baseline is sha2 running through wazero,
// the host path is the streamplace.host_sha256 import (Go's
// crypto/sha256, which uses CPU SHA-NI extensions where available).
//
// Skipped by default; opt in with STREAMPLACE_STRESS=1. Knobs:
//
//	STREAMPLACE_STRESS_SHA256_SIZE        bytes per iteration   default 1 MB
//	STREAMPLACE_STRESS_SHA256_ITERATIONS  iteration count       default 200
func TestSha256Bench(t *testing.T) {
	if os.Getenv("STREAMPLACE_STRESS") == "" {
		t.Skip("set STREAMPLACE_STRESS=1 to run")
	}

	size := envInt(t, "STREAMPLACE_STRESS_SHA256_SIZE", 1024*1024)
	iters := envInt(t, "STREAMPLACE_STRESS_SHA256_ITERATIONS", 200)
	ctx := context.Background()
	mod, err := getModule(ctx)
	require.NoError(t, err)

	runBench := func(mode string) string {
		var stdout bytes.Buffer
		args := []string{
			"muxl-wasm", "bench-sha256",
			"--size", strconv.Itoa(size),
			"--iterations", strconv.Itoa(iters),
			"--mode", mode,
		}
		err := runMuxlWith(ctx, mod, args, nil, true, nil, &stdout, nil, nil, nil, nil)
		require.NoError(t, err)
		return strings.TrimSpace(stdout.String())
	}

	t.Logf("sha256 bench: size=%d iterations=%d", size, iters)
	t.Log(runBench("wasm"))
	t.Log(runBench("host"))
}

func envInt(t *testing.T, key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("invalid %s=%q: %v", key, v, err)
	}
	return n
}
