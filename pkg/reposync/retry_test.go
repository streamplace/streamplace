package reposync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

// fastRetry is the policy the network tests use: same shape as production,
// milliseconds instead of seconds.
func fastRetry() RetryPolicy {
	return RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 20 * time.Millisecond}
}

// throttled builds the failure a rate-limiting host sends, optionally with the
// ratelimit-* headers indigo knows how to parse.
func throttled(reset time.Time) failure {
	f := failure{
		status: http.StatusTooManyRequests,
		body:   `{"error":"RateLimitExceeded","message":"Rate Limit Exceeded"}`,
		header: map[string]string{"Content-Type": "application/json"},
	}
	if !reset.IsZero() {
		f.header["ratelimit-limit"] = "3000"
		f.header["ratelimit-remaining"] = "0"
		f.header["ratelimit-policy"] = "3000;w=300"
		f.header["ratelimit-reset"] = strconv.FormatInt(reset.Unix(), 10)
	}
	return f
}

// htmlThrottled is the shape that actually broke a production walk: a 429 whose
// body is an HTML error page, so indigo cannot decode an XRPCError out of it and
// only the status code survives.
var htmlThrottled = failure{
	status: http.StatusTooManyRequests,
	body:   "<html><head><title>429 Too Many Requests</title></head><body>go away</body></html>",
	header: map[string]string{"Content-Type": "text/html"},
}

func xrpcErr(status int, errStr, msg string) error {
	return fmt.Errorf("getBlocks: %w", &xrpc.Error{
		StatusCode: status,
		Wrapped:    &xrpc.XRPCError{ErrStr: errStr, Message: msg},
	})
}

