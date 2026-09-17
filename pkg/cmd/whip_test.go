package cmd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestWHIPRetryCancellation(t *testing.T) {
	for _, retry := range []string{"0", "1h"} {
		t.Run(retry, func(t *testing.T) {
			requests := make(chan struct{}, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- struct{}{}
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- makeWhipCommand(&config.BuildFlags{}).Run(ctx, []string{
					"whip", "--file=unused.mp4", "--endpoint=" + server.URL, "--retry=" + retry,
				})
			}()
			select {
			case <-requests:
			case <-ctx.Done():
				t.Fatal("WHIP never reached endpoint")
			}
			if retry == "0" {
				select {
				case err := <-done:
					require.Error(t, err)
					require.False(t, errors.Is(err, context.DeadlineExceeded))
				case <-ctx.Done():
					t.Fatal("one-shot WHIP did not exit")
				}
				return
			}
			select {
			case err := <-done:
				t.Fatalf("retry exited before cancellation: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation did not interrupt retry delay")
			}
		})
	}
}

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
