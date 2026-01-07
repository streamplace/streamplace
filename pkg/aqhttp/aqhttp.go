package aqhttp

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"
)

var UserAgent string = "streamplace/unknown"

// Client is the default HTTP client with SSRF protection.
// Uses DNS-over-HTTPS to validate destination IPs and blocks private/bogon ranges.
// For trusted infrastructure endpoints, use TrustedClient instead.
var Client http.Client

// TrustedClient is an HTTP client without SSRF protection.
// Use this only for trusted infrastructure endpoints (e.g., livepeer.com)
// where the validation overhead is problematic.
var TrustedClient http.Client

type ClientOptions struct {
	OverrideInTest bool
}

var defaultClientOptions = ClientOptions{
	OverrideInTest: true,
}

func init() {
	// Initialize the trusted client first.
	TrustedClient = http.Client{
		Transport: NewTrustedTransport(),
		Timeout:   30 * time.Second,
	}

	// Initialize the default (untrusted) client which performs SSRF checks.
	Client = http.Client{
		Transport: NewUntrustedTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}

	// When running under `go test` the test binary name typically ends with ".test".
	// In that case, use the trusted client to avoid SSRF blocking for localhost tests.
	if defaultClientOptions.OverrideInTest && len(os.Args) > 0 && strings.HasSuffix(os.Args[0], ".test") {
		Client = TrustedClient
	}
}

// Do executes an HTTP request with SSRF protection (secure by default).
// Most callsites should use this function.
func Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return Client.Do(req.WithContext(ctx))
}

// DoTrusted executes an HTTP request without SSRF protection.
// Use this only for trusted infrastructure endpoints.
func DoTrusted(ctx context.Context, req *http.Request) (*http.Response, error) {
	return TrustedClient.Do(req.WithContext(ctx))
}
