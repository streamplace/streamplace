package aqhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientRedirects(t *testing.T) {
	// Temporarily disable the test override to test actual Client behavior
	originalOverride := defaultClientOptions.OverrideInTest
	defaultClientOptions.OverrideInTest = false

	// Reinitialize the Client with SSRF protection
	Client = http.Client{
		Transport: NewUntrustedTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}

	defer func() {
		defaultClientOptions.OverrideInTest = originalOverride
		// Restore to TrustedClient for other tests
		Client = TrustedClient
	}()

	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			redirectCount++
			http.Redirect(w, r, "/end", http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/start", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The Client should block localhost/127.0.0.1 due to SSRF protection
	_, err = Do(ctx, req)
	require.Error(t, err, "Client should block requests to localhost")
	require.Contains(t, err.Error(), "private/invalid", "Error should mention private/invalid IPs")
}

func TestClientCanAccessExternal(t *testing.T) {
	// Temporarily disable the test override to test actual Client behavior
	originalOverride := defaultClientOptions.OverrideInTest
	defaultClientOptions.OverrideInTest = false

	// Reinitialize the Client with SSRF protection
	Client = http.Client{
		Transport: NewUntrustedTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}

	defer func() {
		defaultClientOptions.OverrideInTest = originalOverride
		// Restore to TrustedClient for other tests
		Client = TrustedClient
	}()

	req, err := http.NewRequest("GET", "https://plc.directory", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = Do(ctx, req)
	require.NoError(t, err, "Client shouldn't block requests to plc.directory")
	//require.Contains(t, err.Error(), "private/invalid", "Error should mention private/invalid IPs")
}

func TestTrustedClientFollowsRedirects(t *testing.T) {
	redirectCount := 0
	finalCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			redirectCount++
			http.Redirect(w, r, "/end", http.StatusTemporaryRedirect)
			return
		}
		finalCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/start", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := DoTrusted(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "TrustedClient should follow redirects")
	require.Equal(t, 1, redirectCount, "Redirect handler should have been called once")
	require.Equal(t, 1, finalCount, "Final handler should have been called once")
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = Do(ctx, req)
	require.Error(t, err, "Request should timeout")
}

func TestTrustedClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = DoTrusted(ctx, req)
	require.Error(t, err, "Request should timeout")
}

func TestSuccessfulRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := Do(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}