func TestIsRetryable(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"429", xrpcErr(http.StatusTooManyRequests, "RateLimitExceeded", "slow down"), true},
		{"429 with an undecodable body", fmt.Errorf("getBlocks: %w", &xrpc.Error{
			StatusCode: http.StatusTooManyRequests,
			Wrapped:    errors.New("failed to decode xrpc error message: invalid character '<'"),
		}), true},
		{"500", xrpcErr(http.StatusInternalServerError, "InternalServerError", "oops"), true},
		{"502", fmt.Errorf("x: %w", &xrpc.Error{StatusCode: http.StatusBadGateway}), true},
		{"503", fmt.Errorf("x: %w", &xrpc.Error{StatusCode: http.StatusServiceUnavailable}), true},
		{"504", fmt.Errorf("x: %w", &xrpc.Error{StatusCode: http.StatusGatewayTimeout}), true},
		// Numerically 5xx, but a permanent answer: the backfill wants to hear
		// it at once so it can fall back to getRepo.
		{"501", fmt.Errorf("x: %w", &xrpc.Error{StatusCode: http.StatusNotImplemented}), false},
		{"400 could not find cids", xrpcErr(http.StatusBadRequest, "InvalidRequest", "Could not find cids: bafy"), false},
		{"401", fmt.Errorf("x: %w", &xrpc.Error{StatusCode: http.StatusUnauthorized}), false},
		{"404", fmt.Errorf("x: %w", &xrpc.Error{StatusCode: http.StatusNotFound}), false},
		{"connection reset", fmt.Errorf("request failed: %w", syscall.ECONNRESET), true},
		{"connection refused", fmt.Errorf("request failed: %w", syscall.ECONNREFUSED), true},
		{"truncated body", fmt.Errorf("reading response body: %w", io.ErrUnexpectedEOF), true},
		{"timeout", fmt.Errorf("request failed: %w", timeoutError{}), true},
		{"canceled", fmt.Errorf("request failed: %w", context.Canceled), false},
		{"deadline exceeded", fmt.Errorf("request failed: %w", context.DeadlineExceeded), false},
		{"missing block", fmt.Errorf("x: %w", ErrMissingBlock), false},
		{"block mismatch", fmt.Errorf("x: %w", ErrBlockMismatch), false},
		{"plain error", errors.New("nope"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isRetryable(tc.err))
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestRetryDelay(t *testing.T) {
	p := RetryPolicy{BaseDelay: time.Second, MaxDelay: 30 * time.Second}
	plain := errors.New("boom")

	// Exponential, jittered down by at most 25%.
	for _, tc := range []struct{ attempt, wantSec int }{{1, 1}, {2, 2}, {3, 4}, {4, 8}, {5, 16}} {
		full := time.Duration(tc.wantSec) * time.Second
		for i := 0; i < 50; i++ {
			d := p.delay(tc.attempt, plain)
			require.GreaterOrEqual(t, d, time.Duration(float64(full)*0.75), "attempt %d", tc.attempt)
			require.LessOrEqual(t, d, full, "attempt %d", tc.attempt)
		}
	}

	// Capped, and still jittered at the cap so a fleet does not resynchronize.
	var sawJitter bool
	for i := 0; i < 50; i++ {
		d := p.delay(10, plain)
		require.GreaterOrEqual(t, d, 22500*time.Millisecond)
		require.LessOrEqual(t, d, 30*time.Second)
		if d < 29*time.Second {
			sawJitter = true
		}
	}
	require.True(t, sawJitter)

	// The zero policy is the documented defaults.
	require.LessOrEqual(t, RetryPolicy{}.delay(1, plain), DefaultRetryBaseDelay)
	require.GreaterOrEqual(t, RetryPolicy{}.delay(1, plain), DefaultRetryBaseDelay*3/4)

	t.Run("ratelimit reset is honored", func(t *testing.T) {
		// Further out than the backoff for attempt 1 (~1s): wait for the reset.
		err := ratelimited(time.Now().Add(3 * time.Second))
		d := p.delay(1, err)
		require.Greater(t, d, 2500*time.Millisecond)
		require.LessOrEqual(t, d, 3500*time.Millisecond)
	})

	t.Run("ratelimit reset is clamped to MaxDelay", func(t *testing.T) {
		// bsky rate limit windows are minutes long; we would rather make one
		// more doomed attempt than hold a per-PDS lock that long.
		err := ratelimited(time.Now().Add(10 * time.Minute))
		require.Equal(t, 30*time.Second, p.delay(1, err))
	})

	t.Run("a reset in the past does not shorten the backoff", func(t *testing.T) {
		err := ratelimited(time.Now().Add(-time.Minute))
		d := p.delay(3, err)
		require.GreaterOrEqual(t, d, 3*time.Second)
		require.LessOrEqual(t, d, 4*time.Second)
	})
}

func ratelimited(reset time.Time) error {
	return fmt.Errorf("getBlocks: %w", &xrpc.Error{
		StatusCode: http.StatusTooManyRequests,
		Wrapped:    &xrpc.XRPCError{ErrStr: "RateLimitExceeded"},
		Ratelimit:  &xrpc.RatelimitInfo{Limit: 3000, Reset: reset},
	})
}

// TestXRPCBlockFetcherRetries drives the retry loop through the real getBlocks
// path against an HTTP host that fails on a script.
func TestXRPCBlockFetcherRetries(t *testing.T) {
	ctx := context.Background()
	sr := buildSignedRepo(t, testDID, exactnessPaths())

	newFetcher := func(t *testing.T, script ...failure) (*XRPCBlockFetcher, *fakeHost) {
		host := newFakeHost(sr)
		host.blocksFailures = script
		client := host.start(t)
		return &XRPCBlockFetcher{Client: client, DID: testDID, Retry: fastRetry()}, host
	}

	t.Run("429 then success", func(t *testing.T) {
		f, host := newFetcher(t, throttled(time.Time{}), throttled(time.Time{}))
		blocks, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.NoError(t, err)
		require.Contains(t, blocks, sr.root)
		require.Equal(t, 3, host.requests, "two throttles then the real answer")
	})

	t.Run("429 with an HTML body then success", func(t *testing.T) {
		// The production shape: indigo cannot decode the body, so the error is
		// "failed to decode xrpc error message: invalid character '<'" and only
		// the status code is left to classify on.
		f, host := newFetcher(t, htmlThrottled)
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.NoError(t, err)
		require.Equal(t, 2, host.requests)
	})

	t.Run("503 then success", func(t *testing.T) {
		f, host := newFetcher(t, failure{status: http.StatusServiceUnavailable, body: "upstream restarting"})
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.NoError(t, err)
		require.Equal(t, 2, host.requests)
	})

	t.Run("a ratelimit reset in the far future is capped, not slept on", func(t *testing.T) {
		f, host := newFetcher(t, throttled(time.Now().Add(5*time.Minute)))
		start := time.Now()
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.NoError(t, err)
		require.Equal(t, 2, host.requests)
		require.Less(t, time.Since(start), time.Second, "MaxDelay must bound the ratelimit wait")
	})

	t.Run("400 fails fast", func(t *testing.T) {
		// What a TS PDS says when the walk raced a live repo and the blocks of
		// the pinned commit have been garbage collected. Retrying the same CIDs
		// can never help.
		f, host := newFetcher(t, failure{
			status: http.StatusBadRequest,
			body:   `{"error":"InvalidRequest","message":"Could not find cids: bafyreib2"}`,
			header: map[string]string{"Content-Type": "application/json"},
		})
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.Error(t, err)
		require.Equal(t, 1, host.requests, "no retries")
		var xe *xrpc.Error
		require.ErrorAs(t, err, &xe)
		require.Equal(t, http.StatusBadRequest, xe.StatusCode)
		require.NotContains(t, err.Error(), "giving up", "a fail-fast error is passed through unchanged")
	})

	t.Run("retries exhausted", func(t *testing.T) {
		f, host := newFetcher(t, throttled(time.Time{}), throttled(time.Time{}), throttled(time.Time{}),
			throttled(time.Time{}), throttled(time.Time{}), throttled(time.Time{}))
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.Error(t, err)
		require.Equal(t, 5, host.requests, "MaxAttempts is a total, not an extra")
		require.Contains(t, err.Error(), "giving up after 5 attempts")
		var xe *xrpc.Error
		require.ErrorAs(t, err, &xe, "the last error is still inspectable")
		require.Equal(t, http.StatusTooManyRequests, xe.StatusCode)
	})

	t.Run("context cancelled mid backoff", func(t *testing.T) {
		host := newFakeHost(sr)
		host.blocksFailures = []failure{throttled(time.Time{})}
		client := host.start(t)
		// A backoff long enough that returning promptly can only mean the
		// sleep was context aware.
		f := &XRPCBlockFetcher{Client: client, DID: testDID,
			Retry: RetryPolicy{MaxAttempts: 5, BaseDelay: 30 * time.Second, MaxDelay: time.Minute}}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.Error(t, err)
		require.Less(t, time.Since(start), 5*time.Second)
		require.ErrorIs(t, err, context.Canceled)
		var xe *xrpc.Error
		require.ErrorAs(t, err, &xe, "the failure that triggered the backoff is kept too")
		require.Equal(t, http.StatusTooManyRequests, xe.StatusCode)
		require.Equal(t, 1, host.requests)
	})

	t.Run("retrying does not break chunking", func(t *testing.T) {
		// A retry inside one chunk must not disturb the chunk loop: five CIDs
		// at ChunkSize 2 is three chunks, and the failure only costs one extra
		// call.
		host := newFakeHost(sr)
		host.blocksFailures = []failure{throttled(time.Time{})}
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID, ChunkSize: 2, Retry: fastRetry()}
		want := []cid.Cid{sr.root, sr.commitCID}
		for c := range sr.blocks {
			if len(want) >= 5 {
				break
			}
			if c != sr.root && c != sr.commitCID {
				want = append(want, c)
			}
		}
		require.Len(t, want, 5)
		blocks, err := f.GetBlocks(ctx, want)
		require.NoError(t, err)
		require.Len(t, blocks, len(want))
		require.Equal(t, 4, host.requests, "three chunks plus the one retry")
	})
}

