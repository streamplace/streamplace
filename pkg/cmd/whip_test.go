package cmd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// whipTestServer counts the WHIP requests it receives and fails every one, so a
// test can observe how many sessions the client actually started.
func whipTestServer() (*httptest.Server, *atomic.Int64) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	return server, &requests
}

// TestWHIPRetryStartsAnotherSession covers the point of the flag: when a session
// ends, another one starts. Without this the retry loop could be a no-op and the
// delay test below would still pass.
func TestWHIPRetryStartsAnotherSession(t *testing.T) {
	server, requests := whipTestServer()
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- makeWhipCommand(&config.BuildFlags{}).Run(ctx, []string{
			"whip", "--file=unused.mp4", "--endpoint=" + server.URL, "--retry=10ms",
		})
	}()
	require.Eventually(t, func() bool { return requests.Load() >= 2 }, 20*time.Second, 10*time.Millisecond,
		"retry should have started a second WHIP session")
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("client did not exit after cancellation")
	}
}

// TestWHIPWithoutRetryStopsAfterOneSession pins the default: no retry means one
// attempt, so existing callers and --whip-test keep the old behavior.
func TestWHIPWithoutRetryStopsAfterOneSession(t *testing.T) {
	server, requests := whipTestServer()
	defer server.Close()
	err := makeWhipCommand(&config.BuildFlags{}).Run(t.Context(), []string{
		"whip", "--file=unused.mp4", "--endpoint=" + server.URL,
	})
	require.Error(t, err)
	require.False(t, errors.Is(err, context.DeadlineExceeded), "should fail on its own, not on a test deadline")
	require.EqualValues(t, 1, requests.Load())
}

// TestWHIPRetryDelayIsInterruptible: cancelling during the retry delay stops the
// client promptly instead of leaving it sleeping with the terminal closed.
func TestWHIPRetryDelayIsInterruptible(t *testing.T) {
	server, requests := whipTestServer()
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- makeWhipCommand(&config.BuildFlags{}).Run(ctx, []string{
			"whip", "--file=unused.mp4", "--endpoint=" + server.URL, "--retry=1h",
		})
	}()
	require.Eventually(t, func() bool { return requests.Load() >= 1 }, 20*time.Second, 10*time.Millisecond,
		"client never reached the endpoint")
	select {
	case err := <-done:
		t.Fatalf("retry returned instead of waiting: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("cancellation did not interrupt the retry delay")
	}
}

// TestWHIPConnectionCancellation: a WHIP request that never gets a response must
// abort when the context is cancelled rather than wedging the client until the
// node answers.
func TestWHIPConnectionCancellation(t *testing.T) {
	requests := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		<-release
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		client := &WHIPClient{Endpoint: server.URL}
		_, err := client.StartWHIPConnection(ctx, "test", "")
		done <- err
	}()
	select {
	case <-requests:
	case <-ctx.Done():
		t.Fatal("WHIP never reached endpoint")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not interrupt stalled WHIP request")
	}
}
