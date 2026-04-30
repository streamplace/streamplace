package muxl

import (
	"context"
	"crypto"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStress runs many parallel sign calls and reports the latency
// distribution. Skipped by default; opt in with STREAMPLACE_STRESS=1.
//
// Knobs (env):
//   STREAMPLACE_STRESS               required to run (gate)
//   STREAMPLACE_STRESS_MODE          "wasm" | "external" | "both"   default "both"
//   STREAMPLACE_STRESS_CONCURRENCY   parallel goroutines            default runtime.NumCPU
//   STREAMPLACE_STRESS_COUNT         total signs per mode           default 100
//
// Concurrency=N spins up N goroutines that share a queue of `count` jobs
// and pull until empty — closer to production where multiple streamers
// hit the same wasm runtime than per-goroutine count would be.
//
// Each goroutine reuses the same SignerInput shape (segment + cert +
// manifest); only the per-call dispatch differs (KeyPEM vs Sign closure).
//
// Example:
//   STREAMPLACE_STRESS=1 STREAMPLACE_STRESS_MODE=both \
//   STREAMPLACE_STRESS_CONCURRENCY=8 STREAMPLACE_STRESS_COUNT=200 \
//   go test ./pkg/muxl -run TestStress -v -timeout 30m
func TestStress(t *testing.T) {
	if os.Getenv("STREAMPLACE_STRESS") == "" {
		t.Skip("set STREAMPLACE_STRESS=1 to run")
	}

	mode := envDefault("STREAMPLACE_STRESS_MODE", "both")
	concurrency := envInt(t, "STREAMPLACE_STRESS_CONCURRENCY", runtime.NumCPU())
	count := envInt(t, "STREAMPLACE_STRESS_COUNT", 100)

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	muxlRoot := filepath.Join(repoRoot, "..", "muxl")

	keyPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-key.pem"))
	if err != nil {
		t.Skipf("skipping; sibling muxl clone with test keys not available: %v", err)
	}
	certPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-cert.pem"))
	require.NoError(t, err)
	segment, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/sample-segment.mp4"))
	require.NoError(t, err)

	manifest := []byte(`{
		"title": "muxl-sign stress test",
		"assertions": [
			{"label": "c2pa.actions",
			 "data": {"actions": [{"action": "c2pa.created"}]}}
		]
	}`)

	priv := parseES256KPKCS8PEM(t, keyPEM)
	var signer crypto.Signer = priv.ToECDSA()

	wasmInput := func() SignerInput {
		return SignerInput{
			Segment:         segment,
			CertPEM:         certPEM,
			KeyPEM:          keyPEM,
			TrackManifest:   manifest,
			WrapperManifest: manifest,
		}
	}
	externalInput := func() SignerInput {
		return SignerInput{
			Segment:         segment,
			CertPEM:         certPEM,
			Sign:            SignerToCallback(signer, 32),
			TrackManifest:   manifest,
			WrapperManifest: manifest,
		}
	}

	t.Logf("stress: mode=%s concurrency=%d count=%d segment=%d bytes",
		mode, concurrency, count, len(segment))

	// Warm-up: one sign in each mode to ensure the wasm module is compiled
	// before we start timing. CompileModule is cached behind sync.Once so
	// the first call in a fresh process pays for it; we don't want that
	// in the histogram.
	_, err = RunMuxlSigner(context.Background(), wasmInput())
	require.NoError(t, err, "wasm warmup")
	_, err = RunMuxlSigner(context.Background(), externalInput())
	require.NoError(t, err, "external warmup")

	runMode := func(name string, build func() SignerInput) {
		stats := runStress(t, name, concurrency, count, build)
		t.Log(stats.report())
	}

	switch mode {
	case "wasm":
		runMode("wasm", wasmInput)
	case "external":
		runMode("external", externalInput)
	case "both":
		runMode("wasm", wasmInput)
		runMode("external", externalInput)
	default:
		t.Fatalf("unknown STREAMPLACE_STRESS_MODE=%q (want wasm|external|both)", mode)
	}
}

type stressStats struct {
	mode        string
	concurrency int
	count       int
	wallTime    time.Duration
	durations   []time.Duration // sorted ascending
	failures    int
}

func runStress(t *testing.T, name string, concurrency, count int, build func() SignerInput) stressStats {
	t.Helper()
	durations := make([]time.Duration, count)
	var idx atomic.Int64
	idx.Store(-1)
	var failures atomic.Int64

	start := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := idx.Add(1)
				if i >= int64(count) {
					return
				}
				in := build()
				t0 := time.Now()
				_, err := RunMuxlSigner(context.Background(), in)
				dur := time.Since(t0)
				if err != nil {
					failures.Add(1)
					t.Logf("[%s] sign %d failed: %v", name, i, err)
					continue
				}
				durations[i] = dur
			}
		}()
	}
	wg.Wait()

	// Drop any slots that failed (zero duration).
	out := durations[:0]
	for _, d := range durations {
		if d > 0 {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return stressStats{
		mode:        name,
		concurrency: concurrency,
		count:       count,
		wallTime:    time.Since(start),
		durations:   out,
		failures:    int(failures.Load()),
	}
}

func (s stressStats) report() string {
	if len(s.durations) == 0 {
		return fmt.Sprintf("[%s] no successful signs", s.mode)
	}
	throughput := float64(len(s.durations)) / s.wallTime.Seconds()
	mean := time.Duration(0)
	for _, d := range s.durations {
		mean += d
	}
	mean /= time.Duration(len(s.durations))
	return fmt.Sprintf(
		"[%s] concurrency=%d count=%d ok=%d fail=%d wall=%v throughput=%.2f signs/sec\n"+
			"        latency  min=%v  p50=%v  p90=%v  p99=%v  max=%v  mean=%v",
		s.mode, s.concurrency, s.count, len(s.durations), s.failures, s.wallTime.Round(time.Millisecond), throughput,
		s.durations[0],
		percentile(s.durations, 50),
		percentile(s.durations, 90),
		percentile(s.durations, 99),
		s.durations[len(s.durations)-1],
		mean,
	)
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