// TestFetchVerifiedHeadRetries: the head fetch is one getLatestCommit call, and
// it is the first thing every backfill does, so it gets the same treatment.
func TestFetchVerifiedHeadRetries(t *testing.T) {
	ctx := context.Background()
	sr := buildSignedRepo(t, testDID, exactnessPaths())
	pub, err := sr.priv.PublicKey()
	require.NoError(t, err)

	t.Run("throttled then success", func(t *testing.T) {
		host := newFakeHost(sr)
		host.latestFailures = []failure{throttled(time.Time{}), htmlThrottled,
			{status: http.StatusServiceUnavailable, body: "restarting"}}
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID, Retry: fastRetry()}
		head, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID, fastRetry())
		require.NoError(t, err)
		require.Equal(t, sr.commitCID, head.CID)
		require.Equal(t, 4, host.latestRequests)
	})

	t.Run("RepoNotFound fails fast", func(t *testing.T) {
		host := newFakeHost(sr)
		host.latestFailures = []failure{{
			status: http.StatusBadRequest,
			body:   `{"error":"RepoNotFound","message":"could not find repo"}`,
			header: map[string]string{"Content-Type": "application/json"},
		}}
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID, Retry: fastRetry()}
		_, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID, fastRetry())
		require.Error(t, err)
		require.Equal(t, 1, host.latestRequests)
	})

	t.Run("at most one policy", func(t *testing.T) {
		host := newFakeHost(sr)
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		_, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID, fastRetry(), fastRetry())
		require.Error(t, err)
	})
}
